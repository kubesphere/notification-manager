package config

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/pkg/apis/v2beta2"
	"github.com/kubesphere/notification-manager/pkg/utils"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	kcache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	kconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	globalTenantID      = "notification-manager/type/global"
	defaultConfig       = "notification-manager/type/default"
	defaultTenantKey    = "user"
	notificationManager = "notification-manager"
	email               = "email"
	wechat              = "wechat"
	slack               = "slack"
	webhook             = "webhook"
	dingtalk            = "dingtalk"
	opAdd               = "add"
	opDel               = "delete"
	opGet               = "get"
	nsEnvironment       = "NAMESPACE"

	tenantSidecarURL = "http://localhost:19094/api/v2/tenant"
)

var (
	ChannelCapacity = 1000
)

type Config struct {
	logger log.Logger
	ctx    context.Context
	cache  cache.Cache
	client client.Client
	// Default config selector
	defaultConfigSelector *metav1.LabelSelector
	// Default config for email, wechat, slack etc.
	defaultConfig map[string]interface{}
	// Label key used to distinguish different user
	tenantKey string
	// Whether to use sidecar to get tenant list.
	tenantSidecar bool
	// Label selector to filter valid global Receiver CR
	globalReceiverSelector *metav1.LabelSelector
	// Label selector to filter valid tenant Receiver CR
	tenantReceiverSelector *metav1.LabelSelector
	// Receiver config for each tenant user, in form of map[tenantID]map[type/name]Receiver
	receivers    map[string]map[string]Receiver
	ReceiverOpts *v2beta2.Options
	// Channel to receive receiver create/update/delete operations and then update receivers
	ch chan *param
	// The pod's namespace
	namespace string
	// Dose the notification manager crd add.
	nmAdd bool
}

type param struct {
	op                     string
	tenantID               string
	opType                 string
	name                   string
	labels                 map[string]string
	tenantKey              string
	defaultConfigSelector  *metav1.LabelSelector
	tenantReceiverSelector *metav1.LabelSelector
	globalReceiverSelector *metav1.LabelSelector
	obj                    interface{}
	receiver               Receiver
	isConfig               bool
	ReceiverOpts           *v2beta2.Options
	tenantSidecar          bool
	done                   chan interface{}
}

func New(ctx context.Context, logger log.Logger) (*Config, error) {
	scheme := runtime.NewScheme()
	_ = v2beta2.AddToScheme(scheme)
	_ = v1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)

	cfg, err := kconfig.GetConfig()
	if err != nil {
		_ = level.Error(logger).Log("msg", "Failed to get kubeconfig ", "err", err)
		return nil, err
	}

	informerCache, err := cache.New(cfg, cache.Options{
		Scheme: scheme,
	})

	if err != nil {
		_ = level.Error(logger).Log("msg", "Failed to create cache", "err", err)
		return nil, err
	}

	c, err := newClient(cfg, informerCache, scheme)
	if err != nil {
		_ = level.Error(logger).Log("msg", "Failed to create client", "err", err)
		return nil, err
	}

	ns := os.Getenv(nsEnvironment)
	if len(ns) == 0 {
		return nil, level.Error(logger).Log("msg", "namespace is empty")
	}

	return &Config{
		ctx:                    ctx,
		logger:                 logger,
		cache:                  informerCache,
		client:                 c,
		defaultConfig:          make(map[string]interface{}),
		tenantKey:              defaultTenantKey,
		defaultConfigSelector:  nil,
		tenantReceiverSelector: nil,
		globalReceiverSelector: nil,
		receivers:              make(map[string]map[string]Receiver),
		ReceiverOpts:           nil,
		ch:                     make(chan *param, ChannelCapacity),
		namespace:              ns,
	}, nil
}

