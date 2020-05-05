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
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	kconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
	"sync"
	"time"
)

const (
	user                = "User"
	globalTenantID      = "notification-manager/type/global"
	globalDefaultConf   = "notification-manager/type/default"
	defaultTenantKey    = "user"
	notificationManager = "notification-manager"
	emailReceiver       = "email"
	emailConfig         = "email-config"
	wechatReceiver      = "wechat"
	wechatConfig        = "wechat-config"
	slackReceiver       = "slack"
	slackConfig         = "slack-config"
	webhookReceiver     = "webhook"
	webhookConfig       = "webhook-config"
	opAdd               = "add"
	opUpdate            = "update"
	opDel               = "delete"
	opGet               = "get"
	tenantKeyNamespace  = "namespace"
)

var (
	ConfigChannelCapacity = 1000
)

// GlobalConfig defines configuration parameters that are valid globally
// unless overwritten.
type GlobalConfig struct {
	config.GlobalConfig
	WeChatAgentID string
}

type Config struct {
	logger log.Logger
	ctx    context.Context
	cache  cache.Cache
	client client.Client
	// Global default config selector
	globalConfigSelector *metav1.LabelSelector
	// Global config for email, wechat, slack etc.
	globalEmailConfig   *GlobalConfig
	globalWechatConfig  *GlobalConfig
	globalSlackConfig   *GlobalConfig
	globalWebhookConfig *GlobalConfig
	// Label key used to distinguish different user
	tenantKey string
	// Label selector to filter valid global Receiver CR
	globalReceiverSelector *metav1.LabelSelector
	// Label selector to filter valid tenant Receiver CR
	tenantReceiverSelector *metav1.LabelSelector // Receiver config for each tenant user, in form of map[tenantID]map[type/namespace/name]*Receiver
	receivers              map[string]map[string]*Receiver
	ReceiverOpts           *nmv1alpha1.Options
	// Channel to receive receiver create/update/delete operations and then update receivers
	ch chan *param
}

type Email struct {
	To          []string
	EmailConfig *config.EmailConfig
}

type Wechat struct {
	ToUser       string
	ToParty      string
	ToTag        string
	WechatConfig *config.WechatConfig
}

type Slack struct {
	// The channel or user to send notifications to.
	Channel      string
	SlackConfigs *config.SlackConfig
}

type Webhook struct {
	WebhookConfig *config.WebhookConfig
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
	globalEmailConfig      *GlobalConfig
	globalWechatConfig     *GlobalConfig
	globalSlackConfig      *GlobalConfig
	globalWebhookConfig    *GlobalConfig
	tenantKey              string
	globalConfigSelector   *metav1.LabelSelector
	tenantReceiverSelector *metav1.LabelSelector
	globalReceiverSelector *metav1.LabelSelector
	receiver               *Receiver
	ReceiverOpts           *nmv1alpha1.Options
	done                   chan interface{}
}

