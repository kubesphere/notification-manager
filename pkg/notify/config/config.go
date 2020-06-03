package config

import (
	"context"
	"fmt"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	nmv1alpha1 "github.com/kubesphere/notification-manager/pkg/apis/v1alpha1"
	"github.com/prometheus/alertmanager/config"
	"k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	kcache "k8s.io/client-go/tools/cache"
	"net/url"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	kconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
	"strings"
	"sync"
	"time"
)

const (
	user                = "User"
	globalTenantID      = "notification-manager/type/global"
	defaultConfig       = "notification-manager/type/default"
	defaultTenantKey    = "user"
	notificationManager = "notification-manager"
	emailReceiver       = "email"
	emailConfig         = "email-config"
	wechatReceiver      = "wechat"
	wechatConfig        = "wechat-config"
	slackReceiver       = "slack"
	slackConfig         = "slack-config"
	//webhookReceiver     = "webhook"
	//webhookConfig       = "webhook-config"
	opAdd              = "add"
	opDel              = "delete"
	opGet              = "get"
	tenantKeyNamespace = "namespace"
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
	defaultConfig *Receiver
	// Label key used to distinguish different user
	tenantKey string
	// Label selector to filter valid global Receiver CR
	globalReceiverSelector *metav1.LabelSelector
	// Label selector to filter valid tenant Receiver CR
	tenantReceiverSelector *metav1.LabelSelector
	// Receiver config for each tenant user, in form of map[tenantID]map[type/namespace/name]*Receiver
	receivers    map[string]map[string]*Receiver
	ReceiverOpts *nmv1alpha1.Options
	// Channel to receive receiver create/update/delete operations and then update receivers
	ch                chan *param
	monitorNamespaces []string
}

type Email struct {
	To          []string
	EmailConfig *config.EmailConfig
	// True means receiver use the default config.
	UseDefault bool
}

type Wechat struct {
	ToUser       string
	ToParty      string
	ToTag        string
	WechatConfig *config.WechatConfig
	// True means receiver use the default config.
	UseDefault bool
}

type Slack struct {
	// The channel or user to send notifications to.
	Channel     string
	SlackConfig *SlackConfig
	// True means receiver use the default config.
	UseDefault bool
}

type SlackConfig struct {
	// The token of user or bot.
	Token string
}

type Webhook struct {
	WebhookConfig *config.WebhookConfig
	// True means receiver use the default config.
	UseDefault bool
}

type Receiver struct {
	TenantID *string
	Email    *Email
	Wechat   *Wechat
	Slack    *Slack
	Webhook  *Webhook
}

type param struct {
	op                     string
	tenantID               string
	opType                 string
	namespace              string
	name                   string
	defaultConfig          *Receiver
	tenantKey              string
	defaultConfigSelector  *metav1.LabelSelector
	tenantReceiverSelector *metav1.LabelSelector
	globalReceiverSelector *metav1.LabelSelector
	receiver               *Receiver
	ReceiverOpts           *nmv1alpha1.Options
	monitorNamespaces      []string
	done                   chan interface{}
}

type changeEvent struct {
	obj       interface{}
	name      string
	namespace string
	op        string
	opType    string
	labels    map[string]string
	isConfig  bool
}

