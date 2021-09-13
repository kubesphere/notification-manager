package config

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/pkg/apis/v2beta2"
	"github.com/kubesphere/notification-manager/pkg/internal"
	"github.com/kubesphere/notification-manager/pkg/utils"
	"github.com/modern-go/reflect2"
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
	resourceConfig      = "config"
	resourceReceiver    = "receiver"
	opAdd               = "add"
	opUpdate            = "update"
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
	// Label key used to distinguish different user
	tenantKey string
	// Whether to use sidecar to get tenant list.
	tenantSidecar bool
	// Label selector to filter valid global Receiver CR
	globalReceiverSelector *metav1.LabelSelector
	// Label selector to filter valid tenant Receiver CR
	tenantReceiverSelector *metav1.LabelSelector
	// Receiver for each tenant user, in form of map[tenantID]map[type/name]Receiver
	receivers map[string]map[string]internal.Receiver
	// Config for each tenant user, in form of map[tenantID]map[type/name]Receiver
	configs      map[string]map[string]internal.Config
	ReceiverOpts *v2beta2.Options
	// Channel to receive receiver create/update/delete operations and then update receivers
	ch chan *task
	// The pod's namespace
	namespace string
	history   *v2beta2.HistoryReceiver
	// Dose the notification manager crd add.
	nmAdd bool
}

type task struct {
	op       string
	opType   string
	obj      interface{}
	tenantID []string
	done     chan interface{}
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
		tenantKey:              defaultTenantKey,
		defaultConfigSelector:  nil,
		tenantReceiverSelector: nil,
		globalReceiverSelector: nil,
		receivers:              make(map[string]map[string]internal.Receiver),
		configs:                make(map[string]map[string]internal.Config),
		ReceiverOpts:           nil,
		ch:                     make(chan *task, ChannelCapacity),
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
			c.onChange(obj, opAdd, resourceReceiver)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.onChange(newObj, opUpdate, resourceReceiver)
		},
		DeleteFunc: func(obj interface{}) {
			c.onChange(obj, opDel, resourceReceiver)
		},
	})

	configInformer, err := c.cache.GetInformer(&v2beta2.Config{})
	if err != nil {
		_ = level.Error(c.logger).Log("msg", "Failed to get config informer", "err", err)
		return err
	}
	configInformer.AddEventHandler(kcache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.onChange(obj, opAdd, resourceConfig)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.onChange(newObj, opUpdate, resourceConfig)
		},
		DeleteFunc: func(obj interface{}) {
			c.onChange(obj, opDel, resourceConfig)
		},
	})

	if ok := c.cache.WaitForCacheSync(c.ctx.Done()); !ok {
		return utils.Error("NotificationManager cache failed")
	}

	_ = level.Info(c.logger).Log("msg", "Setting up informers successfully")
	return c.ctx.Err()
}

func (c *Config) onNmAdd(obj interface{}) {

	c.onChange(obj, opAdd, notificationManager)
	c.updateReloadTimestamp()
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
	c.onChange(obj, opDel, notificationManager)
}

func (c *Config) onChange(obj interface{}, op, opType string) {

	t := &task{
		op:     op,
		obj:    obj,
		opType: opType,
		done:   make(chan interface{}, 1),
	}

	c.ch <- t
	<-t.done
}