func New(ctx context.Context, logger log.Logger) (*Config, error) {
	scheme := runtime.NewScheme()
	_ = nmv1alpha1.AddToScheme(scheme)
	_ = v1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)

	cfg, err := kconfig.GetConfig()
	if err != nil {
		_ = level.Error(logger).Log("msg", "Failed to get kubeconfig ", "err", err)
	}

	cache, err := cache.New(cfg, cache.Options{
		Scheme: scheme,
	})
	if err != nil {
		_ = level.Error(logger).Log("msg", "Failed to create cache", "err", err)
		return nil, err
	}

	client, err := newClient(cfg, cache, scheme)
	if err != nil {
		_ = level.Error(logger).Log("msg", "Failed to create client", "err", err)
		return nil, err
	}

	return &Config{
		ctx:                    ctx,
		logger:                 logger,
		cache:                  cache,
		client:                 client,
		globalEmailConfig:      nil,
		globalWechatConfig:     nil,
		globalSlackConfig:      nil,
		globalWebhookConfig:    nil,
		tenantKey:              defaultTenantKey,
		globalConfigSelector:   nil,
		tenantReceiverSelector: nil,
		globalReceiverSelector: nil,
		receivers:              make(map[string]map[string]*Receiver),
		ReceiverOpts:           nil,
		ch:                     make(chan *param, ConfigChannelCapacity),
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
	go c.cache.Start(c.ctx.Done())

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

	// Setup informer for EmailConfig
	wechatConfInf, err := c.cache.GetInformer(&nmv1alpha1.WechatConfig{})
	if err != nil {
		_ = level.Error(c.logger).Log("msg", "Failed to get informer for WechatConfig", "err", err)
		return err
	}
	wechatConfInf.AddEventHandler(kcache.ResourceEventHandlerFuncs{
		AddFunc: c.onWchatConfAdd,
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.onWchatConfAdd(newObj)
		},
		DeleteFunc: c.onWchatConfDel,
	})

	// Setup informer for EmailReceiver
	wechatRcvInf, err := c.cache.GetInformer(&nmv1alpha1.EmailReceiver{})
	if err != nil {
		_ = level.Error(c.logger).Log("msg", "Failed to get informer for EmailReceiver", "err", err)
		return err
	}
	wechatRcvInf.AddEventHandler(kcache.ResourceEventHandlerFuncs{
		AddFunc: c.onWchatRcvAdd,
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.onWchatRcvAdd(newObj)
		},
		DeleteFunc: c.onWchatRcvDel,
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
			c.globalConfigSelector = p.globalConfigSelector
			c.tenantReceiverSelector = p.tenantReceiverSelector
			c.globalReceiverSelector = p.globalReceiverSelector
			if p.ReceiverOpts != nil {
				c.ReceiverOpts = p.ReceiverOpts
			}
		case emailReceiver:
			// Setup EmailConfig with global default if emailconfig cannot be found
			if p.receiver.Email.EmailConfig == nil && c.globalEmailConfig != nil {
				p.receiver.Email.EmailConfig.Smarthost = c.globalEmailConfig.SMTPSmarthost
				p.receiver.Email.EmailConfig.AuthSecret = c.globalEmailConfig.SMTPAuthSecret
				p.receiver.Email.EmailConfig.AuthPassword = c.globalEmailConfig.SMTPAuthPassword
				p.receiver.Email.EmailConfig.AuthIdentity = c.globalEmailConfig.SMTPAuthIdentity
				p.receiver.Email.EmailConfig.AuthUsername = c.globalEmailConfig.SMTPAuthUsername
				p.receiver.Email.EmailConfig.Hello = c.globalEmailConfig.SMTPHello
				p.receiver.Email.EmailConfig.From = c.globalEmailConfig.SMTPFrom
			}
			rcvKey := fmt.Sprintf("%s/%s/%s", emailReceiver, p.namespace, p.name)
			if _, exist := c.receivers[p.tenantID]; exist {
				c.receivers[p.tenantID][rcvKey] = p.receiver
			} else if len(p.tenantID) > 0 {
				c.receivers[p.tenantID] = make(map[string]*Receiver)
				c.receivers[p.tenantID][rcvKey] = p.receiver
			}
		case emailConfig:
			// Setup global email config
			if p.globalEmailConfig != nil {
				c.globalEmailConfig = p.globalEmailConfig
				break
			}
			// Update EmailConfig of the recerver with the same tenantID
			if _, exist := c.receivers[p.tenantID]; exist {
				for k := range c.receivers[p.tenantID] {
					c.receivers[p.tenantID][k].Email.EmailConfig = p.receiver.Email.EmailConfig
				}
			}
		case wechatReceiver:
			// Setup EmailConfig with global default if emailconfig cannot be found
			if p.receiver.Wechat.WechatConfig == nil && c.globalWebhookConfig != nil {
				p.receiver.Wechat.WechatConfig.APISecret = c.globalWechatConfig.WeChatAPISecret
				p.receiver.Wechat.WechatConfig.CorpID = c.globalWechatConfig.WeChatAPICorpID
				p.receiver.Wechat.WechatConfig.APIURL = c.globalWechatConfig.WeChatAPIURL
				p.receiver.Wechat.WechatConfig.AgentID = c.globalWechatConfig.WeChatAgentID
			}
			rcvKey := fmt.Sprintf("%s/%s/%s", wechatReceiver, p.namespace, p.name)
			if _, exist := c.receivers[p.tenantID]; exist {
				c.receivers[p.tenantID][rcvKey] = p.receiver
			} else if len(p.tenantID) > 0 {
				c.receivers[p.tenantID] = make(map[string]*Receiver)
				c.receivers[p.tenantID][rcvKey] = p.receiver
			}
		case wechatConfig:
			// Setup global email config
			if p.globalWechatConfig != nil {
				c.globalWechatConfig = p.globalWechatConfig
				break
			}
			// Update globalWechatConfig of the recerver with the same tenantID
			if _, exist := c.receivers[p.tenantID]; exist {
				for k := range c.receivers[p.tenantID] {
					c.receivers[p.tenantID][k].Email.EmailConfig = p.receiver.Email.EmailConfig
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
			c.globalConfigSelector = nil
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
			// Reset global email config
			if p.globalEmailConfig != nil {
				c.globalEmailConfig = nil
				break
			}
			// Delete EmailConfig of the recerver with the same tenantID by setting the EmailConfig to nil
			if _, exist := c.receivers[p.tenantID]; exist {
				for k := range c.receivers[p.tenantID] {
					c.receivers[p.tenantID][k].Email.EmailConfig = nil
				}
			}
		default:
		}
	default:
	}
	p.done <- struct{}{}
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
		p.globalConfigSelector = nm.Spec.GlobalConfigSelector
		p.globalReceiverSelector = nm.Spec.Receivers.GlobalReceiverSelector
		p.tenantReceiverSelector = nm.Spec.Receivers.TenantReceiverSelector
		if nm.Spec.Receivers.Options != nil {
			p.ReceiverOpts = nm.Spec.Receivers.Options
		}
		p.done = make(chan interface{}, 1)
		c.ch <- p
		<-p.done

		// Update receiver and config CRs to trigger update of receivers
		wg := sync.WaitGroup{}
		wg.Add(2)
		go c.updateEmailReceivers(&wg)
		go c.updateEmailConfigs(&wg)
		wg.Wait()
	}
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