// Setting up client
func newClient(cfg *rest.Config, cache cache.Cache, scheme *runtime.Scheme) (client.Client, error) {
	mapper, err := func(c *rest.Config) (meta.RESTMapper, error) {
		return apiutil.NewDynamicRESTMapper(c)
	}(cfg)
	if err != nil {
		return nil, err
	}

	c, err := client.New(cfg, client.Options{Scheme: scheme, Mapper: mapper})
	if err != nil {
		return nil, err
	}

	return &client.DelegatingClient{
		Reader: &client.DelegatingReader{
			CacheReader:  cache,
			ClientReader: c,
		},
		Writer:       c,
		StatusClient: c,
	}, nil
}

func (c *Config) Run() error {
	go func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			case p, more := <-c.ch:
				if !more {
					return
				}
				c.sync(p)
			}
		}
	}(c.ctx)
	go func() {
		_ = c.cache.Start(c.ctx.Done())
	}()

	// Setup informer for NotificationManager
	nmInf, err := c.cache.GetInformer(&v2beta2.NotificationManager{})
	if err != nil {
		_ = level.Error(c.logger).Log("msg", "Failed to get informer for NotificationManager", "err", err)
		return err
	}
	nmInf.AddEventHandler(kcache.ResourceEventHandlerFuncs{
		AddFunc: c.onNmAdd,
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.onNmAdd(newObj)
		},
		DeleteFunc: c.onNmDel,
	})

	receiverInformer, err := c.cache.GetInformer(&v2beta2.Receiver{})
	if err != nil {
		_ = level.Error(c.logger).Log("msg", "Failed to get receiver informer", "err", err)
		return err
	}
	receiverInformer.AddEventHandler(kcache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.onChange(obj, opAdd, false)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.onChange(newObj, opAdd, false)
		},
		DeleteFunc: func(obj interface{}) {
			c.onChange(obj, opDel, false)
		},
	})

	configInformer, err := c.cache.GetInformer(&v2beta2.Config{})
	if err != nil {
		_ = level.Error(c.logger).Log("msg", "Failed to get config informer", "err", err)
		return err
	}
	configInformer.AddEventHandler(kcache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.onChange(obj, opAdd, true)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.onChange(newObj, opAdd, true)
		},
		DeleteFunc: func(obj interface{}) {
			c.onChange(obj, opDel, true)
		},
	})

	if ok := c.cache.WaitForCacheSync(c.ctx.Done()); !ok {
		return fmt.Errorf("NotificationManager cache failed")
	}

	_ = level.Info(c.logger).Log("msg", "Setting up informers successfully")
	return c.ctx.Err()
}

func (c *Config) onChange(obj interface{}, op string, isConfig bool) {

	name := ""
	var lbs map[string]string
	var spec interface{}
	if isConfig {
		config := obj.(*v2beta2.Config)
		name = config.Name
		lbs = config.Labels
		spec = config.Spec
	} else {
		receiver := obj.(*v2beta2.Receiver)
		name = receiver.Name
		lbs = receiver.Labels
		spec = receiver.Spec
	}

	v := reflect.ValueOf(spec)
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		if !f.IsZero() {
			p := &param{
				name:     name,
				labels:   lbs,
				op:       op,
				isConfig: isConfig,
				obj:      f.Interface(),
			}
			p.done = make(chan interface{}, 1)
			c.ch <- p
			<-p.done
		}
	}
}

