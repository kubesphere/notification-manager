package controller

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/pkg/apis/v2beta2"
	"github.com/kubesphere/notification-manager/pkg/constants"
	"github.com/kubesphere/notification-manager/pkg/internal"
	"github.com/kubesphere/notification-manager/pkg/template"
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
	"sigs.k8s.io/yaml"
)

const (
	globalTenantID   = "notification-manager/type/global"
	defaultConfig    = "notification-manager/type/default"
	defaultTenantKey = "user"

	opAdd         = "add"
	opUpdate      = "update"
	opDel         = "delete"
	nsEnvironment = "NAMESPACE"

	tenantSidecarURL = "http://localhost:19094/api/v2/tenant"
)

var (
	ChannelCapacity = 1000
)

type Controller struct {
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
	// Config for each tenant user, in form of map[tenantID]map[type/name]Config
	configs      map[string]map[string]internal.Config
	ReceiverOpts *v2beta2.Options
	// Channel to receive receiver create/update/delete operations and then update receivers
	ch chan *task
	// The pod's namespace
	namespace string
	history   *v2beta2.HistoryReceiver
	// Dose the notification manager crd add.
	nmAdd bool

	groupLabels  []string
	batchMaxSize int
	batchMaxWait metav1.Duration

	routePolicy string

	// Global template.
	template  *v2beta2.Template
	tmpl      *template.Template
	tmplMutex sync.Mutex
}

type task struct {
	op   string
	obj  interface{}
	run  func(t *task)
	done chan interface{}
}

func New(ctx context.Context, logger log.Logger) (*Controller, error) {
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

	return &Controller{
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

func (c *Controller) Run() error {
	go func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			case t, more := <-c.ch:
				if !more {
					return
				}
				t.run(t)
			}
		}
	}(c.ctx)
	go func() {
		_ = c.cache.Start(c.ctx.Done())
	}()

	if ok := c.cache.WaitForCacheSync(c.ctx.Done()); !ok {
		return utils.Error("NotificationManager cache failed")
	}

	// Setup informer for NotificationManager
	nmInf, err := c.cache.GetInformer(&v2beta2.NotificationManager{})
	if err != nil {
		_ = level.Error(c.logger).Log("msg", "Failed to get informer for NotificationManager", "err", err)
		return err
	}

	nmInf.AddEventHandler(kcache.ResourceEventHandlerFuncs{
		AddFunc: func(Obj interface{}) {
			c.onResourceChange(Obj, opAdd, c.nmChange)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.onResourceChange(newObj, opUpdate, c.nmChange)
		},
		DeleteFunc: func(Obj interface{}) {
			c.onResourceChange(Obj, opDel, c.nmChange)
		},
	})

	receiverInformer, err := c.cache.GetInformer(&v2beta2.Receiver{})
	if err != nil {
		_ = level.Error(c.logger).Log("msg", "Failed to get receiver informer", "err", err)
		return err
	}
	receiverInformer.AddEventHandler(kcache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.onResourceChange(obj, opAdd, c.receiverChanged)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.onResourceChange(newObj, opUpdate, c.receiverChanged)
		},
		DeleteFunc: func(obj interface{}) {
			c.onResourceChange(obj, opDel, c.receiverChanged)
		},
	})

	configInformer, err := c.cache.GetInformer(&v2beta2.Config{})
	if err != nil {
		_ = level.Error(c.logger).Log("msg", "Failed to get config informer", "err", err)
		return err
	}
	configInformer.AddEventHandler(kcache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.onResourceChange(obj, opAdd, c.configChanged)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.onResourceChange(newObj, opUpdate, c.configChanged)
		},
		DeleteFunc: func(obj interface{}) {
			c.onResourceChange(obj, opDel, c.configChanged)
		},
	})

	return c.ctx.Err()
}

func (c *Controller) onResourceChange(obj interface{}, op string, run func(t *task)) {
	t := &task{
		op:   op,
		obj:  obj,
		run:  run,
		done: make(chan interface{}, 1),
	}

	c.ch <- t
}

