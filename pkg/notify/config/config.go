package config

import (
	"context"
	"fmt"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/pkg/apis/v1alpha1"
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
	resourceFactory        map[string]factory
	// Receiver config for each tenant user, in form of map[tenantID]map[type/namespace/name]Receiver
	receivers    map[string]map[string]Receiver
	ReceiverOpts *v1alpha1.Options
	// Channel to receive receiver create/update/delete operations and then update receivers
	ch           chan *param
	nmNamespaces []string
	// Dose the notification manager crd add.
	nmAdd bool
}

type param struct {
	op                     string
	tenantID               string
	opType                 string
	namespace              string
	name                   string
	tenantKey              string
	defaultConfigSelector  *metav1.LabelSelector
	tenantReceiverSelector *metav1.LabelSelector
	globalReceiverSelector *metav1.LabelSelector
	obj                    interface{}
	receiver               Receiver
	isConfig               bool
	ReceiverOpts           *v1alpha1.Options
	nmNamespaces           []string
	done                   chan interface{}
}

func New(ctx context.Context, logger log.Logger, namespaces string) (*Config, error) {
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)
	_ = v1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)

	cfg, err := kconfig.GetConfig()
	if err != nil {
		_ = level.Error(logger).Log("msg", "Failed to get kubeconfig ", "err", err)
		return nil, err
	}

	var informerCache cache.Cache
	var nmNamespaces []string
	if len(namespaces) > 0 {
		ns := os.Getenv("NAMESPACE")
		if len(ns) == 0 {
			return nil, fmt.Errorf("namespace unknown")
		}

		// Notification manager namespaces must include the namespace notification manager in.
		nmNamespaces = strings.Split(namespaces, ":")
		if !sliceIn(nmNamespaces, ns) {
			nmNamespaces = append(nmNamespaces, ns)
		}
		ncf := cache.MultiNamespacedCacheBuilder(nmNamespaces)
		informerCache, err = ncf(cfg, cache.Options{
			Scheme: scheme,
		})
	} else {
		informerCache, err = cache.New(cfg, cache.Options{
			Scheme: scheme,
		})
	}
	if err != nil {
		_ = level.Error(logger).Log("msg", "Failed to create cache", "err", err)
		return nil, err
	}

	c, err := newClient(cfg, informerCache, scheme)
	if err != nil {
		_ = level.Error(logger).Log("msg", "Failed to create client", "err", err)
		return nil, err
	}

	f := make(map[string]factory)
	register := func(key string, newReceiverFunc func() Receiver,
		newReceiverObjectFunc func() runtime.Object, newReceiverObjectListFunc func() runtime.Object,
		newConfigObjectFunc func() runtime.Object, newConfigObjectListFunc func() runtime.Object) {
		f[key] = factory{
			key:                       key,
			newReceiverFunc:           newReceiverFunc,
			newReceiverObjectFunc:     newReceiverObjectFunc,
			newReceiverObjectListFunc: newReceiverObjectListFunc,
			newConfigObjectFunc:       newConfigObjectFunc,
			newConfigObjectListFunc:   newConfigObjectListFunc,
		}
	}

	register(dingtalk, NewDingTalkReceiver,
		func() runtime.Object {
			return &v1alpha1.DingTalkReceiver{}
		},
		func() runtime.Object {
			return &v1alpha1.DingTalkReceiverList{}
		},
		func() runtime.Object {
			return &v1alpha1.DingTalkConfig{}
		},
		func() runtime.Object {
			return &v1alpha1.DingTalkConfigList{}
		})
	register(email, NewEmailReceiver,
		func() runtime.Object {
			return &v1alpha1.EmailReceiver{}
		},
		func() runtime.Object {
			return &v1alpha1.EmailReceiverList{}
		},
		func() runtime.Object {
			return &v1alpha1.EmailConfig{}
		},
		func() runtime.Object {
			return &v1alpha1.EmailConfigList{}
		})
	register(slack, NewSlackReceiver,
		func() runtime.Object {
			return &v1alpha1.SlackReceiver{}
		},
		func() runtime.Object {
			return &v1alpha1.SlackReceiverList{}
		},
		func() runtime.Object {
			return &v1alpha1.SlackConfig{}
		},
		func() runtime.Object {
			return &v1alpha1.SlackConfigList{}
		})
	register(webhook, NewWebhookReceiver,
		func() runtime.Object {
			return &v1alpha1.WebhookReceiver{}
		},
		func() runtime.Object {
			return &v1alpha1.WebhookReceiverList{}
		},
		func() runtime.Object {
			return &v1alpha1.WebhookConfig{}
		},
		func() runtime.Object {
			return &v1alpha1.WebhookConfigList{}
		})
	register(wechat, NewWechatReceiver,
		func() runtime.Object {
			return &v1alpha1.WechatReceiver{}
		},
		func() runtime.Object {
			return &v1alpha1.WechatReceiverList{}
		},
		func() runtime.Object {
			return &v1alpha1.WechatConfig{}
		},
		func() runtime.Object {
			return &v1alpha1.WechatConfigList{}
		})

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
		resourceFactory:        f,
		receivers:              make(map[string]map[string]Receiver),
		ReceiverOpts:           nil,
		ch:                     make(chan *param, ChannelCapacity),
		nmNamespaces:           nmNamespaces,
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
	nmInf, err := c.cache.GetInformer(&v1alpha1.NotificationManager{})
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

	addInformer := func(f factory) error {
		informer, err := c.cache.GetInformer(f.newReceiverObjectFunc())
		if err != nil {
			_ = level.Error(c.logger).Log("msg", "Failed to get receiver informer", "receiver", f.key, "err", err)
			return err
		}
		informer.AddEventHandler(kcache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				c.onChange(obj, opAdd, f.key, false)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				c.onChange(newObj, opAdd, f.key, false)
			},
			DeleteFunc: func(obj interface{}) {
				c.onChange(obj, opDel, f.key, false)
			},
		})

		informer, err = c.cache.GetInformer(f.newConfigObjectFunc())
		if err != nil {
			_ = level.Error(c.logger).Log("msg", "Failed to get config informer", "config", f.key, "err", err)
			return err
		}
		informer.AddEventHandler(kcache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				c.onChange(obj, opAdd, f.key, true)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				c.onChange(newObj, opAdd, f.key, true)
			},
			DeleteFunc: func(obj interface{}) {
				c.onChange(obj, opDel, f.key, true)
			},
		})

		return nil
	}

	for _, f := range c.resourceFactory {
		if err := addInformer(f); err != nil {
			return err
		}
	}

	if ok := c.cache.WaitForCacheSync(c.ctx.Done()); !ok {
		return fmt.Errorf("NotificationManager cache failed")
	}

	_ = level.Info(c.logger).Log("msg", "Setting up informers successfully")
	return c.ctx.Err()
}