func (c *Config) generateMailReceiver(mr *nmv1alpha1.EmailReceiver) *Receiver {
	mcList := nmv1alpha1.EmailConfigList{}
	mcSel, _ := metav1.LabelSelectorAsSelector(mr.Spec.EmailConfigSelector)
	if err := c.cache.List(c.ctx, &mcList, client.MatchingLabelsSelector{Selector: mcSel}); client.IgnoreNotFound(err) != nil {
		_ = level.Error(c.logger).Log("msg", "Unable to list EmailConfig", "err", err)
		return nil
	}

	rcv := &Receiver{}
	rcv.Email = &Email{}
	rcv.Email.EmailConfig = &config.EmailConfig{}
	for _, mc := range mcList.Items {
		rcv.Email.EmailConfig.From = mc.Spec.From
		if mc.Spec.Hello != nil {
			rcv.Email.EmailConfig.Hello = *mc.Spec.Hello
		}
		rcv.Email.EmailConfig.Smarthost = config.HostPort(mc.Spec.SmartHost)
		if mc.Spec.AuthUsername != nil {
			rcv.Email.EmailConfig.AuthUsername = *mc.Spec.AuthUsername
		}

		if mc.Spec.AuthIdentify != nil {
			rcv.Email.EmailConfig.AuthIdentity = *mc.Spec.AuthIdentify
		}

		if mc.Spec.AuthPassword != nil {
			authPassword := v1.Secret{}
			if err := c.cache.Get(c.ctx, types.NamespacedName{Namespace: mc.Spec.AuthPassword.Namespace, Name: mc.Spec.AuthPassword.Name}, &authPassword); err != nil {
				_ = level.Error(c.logger).Log("msg", "Unable to get AuthPassword secret", "err", err)
				return nil
			}
			rcv.Email.EmailConfig.AuthPassword = config.Secret(string(authPassword.Data[mc.Spec.AuthPassword.Key]))
		}

		if mc.Spec.AuthSecret != nil {
			authSecret := v1.Secret{}
			if err := c.cache.Get(c.ctx, types.NamespacedName{Namespace: mc.Spec.AuthSecret.Namespace, Name: mc.Spec.AuthSecret.Name}, &authSecret); err != nil {
				_ = level.Error(c.logger).Log("msg", "Unable to get AuthSecret secret", "err", err)
				return nil
			}
			rcv.Email.EmailConfig.AuthSecret = config.Secret(string(authSecret.Data[mc.Spec.AuthSecret.Key]))
		}

		if mc.Spec.RequireTLS != nil {
			rcv.Email.EmailConfig.RequireTLS = mc.Spec.RequireTLS
		}

		break
	}

	// Set EmailConfig to nil to indicate EmailConfig should be setup with global default config
	if len(mcList.Items) == 0 {
		rcv.Email.EmailConfig = nil
	}

	rcv.Email.To = mr.Spec.To
	return rcv
}

func (c *Config) onMailRcvAdd(obj interface{}) {
	if mr, ok := obj.(*nmv1alpha1.EmailReceiver); ok {
		p := &param{}
		p.op = opAdd
		// If EmailReceiver's label matches globalReceiverSelector such as "type = global",
		// then this is a global EmailReceiver, and tenantID should be set to an unique tenantID
		if c.globalReceiverSelector != nil {
			for k, expected := range c.globalReceiverSelector.MatchLabels {
				if v, exists := mr.ObjectMeta.Labels[k]; exists && v == expected {
					p.tenantID = globalTenantID
					break
				}
			}
		}
		// If EmailReceiver's label matches tenantReceiverSelector such as "type = tenant",
		// then EmailReceiver's tenantKey's value should be used as tenantID,
		// For example, if tenantKey is "user" and label "user=admin" exists,
		// then "admin" should be used as tenantID
		if c.tenantReceiverSelector != nil {
			for k, expected := range c.tenantReceiverSelector.MatchLabels {
				if v, exists := mr.ObjectMeta.Labels[k]; exists && v == expected {
					if v, exists := mr.ObjectMeta.Labels[c.tenantKey]; exists {
						p.tenantID = v
					}
					break
				}
			}
		}

		p.name = mr.Name
		p.namespace = mr.Namespace
		p.opType = emailReceiver
		if len(p.tenantID) > 0 {
			p.receiver = c.generateMailReceiver(mr)
			p.receiver.TenantID = &p.tenantID
			p.done = make(chan interface{}, 1)
			c.ch <- p
			<-p.done
		} else {
			_ = level.Warn(c.logger).Log("msg", "Ignore empty tenantID", "tenantKey", c.tenantKey)
		}
	}
}