func New(ctx context.Context, logger log.Logger, namespaces string) (*Config, error) {
	scheme := runtime.NewScheme()
	_ = nmv1alpha1.AddToScheme(scheme)
	_ = v1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)

	cfg, err := kconfig.GetConfig()
	if err != nil {
		_ = level.Error(logger).Log("msg", "Failed to get kubeconfig ", "err", err)
	}

	var monitorNamespaces []string
	if len(namespaces) > 0 {
		monitorNamespaces = strings.Split(namespaces, ":")
	}

	ncf := cache.MultiNamespacedCacheBuilder(monitorNamespaces)
	informerCache, err := ncf(cfg, cache.Options{
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

	return &Config{
		ctx:                    ctx,
		logger:                 logger,
		cache:                  informerCache,
		client:                 c,
		defaultConfig:          &Receiver{},
		tenantKey:              defaultTenantKey,
		defaultConfigSelector:  nil,
		tenantReceiverSelector: nil,
		globalReceiverSelector: nil,
		receivers:              make(map[string]map[string]*Receiver),
		ReceiverOpts:           nil,
		ch:                     make(chan *param, ChannelCapacity),
		monitorNamespaces:      monitorNamespaces,
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
	nmInf, err := c.cache.GetInformer(&nmv1alpha1.NotificationManager{})
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

	// Setup informer for EmailConfig
	mailConfInf, err := c.cache.GetInformer(&nmv1alpha1.EmailConfig{})
	if err != nil {
		_ = level.Error(c.logger).Log("msg", "Failed to get informer for EmailConfig", "err", err)
		return err
	}
	mailConfInf.AddEventHandler(kcache.ResourceEventHandlerFuncs{
		AddFunc: c.onMailConfAdd,
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.onMailConfAdd(newObj)
		},
		DeleteFunc: c.onMailConfDel,
	})

	// Setup informer for EmailReceiver
	mailRcvInf, err := c.cache.GetInformer(&nmv1alpha1.EmailReceiver{})
	if err != nil {
		_ = level.Error(c.logger).Log("msg", "Failed to get informer for EmailReceiver", "err", err)
		return err
	}
	mailRcvInf.AddEventHandler(kcache.ResourceEventHandlerFuncs{
		AddFunc: c.onMailRcvAdd,
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.onMailRcvAdd(newObj)
		},
		DeleteFunc: c.onMailRcvDel,
	})

	// Setup informer for WechatConfig
	wechatConfInf, err := c.cache.GetInformer(&nmv1alpha1.WechatConfig{})
	if err != nil {
		_ = level.Error(c.logger).Log("msg", "Failed to get informer for WechatConfig", "err", err)
		return err
	}
	wechatConfInf.AddEventHandler(kcache.ResourceEventHandlerFuncs{
		AddFunc: c.onWechatConfAdd,
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.onWechatConfAdd(newObj)
		},
		DeleteFunc: c.onWechatConfDel,
	})

	// Setup informer for WechatReceiver
	wechatRcvInf, err := c.cache.GetInformer(&nmv1alpha1.WechatReceiver{})
	if err != nil {
		_ = level.Error(c.logger).Log("msg", "Failed to get informer for WechatReceiver", "err", err)
		return err
	}
	wechatRcvInf.AddEventHandler(kcache.ResourceEventHandlerFuncs{
		AddFunc: c.onWechatRcvAdd,
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.onWechatRcvAdd(newObj)
		},
		DeleteFunc: c.onWechatRcvDel,
	})

	// Setup informer for SlackConfig
	slackConfInf, err := c.cache.GetInformer(&nmv1alpha1.SlackConfig{})
	if err != nil {
		_ = level.Error(c.logger).Log("msg", "Failed to get informer for SlackConfig", "err", err)
		return err
	}
	slackConfInf.AddEventHandler(kcache.ResourceEventHandlerFuncs{
		AddFunc: c.onSlackConfAdd,
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.onSlackConfAdd(newObj)
		},
		DeleteFunc: c.onSlackConfDel,
	})

	// Setup informer for SlackReceiver
	slackRcvInf, err := c.cache.GetInformer(&nmv1alpha1.SlackReceiver{})
	if err != nil {
		_ = level.Error(c.logger).Log("msg", "Failed to get informer for SlackReceiver", "err", err)
		return err
	}
	slackRcvInf.AddEventHandler(kcache.ResourceEventHandlerFuncs{
		AddFunc: c.onSlackRcvAdd,
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.onSlackRcvAdd(newObj)
		},
		DeleteFunc: c.onSlackRcvDel,
	})

	if ok := c.cache.WaitForCacheSync(c.ctx.Done()); !ok {
		return fmt.Errorf("NotificationManager cache failed")
	}

	_ = level.Info(c.logger).Log("msg", "Setting up informers successfully")
	return c.ctx.Err()
}