func (c *Controller) nmChange(t *task) {
	defer func() {
		_ = level.Info(c.logger).Log("msg", "notification manager changed", "op", t.op)
		close(t.done)
	}()

	if t.op == opDel {
		c.tenantKey = defaultTenantKey
		c.globalReceiverSelector = nil
		c.tenantReceiverSelector = nil
		c.defaultConfigSelector = nil
		c.ReceiverOpts = nil
		c.nmAdd = false

		return
	}

	spec := t.obj.(*v2beta2.NotificationManager).Spec
	needToReloadConfig := false
	needToReloadReceiver := false
	if c.tenantKey != spec.Receivers.TenantKey {
		c.tenantKey = spec.Receivers.TenantKey
		needToReloadConfig = true
		needToReloadReceiver = true
	}

	if !reflect.DeepEqual(c.defaultConfigSelector, spec.DefaultConfigSelector) {
		c.defaultConfigSelector = spec.DefaultConfigSelector
		needToReloadConfig = true
	}

	if !reflect.DeepEqual(c.tenantReceiverSelector, spec.Receivers.TenantReceiverSelector) {
		c.tenantReceiverSelector = spec.Receivers.TenantReceiverSelector
		needToReloadConfig = true
		needToReloadReceiver = true
	}

	if !reflect.DeepEqual(c.globalReceiverSelector, spec.Receivers.GlobalReceiverSelector) {
		c.globalReceiverSelector = spec.Receivers.GlobalReceiverSelector
		needToReloadReceiver = true
	}

	c.ReceiverOpts = spec.Receivers.Options
	c.tenantSidecar = false
	if spec.Sidecars != nil {
		if sidecar, ok := spec.Sidecars[v2beta2.Tenant]; ok && sidecar != nil {
			c.tenantSidecar = true
		}
	}

	c.history = spec.History
	c.groupLabels = spec.GroupLabels
	c.batchMaxSize = spec.BatchMaxSize
	c.batchMaxWait = spec.BatchMaxWait
	c.routePolicy = spec.RoutePolicy
	c.template = spec.Template
	c.nmAdd = true

	c.reload(needToReloadConfig, needToReloadReceiver)
}

// Reload all configs and receivers after notification manager changed.
func (c *Controller) reload(needToReloadConfig, needToReloadReceiver bool) {

	go func() {
		// Infinite loop until reload succeeds
		for {
			if needToReloadConfig {
				configList := v2beta2.ConfigList{}
				if err := c.client.List(c.ctx, &configList); err != nil {
					_ = level.Error(c.logger).Log("msg", "Failed to list config", "err", err)
					time.Sleep(time.Second)
					continue
				}

				for _, config := range configList.Items {
					nc := config
					t := &task{
						op:   opUpdate,
						obj:  &nc,
						run:  c.configChanged,
						done: make(chan interface{}, 1),
					}

					c.ch <- t
				}

				needToReloadConfig = false
				_ = level.Info(c.logger).Log("msg", "reload configs")
			}

			if needToReloadReceiver {
				receiverList := v2beta2.ReceiverList{}
				if err := c.client.List(c.ctx, &receiverList); err != nil {
					_ = level.Error(c.logger).Log("msg", "Failed to list receiver", "err", err)
					time.Sleep(time.Second)
					continue
				}

				for _, receiver := range receiverList.Items {
					nr := receiver
					t := &task{
						op:   opUpdate,
						obj:  &nr,
						run:  c.receiverChanged,
						done: make(chan interface{}, 1),
					}

					c.ch <- t
				}
				needToReloadReceiver = false
				_ = level.Info(c.logger).Log("msg", "reload receivers")
			}

			return
		}
	}()
}

func (c *Controller) configChanged(t *task) {

	defer close(t.done)

	if !c.nmAdd {
		return
	}

	config, ok := t.obj.(*v2beta2.Config)
	if !ok {
		_ = level.Warn(c.logger).Log("msg", "not a config object")
		return
	}

	newResourceVersion, _ := strconv.ParseUint(config.ResourceVersion, 10, 64)
	newTenantID := c.getTenantID(config.Labels)
	if len(newTenantID) == 0 {
		_ = level.Warn(c.logger).Log("msg", "Ignore config because of empty tenantID", "name", config.Name, "tenantKey", c.tenantKey)
		return
	}

	oldTenantID := ""
	oldResourceVersion := uint64(0)
	suffix := fmt.Sprintf("/%s", config.Name)
	for id := range c.configs {
		found := false
		for k := range c.configs[id] {
			if strings.HasSuffix(k, suffix) {
				oldTenantID = id
				oldResourceVersion = c.configs[id][k].GetResourceVersion()
				if t.op == opDel {
					delete(c.configs[id], k)
				}
				found = true
			}
		}

		if found {
			break
		}
	}

	if t.op == opDel {
		return
	}

	if newResourceVersion < oldResourceVersion {
		_ = level.Info(c.logger).Log("msg", "A newer config had added", "op", t.op, "name", config.Name)
		return
	}

	if newResourceVersion == oldResourceVersion && newTenantID == oldTenantID {
		_ = level.Debug(c.logger).Log("msg", "Config has no change", "op", t.op, "name", config.Name)
		return
	}

	// Delete the old config.
	if t.op == opUpdate {
		for k := range c.configs[oldTenantID] {
			delete(c.configs[oldTenantID], k)
		}
	}

	configs := NewConfigs(config)
	if _, ok := c.configs[newTenantID]; !ok {
		c.configs[newTenantID] = make(map[string]internal.Config)
	}
	for k, v := range configs {
		if !reflect2.IsNil(v) {
			c.configs[newTenantID][k] = v
		}
	}

	_ = level.Info(c.logger).Log("msg", "Config changed", "op", t.op, "name", config.Name)
}