func (c *Config) onMailRcvDel(obj interface{}) {
	if mr, ok := obj.(*nmv1alpha1.EmailReceiver); ok {
		p := &param{}
		p.op = opDel
		// If EmailReceiver's label matches globalReceiverSelector such as "type = global",
		// then this is a global EmailReceiver, and tenantID should be set to an unique tenantID
		if c.globalReceiverSelector != nil {
			for k, expected := range c.globalReceiverSelector.MatchLabels {
				if v, exists := mr.ObjectMeta.Labels[k]; exists && v == expected {
					p.tenantID = globalTenantID
					break
				}
			}
		}
		// If EmailReceiver's label matches tenantReceiverSelector such as "type = tenant",
		// then EmailReceiver's tenantKey's value should be used as tenantID,
		// For example, if tenantKey is "user" and label "user=admin" exists,
		// then "admin" should be used as tenantID
		if c.tenantReceiverSelector != nil {
			for k, expected := range c.tenantReceiverSelector.MatchLabels {
				if v, exists := mr.ObjectMeta.Labels[k]; exists && v == expected {
					if v, exists := mr.ObjectMeta.Labels[c.tenantKey]; exists {
						p.tenantID = v
					}
					break
				}
			}
		}
		p.name = mr.Name
		p.namespace = mr.Namespace
		p.opType = emailReceiver
		if len(p.tenantID) > 0 {
			p.done = make(chan interface{}, 1)
			c.ch <- p
			<-p.done
		} else {
			_ = level.Warn(c.logger).Log("msg", "Ignore empty tenantID", "tenantKey", c.tenantKey)
		}
	}
}

func (c *Config) generateMailConfig(mc *nmv1alpha1.EmailConfig) *Receiver {
	rcv := &Receiver{}
	rcv.Email = &Email{}
	rcv.Email.EmailConfig = &config.EmailConfig{}
	rcv.Email.EmailConfig.From = mc.Spec.From

	if mc.Spec.Hello != nil {
		rcv.Email.EmailConfig.Hello = *mc.Spec.Hello
	}

	rcv.Email.EmailConfig.Smarthost = config.HostPort(mc.Spec.SmartHost)
	if mc.Spec.AuthUsername != nil {
		rcv.Email.EmailConfig.AuthUsername = *mc.Spec.AuthUsername
	}

	if mc.Spec.AuthIdentify != nil {
		rcv.Email.EmailConfig.AuthIdentity = *mc.Spec.AuthIdentify
	}

	if mc.Spec.AuthPassword != nil {
		authPassword := v1.Secret{}
		if err := c.cache.Get(c.ctx, types.NamespacedName{Namespace: mc.Spec.AuthPassword.Namespace, Name: mc.Spec.AuthPassword.Name}, &authPassword); err != nil {
			_ = level.Error(c.logger).Log("msg", "Unable to get AuthPassword secret", "err", err)
			return nil
		}
		rcv.Email.EmailConfig.AuthPassword = config.Secret(string(authPassword.Data[mc.Spec.AuthPassword.Key]))
	}

	if mc.Spec.AuthSecret != nil {
		authSecret := v1.Secret{}
		if err := c.cache.Get(c.ctx, types.NamespacedName{Namespace: mc.Spec.AuthSecret.Namespace, Name: mc.Spec.AuthSecret.Name}, &authSecret); err != nil {
			_ = level.Error(c.logger).Log("msg", "Unable to get AuthSecret secret", "err", err)
			return nil
		}
		rcv.Email.EmailConfig.AuthSecret = config.Secret(string(authSecret.Data[mc.Spec.AuthSecret.Key]))
	}

	if mc.Spec.RequireTLS != nil {
		rcv.Email.EmailConfig.RequireTLS = mc.Spec.RequireTLS
	}

	return rcv
}