func (c *Config) onChange(obj interface{}, op, opType string, isConfig bool) {
	p := &param{
		op:       op,
		obj:      obj,
		opType:   opType,
		isConfig: isConfig,
	}

	p.done = make(chan interface{}, 1)
	c.ch <- p
	<-p.done
}

func (c *Config) sync(p *param) {

	if p.op == opGet {
		// Return all receivers of the specified tenant (map[opType/namespace/name]*Receiver)
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
	if len(p.tenantID) == 0 {
		p.done <- struct{}{}
		return
	}

	if p.op == opAdd {
		config := p.receiver.GetConfig()

		if p.isConfig {

			if reflect.ValueOf(config).IsNil() {
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
						if strings.HasPrefix(k, p.opType) && c.receivers[p.tenantID][k].UseDefault() {
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

			rcvKey := fmt.Sprintf("%s/%s/%s", p.opType, p.namespace, p.name)
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
			rcvKey := fmt.Sprintf("%s/%s/%s", p.opType, p.namespace, p.name)
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
		if p.ReceiverOpts != nil {
			c.ReceiverOpts = p.ReceiverOpts
		}
		c.nmNamespaces = p.nmNamespaces
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
				rcvs = append(rcvs, v)
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
				rcvs = append(rcvs, v)
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
						rcvs = append(rcvs, v)
					}
				}
			}
		}
	}

	return rcvs
}

func (c *Config) onNmAdd(obj interface{}) {
	if nm, ok := obj.(*v1alpha1.NotificationManager); ok {
		p := &param{}
		p.op = opAdd
		p.name = nm.Name
		p.namespace = nm.Namespace
		p.opType = notificationManager
		p.tenantKey = nm.Spec.Receivers.TenantKey
		p.defaultConfigSelector = nm.Spec.DefaultConfigSelector
		p.globalReceiverSelector = nm.Spec.Receivers.GlobalReceiverSelector
		p.tenantReceiverSelector = nm.Spec.Receivers.TenantReceiverSelector
		p.nmNamespaces = nm.Spec.NotificationManagerNamespaces
		if nm.Spec.Receivers.Options != nil {
			p.ReceiverOpts = nm.Spec.Receivers.Options
		}
		p.done = make(chan interface{}, 1)
		c.ch <- p
		<-p.done

		c.updateReloadtimestamp()
	}
}