func (c *Config) sync(p *param) {
	switch p.op {
	case opGet:
		// Return all receivers of the specified tenant (map[opType/namespace/name]*Receiver)
		// via the done channel if exists
		if v, exist := c.receivers[p.tenantID]; exist {
			p.done <- v
			// Return empty struct if receivers of the specified tenant cannot be found
		} else {
			p.done <- struct{}{}
		}
		return
	case opAdd:
		switch p.opType {
		case notificationManager:
			c.tenantKey = p.tenantKey
			c.defaultConfigSelector = p.defaultConfigSelector
			c.tenantReceiverSelector = p.tenantReceiverSelector
			c.globalReceiverSelector = p.globalReceiverSelector
			if p.ReceiverOpts != nil {
				c.ReceiverOpts = p.ReceiverOpts
			}
			c.monitorNamespaces = p.monitorNamespaces
		case emailReceiver:
			// Setup EmailConfig with default if emailconfig cannot be found
			if p.receiver.Email.EmailConfig == nil {
				p.receiver.Email.UseDefault = true
				if c.defaultConfig.Email != nil {
					p.receiver.Email.EmailConfig = c.defaultConfig.Email.EmailConfig
				}
			}
			rcvKey := fmt.Sprintf("%s/%s/%s", emailReceiver, p.namespace, p.name)
			if _, exist := c.receivers[p.tenantID]; exist {
				c.receivers[p.tenantID][rcvKey] = p.receiver
			} else if len(p.tenantID) > 0 {
				c.receivers[p.tenantID] = make(map[string]*Receiver)
				c.receivers[p.tenantID][rcvKey] = p.receiver
			}
		case emailConfig:
			// Setup default email config
			if p.defaultConfig != nil && p.defaultConfig.Email != nil {
				c.defaultConfig.Email = p.defaultConfig.Email
				// Update EmailConfig of the receivers who use default email config
				for tenantID := range c.receivers {
					for k := range c.receivers[tenantID] {
						if c.receivers[tenantID][k].Email != nil && c.receivers[tenantID][k].Email.UseDefault {
							c.receivers[tenantID][k].Email.EmailConfig = p.defaultConfig.Email.EmailConfig
						}
					}
				}
			}

			// Update tenant email config
			if p.receiver != nil && p.receiver.Email != nil && p.receiver.Email.EmailConfig != nil {
				// Update EmailConfig of the recerver with the same tenantID
				if _, exist := c.receivers[p.tenantID]; exist {
					for k := range c.receivers[p.tenantID] {
						if c.receivers[p.tenantID][k].Email != nil {
							c.receivers[p.tenantID][k].Email.EmailConfig = p.receiver.Email.EmailConfig
							c.receivers[p.tenantID][k].Email.UseDefault = false
						}
					}
				}
			}
		case wechatReceiver:
			// Setup WechatConfig with global default if wechatconfig cannot be found
			if p.receiver.Wechat.WechatConfig == nil && c.defaultConfig.Wechat != nil {
				p.receiver.Wechat.WechatConfig = c.defaultConfig.Wechat.WechatConfig
			}
			rcvKey := fmt.Sprintf("%s/%s/%s", wechatReceiver, p.namespace, p.name)
			if _, exist := c.receivers[p.tenantID]; exist {
				c.receivers[p.tenantID][rcvKey] = p.receiver
			} else if len(p.tenantID) > 0 {
				c.receivers[p.tenantID] = make(map[string]*Receiver)
				c.receivers[p.tenantID][rcvKey] = p.receiver
			}
		case wechatConfig:
			// Setup default wechat config
			if p.defaultConfig != nil && p.defaultConfig.Wechat != nil {
				c.defaultConfig.Wechat = p.defaultConfig.Wechat
				// Update WechatConfig of the receivers who use default wechat config
				for tenantID := range c.receivers {
					for k := range c.receivers[tenantID] {
						if c.receivers[tenantID][k].Wechat != nil && c.receivers[tenantID][k].Wechat.UseDefault {
							c.receivers[tenantID][k].Wechat.WechatConfig = p.defaultConfig.Wechat.WechatConfig
						}
					}
				}
			}

			// Update tenant wechat config
			if p.receiver != nil && p.receiver.Wechat != nil && p.receiver.Wechat.WechatConfig != nil {
				// Update WechatConfig of the recerver with the same tenantID
				if _, exist := c.receivers[p.tenantID]; exist {
					for k := range c.receivers[p.tenantID] {
						if c.receivers[p.tenantID][k].Wechat != nil {
							c.receivers[p.tenantID][k].Wechat.WechatConfig = p.receiver.Wechat.WechatConfig
							c.receivers[p.tenantID][k].Wechat.UseDefault = false
						}
					}
				}
			}
		case slackReceiver:
			// Setup SlackConfig with global default if slackReceiver cannot be found
			if p.receiver.Slack.SlackConfig == nil && c.defaultConfig.Slack != nil {
				p.receiver.Slack.SlackConfig = c.defaultConfig.Slack.SlackConfig
			}
			rcvKey := fmt.Sprintf("%s/%s/%s", slackReceiver, p.namespace, p.name)
			if _, exist := c.receivers[p.tenantID]; exist {
				c.receivers[p.tenantID][rcvKey] = p.receiver
			} else if len(p.tenantID) > 0 {
				c.receivers[p.tenantID] = make(map[string]*Receiver)
				c.receivers[p.tenantID][rcvKey] = p.receiver
			}
		case slackConfig:
			// Setup default slack config
			if p.defaultConfig != nil && p.defaultConfig.Slack != nil {
				c.defaultConfig.Slack = p.defaultConfig.Slack
				// Update SlackConfig of the receivers who use default slack config
				for tenantID := range c.receivers {
					for k := range c.receivers[tenantID] {
						if c.receivers[tenantID][k].Slack != nil && c.receivers[tenantID][k].Slack.UseDefault {
							c.receivers[tenantID][k].Slack.SlackConfig = p.defaultConfig.Slack.SlackConfig
						}
					}
				}
			}

			// Update tenant slack config
			if p.receiver != nil && p.receiver.Slack != nil && p.receiver.Slack.SlackConfig != nil {
				// Update SlackConfig of the recerver with the same tenantID
				if _, exist := c.receivers[p.tenantID]; exist {
					for k := range c.receivers[p.tenantID] {
						if c.receivers[p.tenantID][k].Slack != nil {
							c.receivers[p.tenantID][k].Slack.SlackConfig = p.receiver.Slack.SlackConfig
							c.receivers[p.tenantID][k].Slack.UseDefault = false
						}
					}
				}
			}
		default:
		}
	case opDel:
		switch p.opType {
		case notificationManager:
			c.tenantKey = defaultTenantKey
			c.globalReceiverSelector = nil
			c.tenantReceiverSelector = nil
			c.defaultConfigSelector = nil
			c.ReceiverOpts = nil
		case emailReceiver:
			rcvKey := fmt.Sprintf("%s/%s/%s", emailReceiver, p.namespace, p.name)
			if _, exist := c.receivers[p.tenantID]; exist {
				delete(c.receivers[p.tenantID], rcvKey)
				if len(c.receivers[p.tenantID]) <= 0 {
					delete(c.receivers, p.tenantID)
				}
			}
		case emailConfig:
			if p.tenantID == defaultConfig {
				// Reset default email config
				c.defaultConfig.Email = nil
				// Delete EmailConfig of recervers who use default email config.
				for tenantID := range c.receivers {
					for k := range c.receivers[tenantID] {
						if c.receivers[tenantID][k].Email != nil && c.receivers[tenantID][k].Email.UseDefault {
							c.receivers[tenantID][k].Email.EmailConfig = nil
						}
					}
				}
			} else {
				// Delete EmailConfig of the recerver with the same tenantID by setting the EmailConfig to nil
				if _, exist := c.receivers[p.tenantID]; exist {
					for k := range c.receivers[p.tenantID] {
						if c.receivers[p.tenantID][k].Email != nil {
							c.receivers[p.tenantID][k].Email.EmailConfig = nil
							c.receivers[p.tenantID][k].Email.UseDefault = true
							if c.defaultConfig.Email != nil {
								c.receivers[p.tenantID][k].Email.EmailConfig = c.defaultConfig.Email.EmailConfig
							}
						}
					}
				}
			}
		case wechatReceiver:
			rcvKey := fmt.Sprintf("%s/%s/%s", wechatReceiver, p.namespace, p.name)
			if _, exist := c.receivers[p.tenantID]; exist {
				delete(c.receivers[p.tenantID], rcvKey)
				if len(c.receivers[p.tenantID]) <= 0 {
					delete(c.receivers, p.tenantID)
				}
			}
		case wechatConfig:
			if p.tenantID == defaultConfig {
				// Reset default wechat config
				c.defaultConfig.Wechat = nil
				// Delete WechatConfig of recervers who use default wechat config.
				for tenantID := range c.receivers {
					for k := range c.receivers[tenantID] {
						if c.receivers[tenantID][k].Wechat != nil && c.receivers[tenantID][k].Wechat.UseDefault {
							c.receivers[tenantID][k].Wechat.WechatConfig = nil
						}
					}
				}
			} else {
				// Delete WechatConfig of the recerver with the same tenantID by setting the WechatConfig to nil
				if _, exist := c.receivers[p.tenantID]; exist {
					for k := range c.receivers[p.tenantID] {
						if c.receivers[p.tenantID][k].Wechat != nil {
							c.receivers[p.tenantID][k].Wechat.WechatConfig = nil
							c.receivers[p.tenantID][k].Wechat.UseDefault = true
							if c.defaultConfig.Wechat != nil {
								c.receivers[p.tenantID][k].Wechat.WechatConfig = c.defaultConfig.Wechat.WechatConfig
							}
						}
					}
				}
			}
		case slackReceiver:
			rcvKey := fmt.Sprintf("%s/%s/%s", slackReceiver, p.namespace, p.name)
			if _, exist := c.receivers[p.tenantID]; exist {
				delete(c.receivers[p.tenantID], rcvKey)
				if len(c.receivers[p.tenantID]) <= 0 {
					delete(c.receivers, p.tenantID)
				}
			}
		case slackConfig:
			if p.tenantID == defaultConfig {
				// Reset default slack config
				c.defaultConfig.Slack = nil
				// Delete SlackConfig of recervers who use default slack config.
				for tenantID := range c.receivers {
					for k := range c.receivers[tenantID] {
						if c.receivers[tenantID][k].Slack != nil && c.receivers[tenantID][k].Slack.UseDefault {
							c.receivers[tenantID][k].Slack.SlackConfig = nil
						}
					}
				}
			} else {
				// Delete SlackConfig of the recerver with the same tenantID by setting the SlackConfig to nil
				if _, exist := c.receivers[p.tenantID]; exist {
					for k := range c.receivers[p.tenantID] {
						if c.receivers[p.tenantID][k].Slack != nil {
							c.receivers[p.tenantID][k].Slack.SlackConfig = nil
							c.receivers[p.tenantID][k].Slack.UseDefault = true
							if c.defaultConfig.Wechat != nil {
								c.receivers[p.tenantID][k].Slack.SlackConfig = c.defaultConfig.Slack.SlackConfig
							}
						}
					}
				}
			}
		default:
		}
	default:
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

func (c *Config) RcvsFromNs(namespace *string) []*Receiver {
	rcvs := make([]*Receiver, 0)
	// Return global receiver if namespace is nil, global receiver should receive all notifications
	if namespace == nil {
		p := param{}
		p.op = opGet
		p.tenantID = globalTenantID
		p.done = make(chan interface{}, 1)
		c.ch <- &p
		o := <-p.done
		if r, ok := o.(map[string]*Receiver); ok {
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
		if r, ok := o.(map[string]*Receiver); ok {
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
				if r, ok := o.(map[string]*Receiver); ok {
					for _, v := range r {
						rcvs = append(rcvs, v)
					}
				}
			}
		}
	}

	// TODO: Add receiver generation logic for wechat, slack and webhook
	return rcvs
}

func (c *Config) onNmAdd(obj interface{}) {
	if nm, ok := obj.(*nmv1alpha1.NotificationManager); ok {
		p := &param{}
		p.op = opAdd
		p.name = nm.Name
		p.namespace = nm.Namespace
		p.opType = notificationManager
		p.tenantKey = nm.Spec.Receivers.TenantKey
		p.defaultConfigSelector = nm.Spec.DefaultConfigSelector
		p.globalReceiverSelector = nm.Spec.Receivers.GlobalReceiverSelector
		p.tenantReceiverSelector = nm.Spec.Receivers.TenantReceiverSelector
		p.monitorNamespaces = nm.Spec.MonitorNamespaces
		if nm.Spec.Receivers.Options != nil {
			p.ReceiverOpts = nm.Spec.Receivers.Options
		}
		p.done = make(chan interface{}, 1)
		c.ch <- p
		<-p.done

		// Update receiver and config CRs to trigger update of receivers
		wg := sync.WaitGroup{}
		wg.Add(6)
		go c.updateEmailReceivers(&wg)
		go c.updateEmailConfigs(&wg)
		go c.updateWechatConfigs(&wg)
		go c.updateWechatReceivers(&wg)
		go c.updateSlackConfigs(&wg)
		go c.updateSlackReceivers(&wg)
		wg.Wait()
	}
}

func (c *Config) updateEmailReceivers(wg *sync.WaitGroup) {
	defer wg.Done()

	mrList := nmv1alpha1.EmailReceiverList{}
	if err := c.cache.List(c.ctx, &mrList, client.InNamespace("")); err != nil {
		_ = level.Error(c.logger).Log("msg", "Failed to list EmailReceiver", "err", err)
		return
	}
	for _, mr := range mrList.Items {
		r := mr.DeepCopy()
		r.ObjectMeta.Annotations["reloadtimestamp"] = time.Now().String()
		if err := c.client.Update(c.ctx, r); err != nil {
			_ = level.Error(c.logger).Log("msg", "Failed to update EmailReceiver", "err", err)
		}
	}

	return
}

func (c *Config) updateEmailConfigs(wg *sync.WaitGroup) {
	defer wg.Done()

	mcList := nmv1alpha1.EmailConfigList{}
	if err := c.cache.List(c.ctx, &mcList, client.InNamespace("")); err != nil {
		_ = level.Error(c.logger).Log("msg", "Failed to list EmailConfig", "err", err)
		return
	}
	for _, mc := range mcList.Items {
		cfg := mc.DeepCopy()
		cfg.ObjectMeta.Annotations["reloadtimestamp"] = time.Now().String()
		if err := c.client.Update(c.ctx, cfg); err != nil {
			_ = level.Error(c.logger).Log("msg", "Failed to update EmailConfig", "err", err)
		}
	}

	return
}

func (c *Config) updateWechatReceivers(wg *sync.WaitGroup) {
	defer wg.Done()

	wrList := nmv1alpha1.WechatReceiverList{}
	if err := c.cache.List(c.ctx, &wrList, client.InNamespace("")); err != nil {
		_ = level.Error(c.logger).Log("msg", "Failed to list WechatReceiver", "err", err)
		return
	}
	for _, wr := range wrList.Items {
		r := wr.DeepCopy()
		r.ObjectMeta.Annotations["reloadtimestamp"] = time.Now().String()
		if err := c.client.Update(c.ctx, r); err != nil {
			_ = level.Error(c.logger).Log("msg", "Failed to update WechatReceiver", "err", err)
		}
	}

	return
}

func (c *Config) updateWechatConfigs(wg *sync.WaitGroup) {
	defer wg.Done()

	wcList := nmv1alpha1.WechatConfigList{}
	if err := c.cache.List(c.ctx, &wcList, client.InNamespace("")); err != nil {
		_ = level.Error(c.logger).Log("msg", "Failed to list WechatConfig", "err", err)
		return
	}
	for _, wc := range wcList.Items {
		cfg := wc.DeepCopy()
		cfg.ObjectMeta.Annotations["reloadtimestamp"] = time.Now().String()
		if err := c.client.Update(c.ctx, cfg); err != nil {
			_ = level.Error(c.logger).Log("msg", "Failed to update WechatConfig", "err", err)
		}
	}

	return
}

func (c *Config) updateSlackReceivers(wg *sync.WaitGroup) {
	defer wg.Done()

	srList := nmv1alpha1.SlackReceiverList{}
	if err := c.cache.List(c.ctx, &srList, client.InNamespace("")); err != nil {
		_ = level.Error(c.logger).Log("msg", "Failed to list SlackReceiver", "err", err)
		return
	}
	for _, sr := range srList.Items {
		r := sr.DeepCopy()
		r.ObjectMeta.Annotations["reloadtimestamp"] = time.Now().String()
		if err := c.client.Update(c.ctx, r); err != nil {
			_ = level.Error(c.logger).Log("msg", "Failed to update SlackReceiver", "err", err)
		}
	}

	return
}

func (c *Config) updateSlackConfigs(wg *sync.WaitGroup) {
	defer wg.Done()

	scList := nmv1alpha1.SlackConfigList{}
	if err := c.cache.List(c.ctx, &scList, client.InNamespace("")); err != nil {
		_ = level.Error(c.logger).Log("msg", "Failed to list SlackConfig", "err", err)
		return
	}
	for _, sc := range scList.Items {
		cfg := sc.DeepCopy()
		cfg.ObjectMeta.Annotations["reloadtimestamp"] = time.Now().String()
		if err := c.client.Update(c.ctx, cfg); err != nil {
			_ = level.Error(c.logger).Log("msg", "Failed to update SlackConfig", "err", err)
		}
	}

	return
}

func (c *Config) onNmDel(obj interface{}) {
	if _, ok := obj.(*nmv1alpha1.NotificationManager); ok {
		p := &param{}
		p.op = opDel
		p.opType = notificationManager
		p.done = make(chan interface{}, 1)
		c.ch <- p
		<-p.done
	}
}

func (c *Config) onMailConfAdd(obj interface{}) {
	if mc, ok := obj.(*nmv1alpha1.EmailConfig); ok {
		event := &changeEvent{
			obj:       obj,
			name:      mc.Name,
			namespace: mc.Namespace,
			op:        opAdd,
			opType:    emailConfig,
			labels:    mc.Labels,
			isConfig:  true,
		}

		c.onChange(event)
	}
}

func (c *Config) onMailConfDel(obj interface{}) {
	if mc, ok := obj.(*nmv1alpha1.EmailConfig); ok {
		event := &changeEvent{
			obj:       obj,
			name:      mc.Name,
			namespace: mc.Namespace,
			op:        opDel,
			opType:    emailConfig,
			labels:    mc.Labels,
			isConfig:  true,
		}

		c.onChange(event)
	}
}

func (c *Config) onMailRcvAdd(obj interface{}) {
	if mr, ok := obj.(*nmv1alpha1.EmailReceiver); ok {
		event := &changeEvent{
			obj:       obj,
			name:      mr.Name,
			namespace: mr.Namespace,
			op:        opAdd,
			opType:    emailReceiver,
			labels:    mr.Labels,
			isConfig:  false,
		}

		c.onChange(event)
	}
}

func (c *Config) onMailRcvDel(obj interface{}) {
	if mr, ok := obj.(*nmv1alpha1.EmailReceiver); ok {
		event := &changeEvent{
			obj:       obj,
			name:      mr.Name,
			namespace: mr.Namespace,
			op:        opDel,
			opType:    emailReceiver,
			labels:    mr.Labels,
			isConfig:  false,
		}

		c.onChange(event)
	}
}

func (c *Config) onSlackConfAdd(obj interface{}) {
	if sc, ok := obj.(*nmv1alpha1.SlackConfig); ok {
		event := &changeEvent{
			obj:       obj,
			name:      sc.Name,
			namespace: sc.Namespace,
			op:        opAdd,
			opType:    slackConfig,
			labels:    sc.Labels,
			isConfig:  true,
		}

		c.onChange(event)
	}
}

func (c *Config) onSlackConfDel(obj interface{}) {
	if sc, ok := obj.(*nmv1alpha1.SlackConfig); ok {
		event := &changeEvent{
			obj:       obj,
			name:      sc.Name,
			namespace: sc.Namespace,
			op:        opDel,
			opType:    slackConfig,
			labels:    sc.Labels,
			isConfig:  true,
		}

		c.onChange(event)
	}
}

func (c *Config) onSlackRcvAdd(obj interface{}) {
	if sr, ok := obj.(*nmv1alpha1.SlackReceiver); ok {
		event := &changeEvent{
			obj:       obj,
			name:      sr.Name,
			namespace: sr.Namespace,
			op:        opAdd,
			opType:    slackReceiver,
			labels:    sr.Labels,
			isConfig:  false,
		}

		c.onChange(event)
	}
}

func (c *Config) onSlackRcvDel(obj interface{}) {
	if sr, ok := obj.(*nmv1alpha1.SlackReceiver); ok {
		event := &changeEvent{
			obj:       obj,
			name:      sr.Name,
			namespace: sr.Namespace,
			op:        opDel,
			opType:    slackReceiver,
			labels:    sr.Labels,
			isConfig:  false,
		}

		c.onChange(event)
	}
}

func (c *Config) onWechatConfAdd(obj interface{}) {
	if wc, ok := obj.(*nmv1alpha1.WechatConfig); ok {
		event := &changeEvent{
			obj:       obj,
			name:      wc.Name,
			namespace: wc.Namespace,
			op:        opAdd,
			opType:    wechatConfig,
			labels:    wc.Labels,
			isConfig:  true,
		}

		c.onChange(event)
	}
}

func (c *Config) onWechatConfDel(obj interface{}) {
	if wc, ok := obj.(*nmv1alpha1.WechatConfig); ok {
		event := &changeEvent{
			obj:       obj,
			name:      wc.Name,
			namespace: wc.Namespace,
			op:        opDel,
			opType:    wechatConfig,
			labels:    wc.Labels,
			isConfig:  true,
		}

		c.onChange(event)
	}
}

func (c *Config) onWechatRcvAdd(obj interface{}) {
	if wr, ok := obj.(*nmv1alpha1.WechatReceiver); ok {
		event := &changeEvent{
			obj:       obj,
			name:      wr.Name,
			namespace: wr.Namespace,
			op:        opAdd,
			opType:    wechatReceiver,
			labels:    wr.Labels,
			isConfig:  false,
		}

		c.onChange(event)
	}
}

func (c *Config) onWechatRcvDel(obj interface{}) {
	if wr, ok := obj.(*nmv1alpha1.WechatReceiver); ok {
		event := &changeEvent{
			obj:       obj,
			name:      wr.Name,
			namespace: wr.Namespace,
			op:        opDel,
			opType:    wechatReceiver,
			labels:    wr.Labels,
			isConfig:  false,
		}

		c.onChange(event)
	}
}

func (c *Config) onChange(event *changeEvent) {
	p := &param{}
	p.op = event.op
	p.opType = event.opType

	// If crd's label matches globalSelector such as "type = global",
	// then this is a global receiver or config, and tenantID should be set to an unique tenantID
	if c.globalReceiverSelector != nil {
		for k, expected := range c.globalReceiverSelector.MatchLabels {
			if v, exists := event.labels[k]; exists && v == expected {
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
			if v, exists := event.labels[k]; exists && v == expected {
				if v, exists := event.labels[c.tenantKey]; exists {
					p.tenantID = v
					if event.op == opAdd && event.isConfig {
						p.receiver = c.generateConfig(event)
					}
				}
				break
			}
		}
	}

	// If it is a config crd, update global default configs if crd's label match defaultConfigSelector
	if c.defaultConfigSelector != nil && event.isConfig {
		sel, _ := metav1.LabelSelectorAsSelector(c.defaultConfigSelector)
		if sel.Matches(labels.Set(event.labels)) {
			p.tenantID = defaultConfig
			if p.op == opAdd {
				p.defaultConfig = c.generateConfig(event)
			}
		}
	}

	p.namespace = event.namespace
	p.name = event.name

	if len(p.tenantID) == 0 {
		_ = level.Warn(c.logger).Log("msg", "Ignore empty tenantID", "op", p.op, "type", p.opType, "tenantKey", c.tenantKey, "name", event.name, "namespace", event.namespace)
		return
	}

	if event.op == opAdd && !event.isConfig {
		p.receiver = c.generateReceiver(event)
	}

	if p.receiver != nil {
		p.receiver.TenantID = &p.tenantID
	}

	p.done = make(chan interface{}, 1)
	c.ch <- p
	<-p.done
}

func (c *Config) generateConfig(event *changeEvent) *Receiver {

	switch event.opType {
	case emailConfig:
		ec, ok := event.obj.(*nmv1alpha1.EmailConfig)
		if !ok {
			return nil
		}
		return c.generateMailConfig(ec)
	case slackConfig:
		sc, ok := event.obj.(*nmv1alpha1.SlackConfig)
		if !ok {
			return nil
		}
		return c.generateSlackConfig(sc)
	case wechatConfig:
		wc, ok := event.obj.(*nmv1alpha1.WechatConfig)
		if !ok {
			return nil
		}
		return c.generateWechatConfig(wc)
	}

	return nil
}

func (c *Config) generateReceiver(event *changeEvent) *Receiver {

	switch event.opType {
	case emailReceiver:
		er, ok := event.obj.(*nmv1alpha1.EmailReceiver)
		if !ok {
			return nil
		}
		return c.generateMailReceiver(er)
	case slackReceiver:
		sr, ok := event.obj.(*nmv1alpha1.SlackReceiver)
		if !ok {
			return nil
		}
		return c.generateSlackReceiver(sr)
	case wechatReceiver:
		wr, ok := event.obj.(*nmv1alpha1.WechatReceiver)
		if !ok {
			return nil
		}
		return c.generateWechatReceiver(wr)
	}

	return nil
}

func (c *Config) generateEmail(mc *nmv1alpha1.EmailConfig) *Email {
	e := &Email{
		EmailConfig: &config.EmailConfig{
			From: mc.Spec.From,
		},
	}

	if mc.Spec.Hello != nil {
		e.EmailConfig.Hello = *mc.Spec.Hello
	}

	e.EmailConfig.Smarthost = config.HostPort(mc.Spec.SmartHost)
	if mc.Spec.AuthUsername != nil {
		e.EmailConfig.AuthUsername = *mc.Spec.AuthUsername
	}

	if mc.Spec.AuthIdentify != nil {
		e.EmailConfig.AuthIdentity = *mc.Spec.AuthIdentify
	}

	if mc.Spec.AuthPassword != nil {
		authPassword := v1.Secret{}
		if err := c.cache.Get(c.ctx, types.NamespacedName{Namespace: mc.Spec.AuthPassword.Namespace, Name: mc.Spec.AuthPassword.Name}, &authPassword); err != nil {
			_ = level.Error(c.logger).Log("msg", "Unable to get AuthPassword secret", "err", err)
			return nil
		}
		e.EmailConfig.AuthPassword = config.Secret(authPassword.Data[mc.Spec.AuthPassword.Key])
	}

	if mc.Spec.AuthSecret != nil {
		authSecret := v1.Secret{}
		if err := c.cache.Get(c.ctx, types.NamespacedName{Namespace: mc.Spec.AuthSecret.Namespace, Name: mc.Spec.AuthSecret.Name}, &authSecret); err != nil {
			_ = level.Error(c.logger).Log("msg", "Unable to get AuthSecret secret", "err", err)
			return nil
		}
		e.EmailConfig.AuthSecret = config.Secret(authSecret.Data[mc.Spec.AuthSecret.Key])
	}

	if mc.Spec.RequireTLS != nil {
		e.EmailConfig.RequireTLS = mc.Spec.RequireTLS
	}

	return e
}

func (c *Config) generateMailConfig(mc *nmv1alpha1.EmailConfig) *Receiver {
	rcv := &Receiver{}
	rcv.Email = c.generateEmail(mc)
	return rcv
}

func (c *Config) generateMailReceiver(mr *nmv1alpha1.EmailReceiver) *Receiver {
	mcList := nmv1alpha1.EmailConfigList{}
	mcSel, _ := metav1.LabelSelectorAsSelector(mr.Spec.EmailConfigSelector)
	if err := c.cache.List(c.ctx, &mcList, client.MatchingLabelsSelector{Selector: mcSel}); client.IgnoreNotFound(err) != nil {
		_ = level.Error(c.logger).Log("msg", "Unable to list EmailConfig", "err", err)
		return nil
	}

	rcv := &Receiver{
		Email: &Email{
			To:          mr.Spec.To,
			EmailConfig: nil,
		},
	}
	for _, mc := range mcList.Items {

		if !sliceIn(c.monitorNamespaces, mc.Namespace) {
			continue
		}

		e := c.generateEmail(&mc)
		if e != nil {
			rcv.Email.EmailConfig = e.EmailConfig
			break
		}
	}

	return rcv
}

func (c *Config) generateSlack(sc *nmv1alpha1.SlackConfig) *Slack {
	s := &Slack{
		SlackConfig: &SlackConfig{},
	}

	if sc.Spec.SlackTokenSecret == nil {
		return nil
	}

	secret := v1.Secret{}
	if err := c.cache.Get(c.ctx, types.NamespacedName{Namespace: sc.Namespace, Name: sc.Spec.SlackTokenSecret.Name}, &secret); err != nil {
		_ = level.Error(c.logger).Log("msg", "Unable to get slack token", "err", err)
		return nil
	}

	s.SlackConfig.Token = string(secret.Data[sc.Spec.SlackTokenSecret.Key])

	return s
}

func (c *Config) generateSlackConfig(sc *nmv1alpha1.SlackConfig) *Receiver {
	rcv := &Receiver{}
	rcv.Slack = c.generateSlack(sc)
	return rcv
}

func (c *Config) generateSlackReceiver(sr *nmv1alpha1.SlackReceiver) *Receiver {
	scList := nmv1alpha1.SlackConfigList{}
	scSel, _ := metav1.LabelSelectorAsSelector(sr.Spec.SlackConfigSelector)
	if err := c.cache.List(c.ctx, &scList, client.MatchingLabelsSelector{Selector: scSel}); client.IgnoreNotFound(err) != nil {
		_ = level.Error(c.logger).Log("msg", "Unable to list SlackConfig", "err", err)
		return nil
	}

	rcv := &Receiver{
		Slack: &Slack{
			Channel:     sr.Spec.Channel,
			SlackConfig: nil,
		},
	}
	for _, sc := range scList.Items {

		if !sliceIn(c.monitorNamespaces, sc.Namespace) {
			continue
		}

		s := c.generateSlack(&sc)
		if s != nil {
			rcv.Slack.SlackConfig = s.SlackConfig
			break
		}
	}

	return rcv
}

func (c *Config) generateWechat(wc *nmv1alpha1.WechatConfig) *Wechat {
	w := &Wechat{}
	w.WechatConfig = &config.WechatConfig{}

	if len(wc.Spec.WechatApiUrl) > 0 {
		u := &url.URL{}
		var err error
		u, err = u.Parse(wc.Spec.WechatApiUrl)
		if err != nil {
			_ = level.Error(c.logger).Log("msg", "Unable to parse Wechat apiurl", "url", wc.Spec.WechatApiUrl, "err", err)
			return nil
		}
		w.WechatConfig.APIURL = &config.URL{URL: u}
	}

	if wc.Spec.WechatApiSecret == nil {
		_ = level.Error(c.logger).Log("msg", "ignore wechat config because of empty api secret")
		return nil
	}

	secret := v1.Secret{}
	if err := c.cache.Get(c.ctx, types.NamespacedName{Namespace: wc.Namespace, Name: wc.Spec.WechatApiSecret.Name}, &secret); err != nil {
		_ = level.Error(c.logger).Log("msg", "Unable to get api secret", "err", err)
		return nil
	}
	w.WechatConfig.APISecret = config.Secret(secret.Data[wc.Spec.WechatApiSecret.Key])

	w.WechatConfig.AgentID = wc.Spec.WechatApiAgentId
	w.WechatConfig.CorpID = wc.Spec.WechatApiCorpId

	return w
}

func (c *Config) generateWechatConfig(wc *nmv1alpha1.WechatConfig) *Receiver {
	rcv := &Receiver{}
	rcv.Wechat = c.generateWechat(wc)
	return rcv
}

func (c *Config) generateWechatReceiver(wr *nmv1alpha1.WechatReceiver) *Receiver {
	wcList := nmv1alpha1.WechatConfigList{}
	wcSel, _ := metav1.LabelSelectorAsSelector(wr.Spec.WechatConfigSelector)
	if err := c.cache.List(c.ctx, &wcList, client.MatchingLabelsSelector{Selector: wcSel}); client.IgnoreNotFound(err) != nil {
		_ = level.Error(c.logger).Log("msg", "Unable to list WechatConfig", "err", err)
		return nil
	}

	rcv := &Receiver{
		Wechat: &Wechat{
			ToUser:       wr.Spec.ToUser,
			ToParty:      wr.Spec.ToParty,
			ToTag:        wr.Spec.ToTag,
			WechatConfig: nil,
		},
	}
	for _, wc := range wcList.Items {

		if !sliceIn(c.monitorNamespaces, wc.Namespace) {
			continue
		}

		w := c.generateWechat(&wc)
		if w != nil {
			rcv.Wechat.WechatConfig = w.WechatConfig
			break
		}
	}

	return rcv
}

func sliceIn(src []string, elem string) bool {
	for _, s := range src {
		if s == elem {
			return true
		}
	}

	return false
}