func (c *Config) generateEmailGlobalConfig(mc *nmv1alpha1.EmailConfig) (*GlobalConfig, error) {
	global := &GlobalConfig{}
	global.SMTPFrom = mc.Spec.From
	if mc.Spec.Hello != nil {
		global.SMTPHello = *mc.Spec.Hello
	}

	global.SMTPSmarthost = config.HostPort(mc.Spec.SmartHost)

	if mc.Spec.AuthUsername != nil {
		global.SMTPAuthUsername = *mc.Spec.AuthUsername
	}

	if mc.Spec.AuthIdentify != nil {
		global.SMTPAuthIdentity = *mc.Spec.AuthIdentify
	}

	if mc.Spec.AuthPassword != nil {
		authPassword := v1.Secret{}
		if err := c.cache.Get(c.ctx, types.NamespacedName{Namespace: mc.Spec.AuthPassword.Namespace, Name: mc.Spec.AuthPassword.Name}, &authPassword); err != nil {
			_ = level.Warn(c.logger).Log("msg", "Unable to get AuthPassword secret", "err", err)
			return nil, client.IgnoreNotFound(err)
		}
		global.SMTPAuthPassword = config.Secret(string(authPassword.Data[mc.Spec.AuthPassword.Key]))
	}

	if mc.Spec.AuthSecret != nil {
		authSecret := v1.Secret{}
		if err := c.cache.Get(c.ctx, types.NamespacedName{Namespace: mc.Spec.AuthSecret.Namespace, Name: mc.Spec.AuthSecret.Name}, &authSecret); err != nil {
			_ = level.Warn(c.logger).Log("msg", "Unable to get AuthSecret secret", "err", err)
			return nil, client.IgnoreNotFound(err)
		}
		global.SMTPAuthSecret = config.Secret(string(authSecret.Data[mc.Spec.AuthSecret.Key]))
	}

	if mc.Spec.RequireTLS != nil {
		global.SMTPRequireTLS = *mc.Spec.RequireTLS
	}

	return global, nil
}

func (c *Config) onMailConfAdd(obj interface{}) {
	if mc, ok := obj.(*nmv1alpha1.EmailConfig); ok {
		p := &param{}
		p.op = opAdd
		p.opType = emailConfig

		// If EmailConfig's label matches globalReceiverSelector such as "type = global",
		// then this is a global EmailConfig, and tenantID should be set to an unique tenantID
		if c.globalReceiverSelector != nil {
			for k, expected := range c.globalReceiverSelector.MatchLabels {
				if v, exists := mc.ObjectMeta.Labels[k]; exists && v == expected {
					p.tenantID = globalTenantID
					p.receiver = c.generateMailConfig(mc)
					break
				}
			}
		}
		// If EmailConfig's label matches tenantReceiverSelector such as "type = tenant",
		// then EmailConfig's tenantKey's value should be used as tenantID,
		// For example, if tenantKey is "user" and label "user=admin" exists,
		// then "admin" should be used as tenantID
		if c.tenantReceiverSelector != nil {
			for k, expected := range c.tenantReceiverSelector.MatchLabels {
				if v, exists := mc.ObjectMeta.Labels[k]; exists && v == expected {
					if v, exists := mc.ObjectMeta.Labels[c.tenantKey]; exists {
						p.tenantID = v
						p.receiver = c.generateMailConfig(mc)
					}
					break
				}
			}
		}

		// Update global default configs if emailconfig's label match globalConfigSelector
		if c.globalConfigSelector != nil {
			sel, _ := metav1.LabelSelectorAsSelector(c.globalConfigSelector)
			if sel.Matches(labels.Set(mc.ObjectMeta.Labels)) {
				p.tenantID = globalDefaultConf
				p.globalEmailConfig, _ = c.generateEmailGlobalConfig(mc)
			}
		}

		if len(p.tenantID) > 0 {
			p.done = make(chan interface{}, 1)
			c.ch <- p
			<-p.done
		} else {
			_ = level.Warn(c.logger).Log("msg", "Ignore empty tenantID", "tenantKey", c.tenantKey)
		}
	}
}

func (c *Config) onMailConfDel(obj interface{}) {
	if mc, ok := obj.(*nmv1alpha1.EmailConfig); ok {
		p := &param{}
		p.op = opDel
		p.opType = emailConfig

		// If EmailConfig's label matches globalReceiverSelector such as "type = global",
		// then this is a global EmailConfig, and tenantID should be set to an unique tenantID
		if c.globalReceiverSelector != nil {
			for k, expected := range c.globalReceiverSelector.MatchLabels {
				if v, exists := mc.ObjectMeta.Labels[k]; exists && v == expected {
					p.tenantID = globalTenantID
					break
				}
			}
		}
		// If EmailConfig's label matches tenantReceiverSelector such as "type = tenant",
		// then EmailConfig's tenantKey's value should be used as tenantID,
		// For example, if tenantKey is "user" and label "user=admin" exists,
		// then "admin" should be used as tenantID
		if c.tenantReceiverSelector != nil {
			for k, expected := range c.tenantReceiverSelector.MatchLabels {
				if v, exists := mc.ObjectMeta.Labels[k]; exists && v == expected {
					if v, exists := mc.ObjectMeta.Labels[c.tenantKey]; exists {
						p.tenantID = v
					}
					break
				}
			}
		}

		// Update global default configs if emailconfig's label match globalConfigSelector
		if c.globalConfigSelector != nil {
			sel, _ := metav1.LabelSelectorAsSelector(c.globalConfigSelector)
			if sel.Matches(labels.Set(mc.ObjectMeta.Labels)) {
				p.tenantID = globalDefaultConf
				p.globalEmailConfig = &GlobalConfig{}
			}
		}

		if len(p.tenantID) > 0 {
			p.done = make(chan interface{}, 1)
			c.ch <- p
			<-p.done
		} else {
			_ = level.Warn(c.logger).Log("msg", "Ignore empty tenantID", "tenantKey", c.tenantKey)
		}
	}
}