func (c *Config) sync(p *param) {

	if p.op == opGet {
		// Return all receivers of the specified tenant (map[opType/name]*Receiver)
		// via the done channel if exists
		if v, exist := c.receivers[p.tenantID]; exist {
			p.done <- v
			// Return empty struct if receivers of the specified tenant cannot be found
		} else {
			p.done <- struct{}{}
		}
		return
	}

	if p.opType == notificationManager {
		c.nmChange(p)
		return
	}

	c.getReceiver(p)
	if len(p.tenantID) == 0 || len(p.opType) == 0 {
		p.done <- struct{}{}
		return
	}

	if p.op == opAdd {

		if reflect.ValueOf(p.receiver).IsNil() {
			p.done <- struct{}{}
			return
		}

		config := p.receiver.GetConfig()

		if p.isConfig {

			if reflect.ValueOf(config).IsNil() {
				p.done <- struct{}{}
				return
			}

			if p.tenantID == defaultConfig {
				// Setup default config
				c.defaultConfig[p.opType] = config

				// Update config of the receivers who use default config
				for tenantID := range c.receivers {
					for k := range c.receivers[tenantID] {
						if strings.HasPrefix(k, p.opType) && c.receivers[tenantID][k].UseDefault() {
							_ = c.receivers[tenantID][k].SetConfig(config)
						}
					}
				}
			} else {
				// Update Config of the receiver with the same tenantID
				if _, exist := c.receivers[p.tenantID]; exist {
					for k := range c.receivers[p.tenantID] {
						if strings.HasPrefix(k, p.opType) {
							_ = c.receivers[p.tenantID][k].SetConfig(config)
							c.receivers[p.tenantID][k].SetUseDefault(false)
						}
					}
				}
			}
		} else {
			// Setup Receiver with default config if config is nil
			if reflect.ValueOf(config).IsNil() {
				p.receiver.SetUseDefault(true)
				if dc, ok := c.defaultConfig[p.opType]; ok {
					_ = p.receiver.SetConfig(dc)
				}
			}

			rcvKey := fmt.Sprintf("%s/%s", p.opType, p.name)
			if _, exist := c.receivers[p.tenantID]; !exist {
				c.receivers[p.tenantID] = make(map[string]Receiver)
			}
			c.receivers[p.tenantID][rcvKey] = p.receiver
		}
	} else if p.op == opDel {
		if p.isConfig {
			if p.tenantID == defaultConfig {
				// Reset default config
				delete(c.defaultConfig, p.opType)
				// Delete config of receivers who use default config.
				for tenantID := range c.receivers {
					for k := range c.receivers[tenantID] {
						if strings.HasPrefix(k, p.opType) && c.receivers[tenantID][k].UseDefault() {
							_ = c.receivers[tenantID][k].SetConfig(nil)
						}
					}
				}
			} else {
				// Delete config of the receiver with the same tenantID, and use default config
				if _, exist := c.receivers[p.tenantID]; exist {
					for k := range c.receivers[p.tenantID] {
						if strings.HasPrefix(k, p.opType) {
							_ = c.receivers[p.tenantID][k].SetConfig(nil)
							c.receivers[p.tenantID][k].SetUseDefault(true)
							if dc, ok := c.defaultConfig[p.opType]; ok {
								_ = c.receivers[p.tenantID][k].SetConfig(dc)
							}
						}
					}
				}
			}
		} else {
			// Delete the receiver with the same tenantID
			rcvKey := fmt.Sprintf("%s/%s", p.opType, p.name)
			if _, exist := c.receivers[p.tenantID]; exist {
				delete(c.receivers[p.tenantID], rcvKey)
				// If the tenant has no receiver, delete it
				if len(c.receivers[p.tenantID]) <= 0 {
					delete(c.receivers, p.tenantID)
				}
			}
		}
	}

	p.done <- struct{}{}
}

func (c *Config) nmChange(p *param) {
	if p.op == opAdd {
		c.tenantKey = p.tenantKey
		c.defaultConfigSelector = p.defaultConfigSelector
		c.tenantReceiverSelector = p.tenantReceiverSelector
		c.globalReceiverSelector = p.globalReceiverSelector
		c.ReceiverOpts = p.ReceiverOpts
		c.tenantSidecar = p.tenantSidecar
		c.nmAdd = true
	} else if p.op == opDel {
		c.tenantKey = defaultTenantKey
		c.globalReceiverSelector = nil
		c.tenantReceiverSelector = nil
		c.defaultConfigSelector = nil
		c.ReceiverOpts = nil
	}

	p.done <- struct{}{}
}