func (c *Controller) receiverChanged(t *task) {
	defer close(t.done)

	if !c.nmAdd {
		return
	}

	receiver, ok := t.obj.(*v2beta2.Receiver)
	if !ok {
		_ = level.Warn(c.logger).Log("msg", "not a config object")
		return
	}

	newResourceVersion, _ := strconv.ParseUint(receiver.ResourceVersion, 10, 64)
	newTenantID := c.getTenantID(receiver.Labels)
	if len(newTenantID) == 0 {
		_ = level.Warn(c.logger).Log("msg", "Ignore receiver because of empty tenantID", "name", receiver.Name, "tenantKey", c.tenantKey)
		return
	}

	oldTenantID := ""
	oldResourceVersion := uint64(0)
	suffix := fmt.Sprintf("/%s", receiver.Name)
	for id := range c.receivers {
		found := false
		for k := range c.receivers[id] {
			if strings.HasSuffix(k, suffix) {
				if t.op == opDel {
					delete(c.receivers[id], k)
				}
				oldTenantID = id
				oldResourceVersion = c.receivers[id][k].GetResourceVersion()
				found = true
			}
		}

		if found {
			break
		}
	}

	if t.op == opDel {
		return
	}

	if newResourceVersion < oldResourceVersion {
		_ = level.Info(c.logger).Log("msg", "A newer receiver had added", "op", t.op, "name", receiver.Name)
		return
	}

	if newResourceVersion == oldResourceVersion && newTenantID == oldTenantID {
		_ = level.Debug(c.logger).Log("msg", "Receiver has no change", "op", t.op, "name", receiver.Name)
		return
	}

	// Delete the old receiver.
	if t.op == opUpdate {
		for k := range c.receivers[oldTenantID] {
			delete(c.receivers[oldTenantID], k)
		}
	}

	receivers := NewReceivers(newTenantID, receiver)
	if _, ok := c.receivers[newTenantID]; !ok {
		c.receivers[newTenantID] = make(map[string]internal.Receiver)
	}
	for k, v := range receivers {
		if !reflect2.IsNil(v) {
			c.receivers[newTenantID][k] = v
		}
	}

	_ = level.Info(c.logger).Log("msg", "Receiver changed", "op", t.op, "name", receiver.Name)
}

func (c *Controller) tenantIDFromNs(namespace string) ([]string, error) {
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

	tenantID := r.GetTenantID()
	configSelector := r.GetConfigSelector()
	if tenantID == globalTenantID {
		return match(configs[defaultConfig], configSelector)
	} else {
		if found := match(configs[tenantID], configSelector); !found {
			return match(configs[defaultConfig], nil)
		} else {
			return true
		}
	}
}

func (c *Controller) RcvsFromNs(namespace *string) []internal.Receiver {

	// Global receiver should receive all notifications.
	tenants := []string{globalTenantID}
	if namespace != nil && len(*namespace) > 0 {
		// Get all tenants which need to receive the notifications in this namespace.
		tenantIDs, err := c.tenantIDFromNs(*namespace)
		if err != nil {
			_ = level.Error(c.logger).Log("msg", "get tenantID error", "err", err, "namespace", *namespace)
		} else {
			tenants = append(tenants, tenantIDs...)
		}
	}

	t := &task{
		run: func(t *task) {
			var rcvs []internal.Receiver
			for _, tenant := range tenants {
				for _, rcv := range c.receivers[tenant] {
					if rcv.Enabled() {
						rcv = rcv.Clone()
						getMatchedConfig(rcv, c.configs)
						rcvs = append(rcvs, rcv)
					}
				}
			}

			t.done <- rcvs
		},
		done: make(chan interface{}, 1),
	}

	c.ch <- t
	val := <-t.done
	return val.([]internal.Receiver)
}