func (c *Config) sync(t *task) {

	if t.op == opGet {
		// Return all receivers of the specified tenant (map[opType/name]*Receiver)
		// via the done channel if exists
		for _, id := range t.tenantID {
			t.done <- c.receivers[id]
		}
	} else if t.opType == notificationManager {
		c.nmChange(t)

		_ = level.Info(c.logger).Log("msg", "NotificationManager changed", "op", t.op)
	} else {
		c.resourceChanged(t)
	}

	close(t.done)
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

func (c *Config) RcvsFromTenantID(ids []string) []internal.Receiver {

	t := &task{
		op:       opGet,
		tenantID: ids,
		done:     make(chan interface{}, 1),
	}

	c.ch <- t
	rcvs := make([]internal.Receiver, 0)
	for {
		o, more := <-t.done
		if !more {
			break
		}

		if r, ok := o.(map[string]internal.Receiver); ok {
			for _, v := range r {
				if v.Enabled() {
					rcvs = append(rcvs, v.Clone())
				}
			}
		}
	}

	return rcvs
}

// `matchingConfig` used to get a matched config for a receiver.
// It will return true when config is found.
func getMatchedConfig(r internal.Receiver, configs map[string]map[string]internal.Config) bool {

	match := func(configs map[string]internal.Config, selector *metav1.LabelSelector) bool {
		p := math.MaxInt32
		for k, v := range configs {
			if strings.HasPrefix(k, r.GetType()) {
				if utils.LabelMatchSelector(v.GetLabels(), selector) {
					if v.Validate() == nil {
						if v.GetPriority() < p {
							r.SetConfig(v.Clone())
							p = v.GetPriority()
						}
					}
				}
			}
		}

		if p < math.MaxInt32 {
			return true
		}

		return false
	}

	configSelector := r.GetConfigSelector()
	// If config selector is nil, use default config
	if configSelector == nil {
		return match(configs[defaultConfig], nil)
	}

	tenantID := r.GetTenantID()
	// The global receiver use the default config.
	if tenantID == globalTenantID {
		tenantID = defaultConfig
	}

	// If no config matched the config selector, use default config.
	if found := match(configs[tenantID], configSelector); !found {
		return match(configs[defaultConfig], nil)
	} else {
		return true
	}
}

func (c *Config) RcvsFromNs(namespace *string) []internal.Receiver {

	// Get all global receiver first, global receiver should receive all notifications.
	rcvs := c.RcvsFromTenantID([]string{globalTenantID})

	// Get tenant receivers.
	if namespace != nil && len(*namespace) > 0 {
		// Get all tenants which need to receive the notifications in this namespace.
		tenantIDs, err := c.tenantIDFromNs(*namespace)
		if err != nil {
			_ = level.Error(c.logger).Log("msg", "get tenantID error", "err", err, "namespace", *namespace)
		} else {
			// Get receivers for each tenant.
			rcvs = append(rcvs, c.RcvsFromTenantID(tenantIDs)...)
		}
	}

	for _, rcv := range rcvs {
		getMatchedConfig(rcv, c.configs)
	}

	return rcvs
}

func (c *Config) nmChange(t *task) {
	if t.op == opAdd {
		spec := t.obj.(*v2beta2.NotificationManager).Spec
		c.tenantKey = spec.Receivers.TenantKey
		c.defaultConfigSelector = spec.DefaultConfigSelector
		c.tenantReceiverSelector = spec.Receivers.TenantReceiverSelector
		c.globalReceiverSelector = spec.Receivers.GlobalReceiverSelector
		c.ReceiverOpts = spec.Receivers.Options
		c.tenantSidecar = false
		if spec.Sidecars != nil {
			if sidecar, ok := spec.Sidecars[v2beta2.Tenant]; ok && sidecar != nil {
				c.tenantSidecar = true
			}
		}
		c.history = spec.History
		c.nmAdd = true
	} else {
		c.tenantKey = defaultTenantKey
		c.globalReceiverSelector = nil
		c.tenantReceiverSelector = nil
		c.defaultConfigSelector = nil
		c.ReceiverOpts = nil
	}
}

func (c *Config) getTenantID(label map[string]string) string {
	// If crd is a global receiver, tenantID should be set to a unique tenantID.
	if c.isGlobal(label) {
		return globalTenantID
	}

	// If crd is a tenant receiver or config,
	// then crd's tenantKey's value should be used as tenantID,
	// For example, if tenantKey is "user" and label "user=admin" exists,
	// then "admin" should be used as tenantID
	if b, v := c.isTenant(label); b {
		return v
	}

	// If crd is a defalut config, tenantID should be set to a unique tenantID.
	if c.isDefaultConfig(label) {
		return defaultConfig
	}

	return ""
}

// If the label matches globalSelector such as "type = global",
// then the crd with this label is a global receiver.
func (c *Config) isGlobal(label map[string]string) bool {

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

func (c *Config) resourceChanged(t *task) {
	if !c.nmAdd {
		return
	}

	tenantID := c.getTenantID(utils.GetObjectLabels(t.obj))
	if len(tenantID) == 0 {
		_ = level.Warn(c.logger).Log("msg", "Ignore because of empty tenantID", "name", utils.GetObjectName(t.obj), "tenantKey", c.tenantKey)
		return
	}

	if t.opType == resourceConfig {
		obj, ok := t.obj.(*v2beta2.Config)
		if !ok {
			return
		}

		// If operation is `update` or `delete`, it needs to delete the old config.
		if t.op == opUpdate || t.op == opDel {
			suffix := fmt.Sprintf("/%s", obj.Name)
			for id := range c.configs {
				found := false
				for k := range c.configs[id] {
					if strings.HasSuffix(k, suffix) {
						delete(c.configs[id], k)
						found = true
					}
				}

				if found {
					break
				}
			}
		}

		if t.op == opAdd || t.op == opUpdate {
			configs := NewConfigs(obj)
			if _, ok := c.configs[tenantID]; !ok {
				c.configs[tenantID] = make(map[string]internal.Config)
			}
			for k, v := range configs {
				if !reflect2.IsNil(v) {
					c.configs[tenantID][k] = v
				}
			}
		}

		_ = level.Info(c.logger).Log("msg", "Config changed", "op", t.op, "name", obj.Name)
	} else if t.opType == resourceReceiver {
		obj, ok := t.obj.(*v2beta2.Receiver)
		if !ok {
			return
		}

		// If operation is `update` or `delete`, it needs to delete the old receivers.
		if t.op == opUpdate || t.op == opDel {
			suffix := fmt.Sprintf("/%s", obj.Name)
			for id := range c.receivers {
				found := false
				for k := range c.receivers[id] {
					if strings.HasSuffix(k, suffix) {
						delete(c.receivers[id], k)
						found = true
					}
				}

				if found {
					break
				}
			}
		}

		if t.op == opAdd || t.op == opUpdate {
			receivers := NewReceivers(tenantID, obj)
			if _, ok := c.receivers[tenantID]; !ok {
				c.receivers[tenantID] = make(map[string]internal.Receiver)
			}
			for k, v := range receivers {
				if !reflect2.IsNil(v) {
					c.receivers[tenantID][k] = v
				}
			}
		}

		_ = level.Info(c.logger).Log("msg", "Receiver changed", "op", t.op, "name", obj.Name)
	}
}

func (c *Config) ListReceiver(tenant, opType string) interface{} {

	m := make(map[string]interface{})
	for k, v := range c.receivers {
		if len(tenant) > 0 {
			if k != tenant {
				continue
			}
		}

		for key, value := range v {
			if len(opType) > 0 {
				if !strings.HasPrefix(key, opType) {
					continue
				}
			}

			m[key] = value
		}
	}

	return m
}

func (c *Config) ListConfig(tenant, opType string) interface{} {

	m := make(map[string]interface{})
	for k, v := range c.configs {
		if len(tenant) > 0 {
			if k != tenant {
				continue
			}
		}

		for key, value := range v {
			if len(opType) > 0 {
				if !strings.HasPrefix(key, opType) {
					continue
				}
			}

			m[key] = value
		}
	}

	return m
}

func (c *Config) ListReceiverWithConfig(tenantID, name, opType string) interface{} {

	var rcvs []internal.Receiver
	for k := range c.receivers {
		for _, v := range c.receivers[k] {
			if ((tenantID != "" && v.GetTenantID() == tenantID) || tenantID == "") &&
				((name != "" && v.GetName() == name) || name == "") &&
				((opType != "" && v.GetType() == opType) || opType == "") {
				r := v.Clone()
				getMatchedConfig(r, c.configs)
				rcvs = append(rcvs, r)
			}
		}
	}

	return rcvs
}

func (c *Config) GetCredential(credential *v2beta2.Credential) (string, error) {

	if credential == nil {
		return "", utils.Error("credential is nil")
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

	return "", utils.Error("no value or valueFrom set")
}

// GenerateReceivers generate receivers from the given notification config and notification receiver.
// If the notification config is nil, use the existed config.
// If the notification config is not nil, the receiver will use the given config,
// the notification config type must match the notification receiver type.
func (c *Config) GenerateReceivers(nr *v2beta2.Receiver, nc *v2beta2.Config) ([]internal.Receiver, error) {

	tenantID := c.getTenantID(nr.Labels)
	if len(tenantID) == 0 {
		return nil, utils.Error("unknown tenant")
	}

	receivers := NewReceivers(tenantID, nr)
	configs := NewConfigs(nc)
	var rcvs []internal.Receiver
	for _, r := range receivers {

		// If the config is not nil, try to use the provided config.
		if configs != nil {
			getMatchedConfig(r, map[string]map[string]internal.Config{
				c.getTenantID(nc.Labels): configs,
			})
		} else {
			// Else, use the config in cluster.
			getMatchedConfig(r, c.configs)
		}

		if err := r.Validate(); err != nil {
			return nil, err
		}

		rcvs = append(rcvs, r)
	}

	if rcvs == nil || len(rcvs) == 0 {
		return nil, utils.Error("no receivers provided")
	}

	return rcvs, nil
}

func (c *Config) GetHistoryReceivers() []internal.Receiver {

	var rcvs []internal.Receiver

	if c.history != nil {
		historyReceivers := NewReceivers("", &v2beta2.Receiver{
			Spec: v2beta2.ReceiverSpec{
				Webhook: c.history.Webhook,
			},
		})
		if historyReceivers != nil {
			for _, v := range historyReceivers {
				rcvs = append(rcvs, v)
			}
		}
	}

	return rcvs
}