func (c *Config) tenantIDFromNs(namespace string) ([]string, error) {
	tenantIDs := make([]string, 0)
	// Use namespace as TenantID directly if tenantSidecar not provided.
	if !c.tenantSidecar {
		tenantIDs = append(tenantIDs, namespace)
		return tenantIDs, nil
	}

	p := make(map[string]string)
	p["namespace"] = namespace
	u, err := utils.UrlWithParameters(tenantSidecarURL, p)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")

	body, err := utils.DoHttpRequest(context.Background(), nil, request)
	if err != nil {
		return nil, err
	}

	res := make([]string, 0)
	if err := utils.JsonUnmarshal(body, &res); err != nil {
		return nil, err
	}

	_ = level.Debug(c.logger).Log("msg", "get tenants from namespace", "namespace", namespace, "tenant", utils.ArrayToString(res, ","))

	return res, nil
}

func (c *Config) RcvsFromTenantID(tenantID string) []Receiver {
	p := param{}
	p.op = opGet
	p.tenantID = tenantID
	p.done = make(chan interface{}, 1)
	c.ch <- &p
	o := <-p.done
	rcvs := make([]Receiver, 0)
	if r, ok := o.(map[string]Receiver); ok {
		for _, v := range r {
			if v.Enabled() {
				rcvs = append(rcvs, v)
			}
		}
	}

	return rcvs
}

func (c *Config) RcvsFromNs(namespace *string) []Receiver {

	// Get all global receiver first, global receiver should receive all notifications.
	rcvs := c.RcvsFromTenantID(globalTenantID)

	// Return global receiver if namespace is nil.
	if namespace == nil || len(*namespace) == 0 {
		return rcvs
	}

	// Get all tenants which need to receive the notifications in this namespace.
	tenantIDs, err := c.tenantIDFromNs(*namespace)
	if err != nil {
		_ = level.Error(c.logger).Log("msg", "get tenantID error", "err", err, "namespace", *namespace)
		return rcvs
	}

	// Get receivers for each tenant.
	for _, t := range tenantIDs {
		rcvs = append(rcvs, c.RcvsFromTenantID(t)...)
	}

	return rcvs
}

func (c *Config) onNmAdd(obj interface{}) {
	if nm, ok := obj.(*v2beta2.NotificationManager); ok {
		p := &param{}
		p.op = opAdd
		p.name = nm.Name
		p.opType = notificationManager
		p.tenantKey = nm.Spec.Receivers.TenantKey
		p.defaultConfigSelector = nm.Spec.DefaultConfigSelector
		p.globalReceiverSelector = nm.Spec.Receivers.GlobalReceiverSelector
		p.tenantReceiverSelector = nm.Spec.Receivers.TenantReceiverSelector
		if nm.Spec.Receivers.Options != nil {
			p.ReceiverOpts = nm.Spec.Receivers.Options
		}
		if nm.Spec.TenantSidecar != nil {
			p.tenantSidecar = true
		}
		p.done = make(chan interface{}, 1)
		c.ch <- p
		<-p.done

		c.updateReloadTimestamp()
	}
}

func (c *Config) updateReloadTimestamp() {

	receiverList := v2beta2.ReceiverList{}
	if err := c.client.List(c.ctx, &receiverList, client.InNamespace("")); err != nil {
		_ = level.Error(c.logger).Log("msg", "Failed to list receiver", "err", err)
		return
	}

	configList := v2beta2.ConfigList{}
	if err := c.client.List(c.ctx, &configList, client.InNamespace("")); err != nil {
		_ = level.Error(c.logger).Log("msg", "Failed to list config", "err", err)
		return
	}

	for _, obj := range receiverList.Items {

		annotations := obj.Annotations
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations["reloadtimestamp"] = time.Now().String()
		obj.SetAnnotations(annotations)
		err := c.client.Update(c.ctx, &obj)
		if err != nil {
			_ = level.Error(c.logger).Log("msg", "update receiver error", "name", obj.GetName(), "err", err)
			continue
		}
	}

	for _, obj := range configList.Items {

		annotations := obj.Annotations
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations["reloadtimestamp"] = time.Now().String()
		obj.SetAnnotations(annotations)
		err := c.client.Update(c.ctx, &obj)
		if err != nil {
			_ = level.Error(c.logger).Log("msg", "update config error", "name", obj.GetName(), "err", err)
			continue
		}
	}
}