func (c *Controller) RcvsFromName(names []string, regexName, receiverType string) []internal.Receiver {

	t := &task{
		run: func(t *task) {
			var rcvs []internal.Receiver
			for k := range c.receivers {
				for _, v := range c.receivers[k] {
					if !utils.StringIsNil(receiverType) && v.GetType() != receiverType {
						continue
					}

					if utils.StringInList(v.GetName(), names) || utils.RegularMatch(regexName, v.GetName()) {
						if v.Enabled() {
							rcv := v.Clone()
							getMatchedConfig(rcv, c.configs)
							rcvs = append(rcvs, rcv)
						}
					}
				}
			}

			t.done <- rcvs
		},
		done: make(chan interface{}, 1),
	}

	c.ch <- t
	val := <-t.done
	return val.([]internal.Receiver)
}

func (c *Controller) RcvsFromSelector(selector *metav1.LabelSelector, receiverType string) []internal.Receiver {

	t := &task{
		run: func(t *task) {
			var rcvs []internal.Receiver
			for k := range c.receivers {
				for _, v := range c.receivers[k] {
					if !utils.StringIsNil(receiverType) && v.GetType() != receiverType {
						continue
					}

					if utils.LabelMatchSelector(v.GetLabels(), selector) {
						if v.Enabled() {
							rcv := v.Clone()
							getMatchedConfig(rcv, c.configs)
							rcvs = append(rcvs, rcv)
						}
					}
				}
			}

			t.done <- rcvs
		},
		done: make(chan interface{}, 1),
	}

	c.ch <- t
	val := <-t.done
	return val.([]internal.Receiver)
}