func (c *Config) updateWchatReceivers(wg *sync.WaitGroup) {
	defer wg.Done()

	wrList := nmv1alpha1.WechatReceiverList{}
	if err := c.cache.List(c.ctx, &wrList, client.InNamespace("")); err != nil {
		_ = level.Error(c.logger).Log("msg", "Failed to list WechatReceiver", "err", err)
		return
	}
	for _, mr := range wrList.Items {
		r := mr.DeepCopy()
		r.ObjectMeta.Annotations["reloadtimestamp"] = time.Now().String()
		if err := c.client.Update(c.ctx, r); err != nil {
			_ = level.Error(c.logger).Log("msg", "Failed to update WechatReceiver", "err", err)
		}
	}

	return
}

func (c *Config) updateWchatConfigs(wg *sync.WaitGroup) {
	defer wg.Done()

	wcList := nmv1alpha1.WechatConfigList{}
	if err := c.cache.List(c.ctx, &wcList, client.InNamespace("")); err != nil {
		_ = level.Error(c.logger).Log("msg", "Failed to list WechatConfig", "err", err)
		return
	}
	for _, mc := range wcList.Items {
		cfg := mc.DeepCopy()
		cfg.ObjectMeta.Annotations["reloadtimestamp"] = time.Now().String()
		if err := c.client.Update(c.ctx, cfg); err != nil {
			_ = level.Error(c.logger).Log("msg", "Failed to update WechatConfig", "err", err)
		}
	}

	return
}

func (c *Config) generateWechatReceiver(wr *nmv1alpha1.WechatReceiver) *Receiver {
	wcList := nmv1alpha1.WechatConfigList{}
	wcSel, _ := metav1.LabelSelectorAsSelector(wr.Spec.WechatConfigSelector)
	if err := c.cache.List(c.ctx, &wcList, client.MatchingLabelsSelector{Selector: wcSel}); client.IgnoreNotFound(err) != nil {
		_ = level.Error(c.logger).Log("msg", "Unable to list WechatConfig", "err", err)
		return nil
	}

	rcv := &Receiver{}
	rcv.Wechat = &Wechat{}
	for _, wc := range wcList.Items {
		rcv.Wechat.WechatConfig = &config.WechatConfig{}

		if len(wc.Spec.WechatApiUrl) > 0 {
			url := &config.URL{}
			var err error
			url.URL, err = url.Parse(wc.Spec.WechatApiUrl)
			if err != nil {
				_ = level.Error(c.logger).Log("msg", "Unable to parse Wechat apiurl", "url", wc.Spec.WechatApiUrl, "err", err)
				continue
			}
			rcv.Wechat.WechatConfig.APIURL = url
		}

		if wc.Spec.WechatApiSecret == nil {
			_ = level.Error(c.logger).Log("msg", "ignore wechat config because of empty api secret")
			continue
		}

		secret := v1.Secret{}
		if err := c.cache.Get(c.ctx, types.NamespacedName{Namespace: wr.Namespace, Name: wc.Spec.WechatApiSecret.Name}, &secret); err != nil {
			_ = level.Error(c.logger).Log("msg", "Unable to get api secret", "err", err)
			continue
		}
		rcv.Wechat.WechatConfig.APISecret = config.Secret(string(secret.Data[wc.Spec.WechatApiSecret.Key]))

		rcv.Wechat.WechatConfig.AgentID = wc.Spec.WechatAgentId
		rcv.Wechat.WechatConfig.CorpID = wc.Spec.WechatApiCorpId

		break
	}

	// Set EmailConfig to nil to indicate EmailConfig should be setup with global default config
	if len(rcv.Wechat.WechatConfig.CorpID) == 0 {
		rcv.Wechat.WechatConfig = nil
	}

	rcv.Wechat.ToUser = wr.Spec.ToUser
	rcv.Wechat.ToParty = wr.Spec.ToParty
	rcv.Wechat.ToTag = wr.Spec.ToTag

	return rcv
}