func (c *Config) onNmDel(obj interface{}) {
	if _, ok := obj.(*v2beta2.NotificationManager); ok {
		p := &param{}
		p.op = opDel
		p.opType = notificationManager
		p.done = make(chan interface{}, 1)
		c.ch <- p
		<-p.done
	}
}

func (c *Config) getReceiver(p *param) {

	if !c.nmAdd {
		return
	}

	_ = level.Info(c.logger).Log("msg", "resource change", "op", p.op, "name", p.name)

	// If crd is a global receiver, tenantID should be set to an unique tenantID.
	if c.isGlobalReceiver(p.labels) {
		p.tenantID = globalTenantID
	}

	// If crd is a tenant receiver or config,
	// then crd's tenantKey's value should be used as tenantID,
	// For example, if tenantKey is "user" and label "user=admin" exists,
	// then "admin" should be used as tenantID
	if b, v := c.isTenant(p.labels); b {
		p.tenantID = v
	}

	// If it is a config crd, update global default configs if crd is a default config
	if p.isConfig {
		if c.isDefaultConfig(p.labels) {
			p.tenantID = defaultConfig
		}
	}

	if len(p.tenantID) == 0 {
		_ = level.Warn(c.logger).Log("msg", "Ignore empty tenantID", "op", p.op, "tenantKey", c.tenantKey, "name", p.name)
		return
	}

	p.opType = getOpType(p.obj)
	if p.op == opAdd {
		p.receiver = NewReceiver(c, p.obj)
	}
}

// If the label matches globalSelector such as "type = global",
// then the crd with this label is a global receiver.
func (c *Config) isGlobalReceiver(label map[string]string) bool {

	if c.globalReceiverSelector != nil {
		for k, expected := range c.globalReceiverSelector.MatchLabels {
			if v, exists := label[k]; exists && v == expected {
				return true
			}
		}
	}

	return false
}

// If the label matches defaultConfigSelector such as "type = default",
// then the crd with this label is a default config.
func (c *Config) isDefaultConfig(label map[string]string) bool {

	if c.defaultConfigSelector != nil {
		sel, _ := metav1.LabelSelectorAsSelector(c.defaultConfigSelector)
		if sel.Matches(labels.Set(label)) {
			return true
		}
	}

	return false
}

// If the label matches tenantReceiverSelector such as "type = tenant",
// then the crd with this label is a tenant receiver or config,
func (c *Config) isTenant(label map[string]string) (bool, string) {

	if c.tenantReceiverSelector != nil {
		for k, expected := range c.tenantReceiverSelector.MatchLabels {
			if v, exists := label[k]; exists && v == expected {
				if v, exists := label[c.tenantKey]; exists {
					return true, v
				}
				break
			}
		}
	}

	return false, ""
}

func (c *Config) OutputReceiver(tenant, receiver string) interface{} {

	m := make(map[string]interface{})
	for k, v := range c.receivers {
		if len(tenant) > 0 {
			if k != tenant {
				continue
			}
		}

		for key, value := range v {
			if len(receiver) > 0 {
				if !strings.HasPrefix(key, receiver) {
					continue
				}
			}

			m[key] = value
		}
	}

	return m
}