func (c *Controller) getTenantID(label map[string]string) string {
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
func (c *Controller) isGlobal(label map[string]string) bool {

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
func (c *Controller) isDefaultConfig(label map[string]string) bool {

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
func (c *Controller) isTenant(label map[string]string) (bool, string) {

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

func (c *Controller) ListReceiver(tenant, opType string) interface{} {

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

func (c *Controller) ListConfig(tenant, opType string) interface{} {

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

func (c *Controller) ListReceiverWithConfig(tenantID, name, opType string) interface{} {

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

func (c *Controller) GetCredential(credential *v2beta2.Credential) (string, error) {

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
// If the notification config is nil, use the existing config.
// If the notification config is not nil, the receiver will use the given config,
// the notification config type must match the notification receiver type.
func (c *Controller) GenerateReceivers(nr *v2beta2.Receiver, nc *v2beta2.Config) ([]internal.Receiver, error) {

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

	if len(rcvs) == 0 {
		return nil, utils.Error("no receivers provided")
	}

	return rcvs, nil
}

func (c *Controller) GetHistoryReceivers() []internal.Receiver {

	var rcvs []internal.Receiver

	if c.history != nil {
		receiver := &v2beta2.Receiver{
			Spec: v2beta2.ReceiverSpec{
				Webhook: c.history.Webhook,
			},
		}
		tmpl := constants.DefaultWebhookTemplate
		receiver.Spec.Webhook.Template = &tmpl
		historyReceivers := NewReceivers("", receiver)
		if historyReceivers != nil {
			for _, v := range historyReceivers {
				if v.Enabled() {
					rcvs = append(rcvs, v)
				}
			}
		}
	}

	return rcvs
}

func (c *Controller) GetGroupLabels() []string {
	return c.groupLabels
}

func (c *Controller) GetBatchMaxSize() int {
	return c.batchMaxSize
}

func (c *Controller) GetBatchMaxWait() time.Duration {
	return c.batchMaxWait.Duration
}

func (c *Controller) GetActiveSilences(ctx context.Context, tenant string) ([]v2beta2.Silence, error) {

	var selector *metav1.LabelSelector
	// Get global silence.
	if utils.StringIsNil(tenant) {
		selector = c.globalReceiverSelector.DeepCopy()
	} else {
		// Get tenant silence.
		selector = c.tenantReceiverSelector.DeepCopy()
		selector.MatchLabels[c.tenantKey] = tenant
	}

	list := &v2beta2.SilenceList{}
	if err := c.cache.List(ctx, list, &client.ListOptions{LabelSelector: labels.SelectorFromSet(selector.MatchLabels)}); err != nil {
		return nil, err
	}

	var ss []v2beta2.Silence
	for _, silence := range list.Items {
		if silence.IsActive() {
			ss = append(ss, silence)
		}
	}

	return ss, nil
}

func (c *Controller) GetActiveRouters(ctx context.Context) ([]v2beta2.Router, error) {

	list := &v2beta2.RouterList{}
	if err := c.cache.List(ctx, list); err != nil {
		return nil, err
	}

	var rs []v2beta2.Router
	for _, router := range list.Items {
		if router.Spec.Enabled == nil || *router.Spec.Enabled {
			rs = append(rs, router)
		}
	}

	return rs, nil
}

func (c *Controller) GetRoutePolicy() string {
	return c.routePolicy
}

func (c *Controller) GetConfigmap(configmaps ...*v2beta2.ConfigmapKeySelector) ([]string, error) {
	if len(configmaps) == 0 {
		return nil, nil
	}

	var res []string
	for _, configmap := range configmaps {

		if configmap == nil {
			continue
		}

		ns := configmap.Namespace
		if len(ns) == 0 {
			ns = c.namespace
		}

		cm := v1.ConfigMap{}
		if err := c.cache.Get(c.ctx, types.NamespacedName{Namespace: ns, Name: configmap.Name}, &cm); err != nil {
			return nil, err
		}

		if utils.StringIsNil(configmap.Key) {
			for _, v := range cm.Data {
				res = append(res, v)
			}
		} else {
			if val, ok := cm.Data[configmap.Key]; !ok {
				return nil, utils.Errorf("'%s' is not found in configmap %s/%s", configmap.Key, cm.Name, ns)
			} else {
				res = append(res, val)
			}
		}
	}

	return res, nil
}

func (c *Controller) GetGlobalTmpl() (*template.Template, error) {

	c.tmplMutex.Lock()
	defer c.tmplMutex.Unlock()

	if c.tmpl != nil {
		if c.template == nil || !c.tmpl.Expired(c.template.ReloadCycle.Duration) {
			return c.tmpl.Clone(), nil
		}
	}

	var err error
	var tmpl *template.Template
	if c.template == nil {
		tmpl, err = template.New("", nil)
	} else {
		pack, err := c.GetConfigmap(c.template.LanguagePack...)
		if err != nil {
			return nil, err
		}

		tmpl, err = template.New(c.template.Language, pack)
	}

	if err != nil {
		return nil, err
	}

	if c.ReceiverOpts != nil && c.ReceiverOpts.Global != nil && len(c.ReceiverOpts.Global.TemplateFiles) > 0 {
		tmpl, err = tmpl.ParserFile(c.ReceiverOpts.Global.TemplateFiles...)
		if err != nil {
			return nil, err
		}
	}

	if c.template != nil {
		text, err := c.GetConfigmap(c.template.Text)
		if err != nil {
			return nil, err
		}

		tmpl, err = tmpl.ParserText(text...)
		if err != nil {
			return nil, err
		}
	}

	c.tmpl = tmpl

	return c.tmpl.Clone(), nil
}

func (c *Controller) GetReceiverTmpl(cm *v2beta2.ConfigmapKeySelector) (*template.Template, error) {
	text, err := c.GetConfigmap(cm)
	if err != nil {
		return nil, err
	}

	globalTmpl, err := c.GetGlobalTmpl()
	if err != nil {
		return nil, err
	}

	return globalTmpl.ParserText(text...)
}

type clusterConfig struct {
	MultiCluster multiCluster `json:"multicluster,omitempty" yaml:"multicluster,omitempty"`
}

type multiCluster struct {
	ClusterName string `json:"clusterName,omitempty" yaml:"clusterName,omitempty"`
}

// GetCluster returns the name of the cluster where the notification manager is located.
// If a cluster name is specified, the specified cluster name is used.
// Otherwise, for kubespere platforms (version >= 3.3), it will try to get the cluster name
// from the configmap kubesphere-config.
// Otherwise, the default cluster name "default" will be returned.
func (c *Controller) GetCluster() string {

	if c.ReceiverOpts != nil && c.ReceiverOpts.Global != nil && !utils.StringIsNil(c.ReceiverOpts.Global.Cluster) {
		return c.ReceiverOpts.Global.Cluster
	}

	files, err := c.GetConfigmap(&v2beta2.ConfigmapKeySelector{
		Namespace: constants.KubesphereConfigNamespace,
		Name:      constants.KubesphereConfigName,
		Key:       constants.KubesphereConfigKey,
	})
	if err != nil {
		_ = level.Debug(c.logger).Log("msg", "get kubesphere config configmap error", "err", err)
		return constants.DefaultClusterName
	}

	if len(files) == 0 {
		_ = level.Debug(c.logger).Log("msg", "get kubesphere cluster name error, key is not found")
		return constants.DefaultClusterName
	}

	cc := &clusterConfig{}
	if err := yaml.Unmarshal([]byte(files[0]), cc); err != nil {
		_ = level.Debug(c.logger).Log("msg", "decode kubesphere config error", "error", err)
		return constants.DefaultClusterName
	}

	if !utils.StringIsNil(cc.MultiCluster.ClusterName) {
		return cc.MultiCluster.ClusterName
	}

	return constants.DefaultClusterName
}