func (c *Config) onWchatRcvAdd(obj interface{}) {
	if wr, ok := obj.(*nmv1alpha1.WechatReceiver); ok {
		p := &param{}
		p.op = opAdd
		// If WechatReceiver's label matches globalReceiverSelector such as "type = global",
		// then this is a global EmailReceiver, and tenantID should be set to an unique tenantID
		if c.globalReceiverSelector != nil {
			for k, expected := range c.globalReceiverSelector.MatchLabels {
				if v, exists := wr.ObjectMeta.Labels[k]; exists && v == expected {
					p.tenantID = globalTenantID
					break
				}
			}
		}
		// If WechatReceiver's label matches tenantReceiverSelector such as "type = tenant",
		// then WechatReceiver's tenantKey's value should be used as tenantID,
		// For example, if tenantKey is "user" and label "user=admin" exists,
		// then "admin" should be used as tenantID
		if c.tenantReceiverSelector != nil {
			for k, expected := range c.tenantReceiverSelector.MatchLabels {
				if v, exists := wr.ObjectMeta.Labels[k]; exists && v == expected {
					if v, exists := wr.ObjectMeta.Labels[c.tenantKey]; exists {
						p.tenantID = v
					}
					break
				}
			}
		}

		p.name = wr.Name
		p.namespace = wr.Namespace
		p.opType = wechatReceiver
		if len(p.tenantID) > 0 {
			p.receiver = c.generateWechatReceiver(wr)
			p.done = make(chan interface{}, 1)
			c.ch <- p
			<-p.done
		} else {
			_ = level.Warn(c.logger).Log("msg", "Ignore empty tenantID", "tenantKey", c.tenantKey)
		}
	}
}

func (c *Config) onWchatRcvDel(obj interface{}) {
	if wr, ok := obj.(*nmv1alpha1.WechatReceiver); ok {
		p := &param{}
		p.op = opDel
		// If WechatReceiver's label matches globalReceiverSelector such as "type = global",
		// then this is a global WechatReceiver, and tenantID should be set to an unique tenantID
		if c.globalReceiverSelector != nil {
			for k, expected := range c.globalReceiverSelector.MatchLabels {
				if v, exists := wr.ObjectMeta.Labels[k]; exists && v == expected {
					p.tenantID = globalTenantID
					break
				}
			}
		}
		// If WechatReceiver's label matches tenantReceiverSelector such as "type = tenant",
		// then WechatReceiver's tenantKey's value should be used as tenantID,
		// For example, if tenantKey is "user" and label "user=admin" exists,
		// then "admin" should be used as tenantID
		if c.tenantReceiverSelector != nil {
			for k, expected := range c.tenantReceiverSelector.MatchLabels {
				if v, exists := wr.ObjectMeta.Labels[k]; exists && v == expected {
					if v, exists := wr.ObjectMeta.Labels[c.tenantKey]; exists {
						p.tenantID = v
					}
					break
				}
			}
		}
		p.name = wr.Name
		p.namespace = wr.Namespace
		p.opType = wechatReceiver
		if len(p.tenantID) > 0 {
			p.done = make(chan interface{}, 1)
			c.ch <- p
			<-p.done
		} else {
			_ = level.Warn(c.logger).Log("msg", "Ignore empty tenantID", "tenantKey", c.tenantKey)
		}
	}
}