func (c *Config) updateReloadtimestamp() {

	getObjects := func(objList runtime.Object) ([]runtime.Object, error) {

		if err := c.client.List(c.ctx, objList, client.InNamespace("")); err != nil {
			return nil, err
		}

		objs, err := meta.ExtractList(objList)
		if err != nil {
			return nil, err
		}

		return objs, nil
	}

	for key, f := range c.resourceFactory {
		var objs []runtime.Object
		receivers, err := getObjects(f.newReceiverObjectListFunc())
		if err != nil {
			_ = level.Error(c.logger).Log("msg", "Failed to list %s receiver", key, "err", err)
		}
		objs = append(objs, receivers...)

		configs, err := getObjects(f.newConfigObjectListFunc())
		if err != nil {
			_ = level.Error(c.logger).Log("msg", "Failed to list %s config", key, "err", err)
		}
		objs = append(objs, configs...)

		for _, obj := range objs {
			accessor, err := meta.Accessor(obj)
			if err != nil {
				_ = level.Warn(c.logger).Log("msg", "obj is not a meta object")
				continue
			}

			annotations := accessor.GetAnnotations()
			annotations["reloadtimestamp"] = time.Now().String()
			accessor.SetAnnotations(annotations)

			err = c.client.Update(c.ctx, obj)
			if err != nil {
				_ = level.Error(c.logger).Log("msg", "update %s error", key, "err", err)
				continue
			}
		}
	}
}

func (c *Config) onNmDel(obj interface{}) {
	if _, ok := obj.(*v1alpha1.NotificationManager); ok {
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

	runtimeObj, ok := p.obj.(runtime.Object)
	if !ok {
		_ = level.Warn(c.logger).Log("msg", "obj is not a runtime object")
		return
	}

	accessor, err := meta.Accessor(runtimeObj)
	if err != nil {
		_ = level.Warn(c.logger).Log("msg", "obj is not a meta object")
		return
	}

	p.name = accessor.GetName()
	p.namespace = accessor.GetNamespace()

	lbs := accessor.GetLabels()

	_ = level.Info(c.logger).Log("msg", "resource change", "op", p.op, "type", p.opType, "name", p.name, "namespace", p.namespace)

	// If crd's label matches globalSelector such as "type = global",
	// then this is a global receiver or config, and tenantID should be set to an unique tenantID
	if c.globalReceiverSelector != nil {
		for k, expected := range c.globalReceiverSelector.MatchLabels {
			if v, exists := lbs[k]; exists && v == expected {
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
			if v, exists := lbs[k]; exists && v == expected {
				if v, exists := lbs[c.tenantKey]; exists {
					p.tenantID = v
				}
				break
			}
		}
	}

	// If it is a config crd, update global default configs if crd's label match defaultConfigSelector
	if c.defaultConfigSelector != nil && p.isConfig {
		sel, _ := metav1.LabelSelectorAsSelector(c.defaultConfigSelector)
		if sel.Matches(labels.Set(lbs)) {
			p.tenantID = defaultConfig
		}
	}

	if len(p.tenantID) == 0 {
		_ = level.Warn(c.logger).Log("msg", "Ignore empty tenantID", "op", p.op, "type", p.opType, "tenantKey", c.tenantKey, "name", p.name, "namespace", p.namespace)
		return
	}

	if p.op == opAdd {

		f, ok := c.resourceFactory[p.opType]
		if !ok {
			_ = level.Warn(c.logger).Log("msg", "receiver type error", "op", p.op, "type", p.opType, "tenantKey", c.tenantKey, "name", p.name, "namespace", p.namespace)
			return
		}

		receiver := f.newReceiverFunc()
		if reflect.ValueOf(receiver).IsNil() {
			_ = level.Warn(c.logger).Log("msg", "generate receiver error", "op", p.op, "type", p.opType, "tenantKey", c.tenantKey, "name", p.name, "namespace", p.namespace)
			return
		}

		receiver.SetNamespace(p.namespace)

		if p.isConfig {
			receiver.GenerateConfig(c, p.obj)
		} else {
			receiver.GenerateReceiver(c, p.obj)
		}
		receiver.SetTenantID(p.tenantID)
		p.receiver = receiver
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

func (c *Config) GetSecretData(namespace string, selector *v1.SecretKeySelector) (string, error) {

	if selector == nil {
		return "", fmt.Errorf("SecretKeySelector is nil")
	}

	secret := v1.Secret{}
	if err := c.cache.Get(c.ctx, types.NamespacedName{Namespace: namespace, Name: selector.Name}, &secret); err != nil {
		return "", err
	}

	return string(secret.Data[selector.Key]), nil
}