func (c *Config) GetCredential(credential *v2beta2.Credential) (string, error) {

	if credential == nil {
		return "", fmt.Errorf("credential is nil")
	}

	if len(credential.Value) > 0 {
		return credential.Value, nil
	}

	if credential.ValueFrom != nil {
		if credential.ValueFrom.SecretKeyRef != nil {
			ns := credential.ValueFrom.SecretKeyRef.Namespace
			if len(ns) == 0 {
				ns = c.namespace
			}

			secret := v1.Secret{}
			if err := c.cache.Get(c.ctx, types.NamespacedName{Namespace: ns, Name: credential.ValueFrom.SecretKeyRef.Name}, &secret); err != nil {
				return "", err
			}

			return string(secret.Data[credential.ValueFrom.SecretKeyRef.Key]), nil
		}
	}

	return "", fmt.Errorf("no value or valueFrom set")
}

// GenerateReceivers generate receivers from the given notification config and notification receiver.
// If the notification config is nil, use the exist config.
// If the notification config is not nil, the receiver will use the given config,
// the notification config type must matched the notification receiver type.
func (c *Config) GenerateReceivers(nr *v2beta2.Receiver, nc *v2beta2.Config) ([]Receiver, error) {

	var receivers []Receiver
	v := reflect.ValueOf(nr.Spec)
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		if f.IsZero() {
			continue
		}

		// create receiver with exist config.
		receiver := NewReceiver(c, f.Interface())
		if receiver == nil {
			continue
		}

		configSelector := receiver.GetConfigSelector()
		// If the config is not nil, try to use this config.
		if nc != nil {

			// Check whether config matchs receiver or not.
			// If the notification receiver is a global receiver, it needs a default config
			if c.isGlobalReceiver(nr.Labels) {
				if !c.isDefaultConfig(nc.Labels) {
					return nil, fmt.Errorf("receiver %s, need default config", receiver.GetType())
				}
			} else if b, _ := c.isTenant(nr.Labels); b {
				// If the notification receiver is a tenant receiver, it will select config with config selector.
				// If the config selector of receiver is nil, it needs a default config.
				if configSelector == nil {
					if !c.isDefaultConfig(nc.Labels) {
						return nil, fmt.Errorf("receiver %s needs a default config", receiver.GetType())
					}
				} else {
					// If the config selector of receiver is not nil, it needs a tenant config.
					labelSelector, _ := metav1.LabelSelectorAsSelector(receiver.GetConfigSelector())
					if labelSelector.Empty() {
						return nil, fmt.Errorf("invalid config selector for receiver %s", receiver.GetType())
					}

					if !labelSelector.Matches(labels.Set(nc.Labels)) {
						return nil, fmt.Errorf("config not matched receiver %s", receiver.GetType())
					}
				}
			} else {
				return nil, fmt.Errorf("receiver is neither global nor tenant")
			}

			_ = receiver.SetConfig(c.getConfigFromCRD(nc, receiver.GetType()))
		} else {
			// If the config is nil, it will use the exist config.

			// If the config selector is nil, it will use the exist default config.
			if configSelector == nil {
				_ = receiver.SetConfig(c.defaultConfig[receiver.GetType()])
			}
		}

		if err := receiver.Validate(); err != nil {
			return nil, fmt.Errorf("validate receiver %s error, %s", receiver.GetType(), err.Error())
		}

		receivers = append(receivers, receiver)
	}

	if receivers == nil || len(receivers) == 0 {
		return nil, fmt.Errorf("no receivers provided")
	}

	return receivers, nil
}

func (c *Config) getConfigFromCRD(config *v2beta2.Config, receiverType string) interface{} {

	v := reflect.ValueOf(config.Spec)
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		if !f.IsZero() {
			r := NewReceiver(c, f.Interface())
			if !reflect.ValueOf(r).IsNil() && r.GetType() == receiverType {
				return r.GetConfig()
			}
		}
	}

	return nil
}