func (c *Config) generateWechatConfig(wc *nmv1alpha1.WechatConfig) *Receiver {
	rcv := &Receiver{}
	rcv.Wechat = &Wechat{}
	rcv.Wechat.WechatConfig = &config.WechatConfig{}

	if len(wc.Spec.WechatApiUrl) > 0 {
		url := &config.URL{}
		var err error
		url.URL, err = url.Parse(wc.Spec.WechatApiUrl)
		if err != nil {
			_ = level.Error(c.logger).Log("msg", "Unable to parse Wechat apiurl", "url", wc.Spec.WechatApiUrl, "err", err)
			return nil
		}
		rcv.Wechat.WechatConfig.APIURL = url
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
	rcv.Wechat.WechatConfig.APISecret = config.Secret(string(secret.Data[wc.Spec.WechatApiSecret.Key]))

	rcv.Wechat.WechatConfig.AgentID = wc.Spec.WechatAgentId
	rcv.Wechat.WechatConfig.CorpID = wc.Spec.WechatApiCorpId

	return rcv
}

func (c *Config) generateWechatGlobalConfig(wc *nmv1alpha1.WechatConfig) (*GlobalConfig, error) {
	global := &GlobalConfig{}
	if len(wc.Spec.WechatApiUrl) > 0 {
		url := &config.URL{}
		var err error
		url.URL, err = url.Parse(wc.Spec.WechatApiUrl)
		if err != nil {
			_ = level.Error(c.logger).Log("msg", "Unable to parse Wechat apiurl", "url", wc.Spec.WechatApiUrl, "err", err)
			return nil, err
		}
		global.WeChatAPIURL = url
	}

	if wc.Spec.WechatApiSecret == nil {
		_ = level.Error(c.logger).Log("msg", "ignore wechat config because of empty api secret")
		return nil, fmt.Errorf("ignore wechat config because of empty api secret")
	}

	secret := v1.Secret{}
	if err := c.cache.Get(c.ctx, types.NamespacedName{Namespace: wc.Namespace, Name: wc.Spec.WechatApiSecret.Name}, &secret); err != nil {
		_ = level.Error(c.logger).Log("msg", "Unable to get api secret", "err", err)
		return nil, err
	}
	global.WeChatAPISecret = config.Secret(string(secret.Data[wc.Spec.WechatApiSecret.Key]))

	global.WeChatAPICorpID = wc.Spec.WechatApiCorpId
	global.WeChatAgentID = wc.Spec.WechatAgentId
	return global, nil
}

func (c *Config) onWchatConfAdd(obj interface{}) {
	if wc, ok := obj.(*nmv1alpha1.WechatConfig); ok {
		p := &param{}
		p.op = opAdd
		p.opType = wechatConfig

		// If WechatConfig's label matches globalReceiverSelector such as "type = global",
		// then this is a global EmailConfig, and tenantID should be set to an unique tenantID
		if c.globalReceiverSelector != nil {
			for k, expected := range c.globalReceiverSelector.MatchLabels {
				if v, exists := wc.ObjectMeta.Labels[k]; exists && v == expected {
					p.tenantID = globalTenantID
					p.receiver = c.generateWechatConfig(wc)
					break
				}
			}
		}
		// If WechatConfig's label matches tenantReceiverSelector such as "type = tenant",
		// then WechatConfig's tenantKey's value should be used as tenantID,
		// For example, if tenantKey is "user" and label "user=admin" exists,
		// then "admin" should be used as tenantID
		if c.tenantReceiverSelector != nil {
			for k, expected := range c.tenantReceiverSelector.MatchLabels {
				if v, exists := wc.ObjectMeta.Labels[k]; exists && v == expected {
					if v, exists := wc.ObjectMeta.Labels[c.tenantKey]; exists {
						p.tenantID = v
						p.receiver = c.generateWechatConfig(wc)
					}
					break
				}
			}
		}

		// Update global default configs if emailconfig's label match globalConfigSelector
		if c.globalConfigSelector != nil {
			sel, _ := metav1.LabelSelectorAsSelector(c.globalConfigSelector)
			if sel.Matches(labels.Set(wc.ObjectMeta.Labels)) {
				p.tenantID = globalDefaultConf
				p.globalEmailConfig, _ = c.generateWechatGlobalConfig(wc)
			}
		}

		if len(p.tenantID) > 0 {
			p.done = make(chan interface{}, 1)
			c.ch <- p
			<-p.done
		} else {
			_ = level.Warn(c.logger).Log("msg", "Ignore empty tenantID", "tenantKey", c.tenantKey)
		}
	}
}

func (c *Config) onWchatConfDel(obj interface{}) {
	if wc, ok := obj.(*nmv1alpha1.WechatConfig); ok {
		p := &param{}
		p.op = opDel
		p.opType = wechatConfig

		// If WechatConfig's label matches globalReceiverSelector such as "type = global",
		// then this is a global EmailConfig, and tenantID should be set to an unique tenantID
		if c.globalReceiverSelector != nil {
			for k, expected := range c.globalReceiverSelector.MatchLabels {
				if v, exists := wc.ObjectMeta.Labels[k]; exists && v == expected {
					p.tenantID = globalTenantID
					break
				}
			}
		}
		// If WechatConfig's label matches tenantReceiverSelector such as "type = tenant",
		// then WechatConfig's tenantKey's value should be used as tenantID,
		// For example, if tenantKey is "user" and label "user=admin" exists,
		// then "admin" should be used as tenantID
		if c.tenantReceiverSelector != nil {
			for k, expected := range c.tenantReceiverSelector.MatchLabels {
				if v, exists := wc.ObjectMeta.Labels[k]; exists && v == expected {
					if v, exists := wc.ObjectMeta.Labels[c.tenantKey]; exists {
						p.tenantID = v
					}
					break
				}
			}
		}

		// Update global default configs if emailconfig's label match globalConfigSelector
		if c.globalConfigSelector != nil {
			sel, _ := metav1.LabelSelectorAsSelector(c.globalConfigSelector)
			if sel.Matches(labels.Set(wc.ObjectMeta.Labels)) {
				p.tenantID = globalDefaultConf
				p.globalEmailConfig = &GlobalConfig{}
			}
		}

		if len(p.tenantID) > 0 {
			p.done = make(chan interface{}, 1)
			c.ch <- p
			<-p.done
		} else {
			_ = level.Warn(c.logger).Log("msg", "Ignore empty tenantID", "tenantKey", c.tenantKey)
		}
	}
}
