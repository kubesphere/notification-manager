package config

import (
	"context"
	"fmt"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/pkg/apis/v2beta1"
	"k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	kcache "k8s.io/client-go/tools/cache"
	"os"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	kconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
	"strings"
	"time"
)

const (
	user                = "User"
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
	tenantKeyNamespace  = "namespace"
	nsEnvironment       = "NAMESPACE"
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
	// Label selector to filter valid global Receiver CR
	globalReceiverSelector *metav1.LabelSelector
	// Label selector to filter valid tenant Receiver CR
	tenantReceiverSelector *metav1.LabelSelector
	// Receiver config for each tenant user, in form of map[tenantID]map[type/name]Receiver
	receivers    map[string]map[string]Receiver
	ReceiverOpts *v2beta1.Options
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
	ReceiverOpts           *v2beta1.Options
	done                   chan interface{}
}

func New(ctx context.Context, logger log.Logger) (*Config, error) {
	scheme := runtime.NewScheme()
	_ = v2beta1.AddToScheme(scheme)
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
	nmInf, err := c.cache.GetInformer(&v2beta1.NotificationManager{})
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

	receiverInformer, err := c.cache.GetInformer(&v2beta1.Receiver{})
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

	configInformer, err := c.cache.GetInformer(&v2beta1.Config{})
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
		config := obj.(*v2beta1.Config)
		name = config.Name
		lbs = config.Labels
		spec = config.Spec
	} else {
		receiver := obj.(*v2beta1.Receiver)
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

func (c *Config) tenantIDFromNs(namespace *string) ([]string, error) {
	tenantIDs := make([]string, 0)
	// Use namespace as TenantID directly if tenantKey is "namespace"
	if c.tenantKey == tenantKeyNamespace {
		tenantIDs = append(tenantIDs, *namespace)
		return tenantIDs, nil
	}

	// Find User in rolebinding for KubeSphere
	rbList := rbacv1.RoleBindingList{}
	if err := c.cache.List(c.ctx, &rbList, client.InNamespace(*namespace)); err != nil {
		_ = level.Error(c.logger).Log("msg", "Failed to list rolebinding", "err", err)
		return []string{}, err
	}
	for _, rb := range rbList.Items {
		if rb.Subjects != nil {
			for _, v := range rb.Subjects {
				if v.Kind == user {
					tenantIDs = append(tenantIDs, v.Name)
				}
			}
		}
	}
	return tenantIDs, nil
}

func (c *Config) RcvsFromNs(namespace *string) []Receiver {
	rcvs := make([]Receiver, 0)
	// Return global receiver if namespace is nil, global receiver should receive all notifications
	if namespace == nil {
		p := param{}
		p.op = opGet
		p.tenantID = globalTenantID
		p.done = make(chan interface{}, 1)
		c.ch <- &p
		o := <-p.done
		if r, ok := o.(map[string]Receiver); ok {
			for _, v := range r {
				if v.Enabled() {
					rcvs = append(rcvs, v)
				}
			}
		}
	} else {
		// Get all global receiver first, global receiver should receive all notifications
		p := param{}
		p.op = opGet
		p.tenantID = globalTenantID
		p.done = make(chan interface{}, 1)
		c.ch <- &p
		o := <-p.done
		if r, ok := o.(map[string]Receiver); ok {
			for _, v := range r {
				if v.Enabled() {
					rcvs = append(rcvs, v)
				}
			}
		}

		// Get receivers for each tenant if namespace is not nil
		if tenantIDs, err := c.tenantIDFromNs(namespace); err != nil {
			_ = level.Error(c.logger).Log("msg", "Unable to find tenantID", "err", err)
		} else {
			for _, t := range tenantIDs {
				p := param{}
				p.op = opGet
				p.tenantID = t
				p.done = make(chan interface{}, 1)
				c.ch <- &p
				o := <-p.done
				if r, ok := o.(map[string]Receiver); ok {
					for _, v := range r {
						if v.Enabled() {
							rcvs = append(rcvs, v)
						}
					}
				}
			}
		}
	}

	return rcvs
}

func (c *Config) onNmAdd(obj interface{}) {
	if nm, ok := obj.(*v2beta1.NotificationManager); ok {
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
		p.done = make(chan interface{}, 1)
		c.ch <- p
		<-p.done

		c.updateReloadTimestamp()
	}
}

func (c *Config) updateReloadTimestamp() {

	receiverList := v2beta1.ReceiverList{}
	if err := c.client.List(c.ctx, &receiverList, client.InNamespace("")); err != nil {
		_ = level.Error(c.logger).Log("msg", "Failed to list receiver", "err", err)
		return
	}

	configList := v2beta1.ConfigList{}
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
	if _, ok := obj.(*v2beta1.NotificationManager); ok {
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

	// If crd's label matches globalSelector such as "type = global",
	// then this is a global receiver or config, and tenantID should be set to an unique tenantID
	if c.globalReceiverSelector != nil {
		for k, expected := range c.globalReceiverSelector.MatchLabels {
			if v, exists := p.labels[k]; exists && v == expected {
				p.tenantID = globalTenantID
				break
			}
		}
	}

	// If crd's label matches tenantReceiverSelector such as "type = tenant",
	// then crd's tenantKey's value should be used as tenantID,
	// For example, if tenantKey is "user" and label "user=admin" exists,
	// then "admin" should be used as tenantID
	if c.tenantReceiverSelector != nil {
		for k, expected := range c.tenantReceiverSelector.MatchLabels {
			if v, exists := p.labels[k]; exists && v == expected {
				if v, exists := p.labels[c.tenantKey]; exists {
					p.tenantID = v
				}
				break
			}
		}
	}

	// If it is a config crd, update global default configs if crd's label match defaultConfigSelector
	if c.defaultConfigSelector != nil && p.isConfig {
		sel, _ := metav1.LabelSelectorAsSelector(c.defaultConfigSelector)
		if sel.Matches(labels.Set(p.labels)) {
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

func (c *Config) GetSecretData(selector *v2beta1.SecretKeySelector) (string, error) {

	if selector == nil {
		return "", fmt.Errorf("SecretKeySelector is nil")
	}

	ns := selector.Namespace
	if len(ns) == 0 {
		ns = c.namespace
	}

	secret := v1.Secret{}
	if err := c.cache.Get(c.ctx, types.NamespacedName{Namespace: ns, Name: selector.Name}, &secret); err != nil {
		return "", err
	}

	return string(secret.Data[selector.Key]), nil
}
