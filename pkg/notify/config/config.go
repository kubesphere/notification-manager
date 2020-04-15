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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	kcache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	user                = "User"
	scope               = "scope"
	global              = "global"
	globalTenantID      = "notification-manager/tenant/global"
	tenant              = "tenant"
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
)

var (
	ConfigChannelCapacity = 1000
)

type Config struct {
	logger log.Logger
	ctx    context.Context
	cache  cache.Cache
	// Global config for email, wechat, slack etc.
	GlobalEmailConfig   *config.GlobalConfig
	GlobalWechatConfig  *config.GlobalConfig
	GlobalSlackConfig   *config.GlobalConfig
	GlobalWebhookConfig *config.GlobalConfig
	// Label key used to distinguish different user
	TenantKey string
	// Label selector to filter valid global Receiver CR
	GlobalReceiverSelector *metav1.LabelSelector
	// Label selector to filter valid tenant Receiver CR
	TenantReceiverSelector *metav1.LabelSelector // Receiver config for each tenant user, in form of map[TenantID]map[Type/Namespace/Name]*Receiver
	Receivers              map[string]map[string]*Receiver
	// Channel to receive receiver create/update/delete operations and then update Receivers
	ch chan *param
}

type Email struct {
	To          []string
	EmailConfig *config.EmailConfig
}

type Wechat struct {
	Message      string
	AgentId      string
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
	Email   *Email
	Wechat  *Wechat
	Slack   *Slack
	Webhook *Webhook
}

type param struct {
	Op                     string
	TenantID               string
	Type                   string
	Namespace              string
	Name                   string
	GlobalEmailConfig      *config.GlobalConfig
	GlobalWechatConfig     *config.GlobalConfig
	GlobalSlackConfig      *config.GlobalConfig
	GlobalWebhookConfig    *config.GlobalConfig
	TenantKey              string
	TenantReceiverSelector *metav1.LabelSelector
	GlobalReceiverSelector *metav1.LabelSelector
	Receiver               *Receiver
	done                   chan interface{}
}

func New(ctx context.Context, logger log.Logger) (*Config, error) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = nmv1alpha1.AddToScheme(scheme)

	cfg, err := kconfig.GetConfig()
	if err != nil {
		_ = level.Error(logger).Log("msg", "Failed to get kubeconfig ", "err", err)
	}

	c, err := cache.New(cfg, cache.Options{
		Scheme: scheme,
	})
	if err != nil {
		_ = level.Error(logger).Log("msg", "Failed to create cache", "err", err)
		return nil, err
	}

	return &Config{
		ctx:                    ctx,
		logger:                 logger,
		cache:                  c,
		GlobalEmailConfig:      nil,
		GlobalWechatConfig:     nil,
		GlobalSlackConfig:      nil,
		GlobalWebhookConfig:    nil,
		TenantKey:              "namespace",
		TenantReceiverSelector: nil,
		GlobalReceiverSelector: nil,
		Receivers:              make(map[string]map[string]*Receiver),
		ch:                     make(chan *param, ConfigChannelCapacity),
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
		AddFunc: c.OnNmAdd,
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.OnNmAdd(newObj)
		},
		DeleteFunc: c.OnNmDel,
	})

	// Setup informer for EmailConfig
	mailConfInf, err := c.cache.GetInformer(&nmv1alpha1.EmailConfig{})
	if err != nil {
		_ = level.Error(c.logger).Log("msg", "Failed to get informer for EmailConfig", "err", err)
		return err
	}
	mailConfInf.AddEventHandler(kcache.ResourceEventHandlerFuncs{
		AddFunc: c.OnMailConfAdd,
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.OnMailConfAdd(newObj)
		},
		DeleteFunc: c.OnMailConfDel,
	})

	// Setup informer for EmailReceiver
	mailRcvInf, err := c.cache.GetInformer(&nmv1alpha1.EmailReceiver{})
	if err != nil {
		_ = level.Error(c.logger).Log("msg", "Failed to get informer for EmailReceiver", "err", err)
		return err
	}
	mailRcvInf.AddEventHandler(kcache.ResourceEventHandlerFuncs{
		AddFunc: c.OnMailRcvAdd,
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.OnMailRcvAdd(newObj)
		},
		DeleteFunc: c.OnMailRcvDel,
	})

	if ok := c.cache.WaitForCacheSync(c.ctx.Done()); !ok {
		return fmt.Errorf("NotificationManager cache failed")
	}
	_ = level.Info(c.logger).Log("msg", "Setting up informers successfully")
	return c.ctx.Err()
}

func (c *Config) sync(p *param) {
	switch p.Op {
	case opGet:
		// Return all receivers of the specified tenant (map[Type/Namespace/Name]*Receiver)
		// via the done channel if exists
		if v, exist := c.Receivers[p.TenantID]; exist {
			p.done <- v
			// Return empty struct if receivers of the specified tenant cannot be found
		} else {
			p.done <- struct{}{}
		}
		return
	case opAdd:
		switch p.Type {
		case notificationManager:
			c.TenantKey = p.TenantKey
			c.TenantReceiverSelector = p.TenantReceiverSelector
			c.GlobalReceiverSelector = p.GlobalReceiverSelector
		case emailReceiver:
			rcvKey := fmt.Sprintf("%s/%s/%s", emailReceiver, p.Namespace, p.Name)
			if _, exist := c.Receivers[p.TenantID]; exist {
				c.Receivers[p.TenantID][rcvKey] = p.Receiver
			} else if len(p.TenantID) > 0 {
				c.Receivers[p.TenantID] = make(map[string]*Receiver)
				c.Receivers[p.TenantID][rcvKey] = p.Receiver
			}
		case emailConfig:
			// Setup global email config
			if p.GlobalEmailConfig != nil {
				c.GlobalEmailConfig = p.GlobalEmailConfig
			}
			// Update EmailConfig of the recerver with the same TenantID
			if _, exist := c.Receivers[p.TenantID]; exist {
				for k := range c.Receivers[p.TenantID] {
					c.Receivers[p.TenantID][k].Email.EmailConfig = p.Receiver.Email.EmailConfig
				}
			}
		default:
		}
	case opDel:
		switch p.Type {
		case notificationManager:
			c.TenantKey = "namespace"
			c.GlobalReceiverSelector = nil
			c.TenantReceiverSelector = nil
		case emailReceiver:
			rcvKey := fmt.Sprintf("%s/%s/%s", emailReceiver, p.Namespace, p.Name)
			if _, exist := c.Receivers[p.TenantID]; exist {
				delete(c.Receivers[p.TenantID], rcvKey)
				if len(c.Receivers[p.TenantID]) <= 0 {
					delete(c.Receivers, p.TenantID)
				}
			}
		case emailConfig:
			// Reset global email config
			if p.GlobalEmailConfig != nil {
				c.GlobalEmailConfig = nil
			}
			// Delete EmailConfig of the recerver with the same TenantID by setting the EmailConfig to nil
			if _, exist := c.Receivers[p.TenantID]; exist {
				for k := range c.Receivers[p.TenantID] {
					c.Receivers[p.TenantID][k].Email.EmailConfig = nil
				}
			}
		default:
		}
	default:
	}
	p.done <- struct{}{}
}

func (c *Config) TenantIDFromNs(namespace string) ([]string, error) {
	tenantIDs := make([]string, 0)
	rbList := rbacv1.RoleBindingList{}
	if err := c.cache.List(c.ctx, &rbList, client.InNamespace(namespace)); err != nil {
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
	// Return global receiver if namespace is nil
	if namespace == nil {
		p := param{}
		p.Op = opGet
		p.TenantID = globalTenantID
		p.done = make(chan interface{}, 1)
		c.ch <- &p
		o := <-p.done
		if r, ok := o.(map[string]*Receiver); ok {
			for _, v := range r {
				rcvs = append(rcvs, v)
			}
		}

	} else {
		// Return receivers for each tenant if namespace is not nil
		if tenantIDs, err := c.TenantIDFromNs(*namespace); err != nil {
			_ = level.Error(c.logger).Log("msg", "Unable to find tenantID", "err", err)
		} else {
			for _, t := range tenantIDs {
				p := param{}
				p.Op = opGet
				p.TenantID = t
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

func (c *Config) OnNmAdd(obj interface{}) {
	if nm, ok := obj.(*nmv1alpha1.NotificationManager); ok {
		p := &param{}
		p.Op = opAdd
		p.Name = nm.Name
		p.Namespace = nm.Namespace
		p.Type = notificationManager
		p.TenantKey = nm.Spec.Receivers.TenantKey
		p.GlobalReceiverSelector = nm.Spec.Receivers.GlobalReceiverSelector
		p.TenantReceiverSelector = nm.Spec.Receivers.TenantReceiverSelector
		p.done = make(chan interface{}, 1)
		c.ch <- p
		<-p.done
	}
}

func (c *Config) OnNmDel(obj interface{}) {
	if _, ok := obj.(*nmv1alpha1.NotificationManager); ok {
		p := &param{}
		p.Op = opDel
		p.Type = notificationManager
		p.done = make(chan interface{}, 1)
		c.ch <- p
		<-p.done
	}
}

func (c *Config) generateMailReceiver(mr *nmv1alpha1.EmailReceiver) *Receiver {
	mcList := nmv1alpha1.EmailConfigList{}
	mcSel, _ := metav1.LabelSelectorAsSelector(mr.Spec.EmailConfigSelector)
	if err := c.cache.List(c.ctx, &mcList, client.MatchingLabelsSelector{Selector: mcSel}); err != nil {
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
	rcv.Email.To = mr.Spec.To

	return rcv
}

func (c *Config) OnMailRcvAdd(obj interface{}) {
	if mr, ok := obj.(*nmv1alpha1.EmailReceiver); ok {
		p := &param{}
		p.Op = opAdd
		// If EmailReceiver's label matches GlobalReceiverSelector such as "scope = global",
		// then this is a global EmailReceiver, and TenantID should be set to an unique TenantID
		if c.GlobalReceiverSelector != nil {
			for k, expected := range c.GlobalReceiverSelector.MatchLabels {
				if v, exists := mr.ObjectMeta.Labels[k]; exists && v == expected {
					p.TenantID = globalTenantID
					break
				}
			}
		}
		// If EmailReceiver's label matches TenantReceiverSelector such as "scope = tenant",
		// then EmailReceiver's TenantKey's value should be used as TenantID,
		// For example, if TenantKey is "user" and label "user=admin" exists,
		// then "admin" should be used as TenantID
		if c.TenantReceiverSelector != nil {
			for k, expected := range c.TenantReceiverSelector.MatchLabels {
				if v, exists := mr.ObjectMeta.Labels[k]; exists && v == expected {
					if v, exists := mr.ObjectMeta.Labels[c.TenantKey]; exists {
						p.TenantID = v
					}
					break
				}
			}
		}

		p.Name = mr.Name
		p.Namespace = mr.Namespace
		p.Type = emailReceiver
		if len(p.TenantID) > 0 {
			p.Receiver = c.generateMailReceiver(mr)
			p.done = make(chan interface{}, 1)
			c.ch <- p
			<-p.done
		} else {
			_ = level.Warn(c.logger).Log("msg", "Ignore empty TenantID", "TenantKey", c.TenantKey)
		}
	}
}

func (c *Config) OnMailRcvDel(obj interface{}) {
	if mr, ok := obj.(*nmv1alpha1.EmailReceiver); ok {
		p := &param{}
		p.Op = opDel
		// If EmailReceiver's label matches GlobalReceiverSelector such as "scope = global",
		// then this is a global EmailReceiver, and TenantID should be set to an unique TenantID
		if c.GlobalReceiverSelector != nil {
			for k, expected := range c.GlobalReceiverSelector.MatchLabels {
				if v, exists := mr.ObjectMeta.Labels[k]; exists && v == expected {
					p.TenantID = globalTenantID
					break
				}
			}
		}
		// If EmailReceiver's label matches TenantReceiverSelector such as "scope = tenant",
		// then EmailReceiver's TenantKey's value should be used as TenantID,
		// For example, if TenantKey is "user" and label "user=admin" exists,
		// then "admin" should be used as TenantID
		if c.TenantReceiverSelector != nil {
			for k, expected := range c.TenantReceiverSelector.MatchLabels {
				if v, exists := mr.ObjectMeta.Labels[k]; exists && v == expected {
					if v, exists := mr.ObjectMeta.Labels[c.TenantKey]; exists {
						p.TenantID = v
					}
					break
				}
			}
		}
		p.Name = mr.Name
		p.Namespace = mr.Namespace
		p.Type = emailReceiver
		if len(p.TenantID) > 0 {
			p.done = make(chan interface{}, 1)
			c.ch <- p
			<-p.done
		} else {
			_ = level.Warn(c.logger).Log("msg", "Ignore empty TenantID", "TenantKey", c.TenantKey)
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

func (c *Config) generateEmailGlobalConfig(mc *nmv1alpha1.EmailConfig) (*config.GlobalConfig, error) {
	global := &config.GlobalConfig{}
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

func (c *Config) OnMailConfAdd(obj interface{}) {
	if mc, ok := obj.(*nmv1alpha1.EmailConfig); ok {
		p := &param{}
		p.Op = opAdd
		p.Type = emailConfig

		// If EmailConfig's label matches GlobalReceiverSelector such as "scope = global",
		// then this is a global EmailConfig, and TenantID should be set to an unique TenantID
		if c.GlobalReceiverSelector != nil {
			for k, expected := range c.GlobalReceiverSelector.MatchLabels {
				if v, exists := mc.ObjectMeta.Labels[k]; exists && v == expected {
					p.GlobalEmailConfig, _ = c.generateEmailGlobalConfig(mc)
					p.TenantID = globalTenantID
					p.Receiver = c.generateMailConfig(mc)
					break
				}
			}
		}
		// If EmailConfig's label matches TenantReceiverSelector such as "scope = tenant",
		// then EmailConfig's TenantKey's value should be used as TenantID,
		// For example, if TenantKey is "user" and label "user=admin" exists,
		// then "admin" should be used as TenantID
		if c.TenantReceiverSelector != nil {
			for k, expected := range c.TenantReceiverSelector.MatchLabels {
				if v, exists := mc.ObjectMeta.Labels[k]; exists && v == expected {
					if v, exists := mc.ObjectMeta.Labels[c.TenantKey]; exists {
						p.TenantID = v
						p.Receiver = c.generateMailConfig(mc)
					}
					break
				}
			}
		}

		if len(p.TenantID) > 0 {
			p.done = make(chan interface{}, 1)
			c.ch <- p
			<-p.done
		} else {
			_ = level.Warn(c.logger).Log("msg", "Ignore empty TenantID", "TenantKey", c.TenantKey)
		}
	}
}

func (c *Config) OnMailConfDel(obj interface{}) {
	if mc, ok := obj.(*nmv1alpha1.EmailConfig); ok {
		p := &param{}
		p.Op = opDel
		p.Type = emailConfig

		// If EmailConfig's label matches GlobalReceiverSelector such as "scope = global",
		// then this is a global EmailConfig, and TenantID should be set to an unique TenantID
		if c.GlobalReceiverSelector != nil {
			for k, expected := range c.GlobalReceiverSelector.MatchLabels {
				if v, exists := mc.ObjectMeta.Labels[k]; exists && v == expected {
					p.GlobalEmailConfig = &config.GlobalConfig{}
					p.TenantID = globalTenantID
					break
				}
			}
		}
		// If EmailConfig's label matches TenantReceiverSelector such as "scope = tenant",
		// then EmailConfig's TenantKey's value should be used as TenantID,
		// For example, if TenantKey is "user" and label "user=admin" exists,
		// then "admin" should be used as TenantID
		if c.TenantReceiverSelector != nil {
			for k, expected := range c.TenantReceiverSelector.MatchLabels {
				if v, exists := mc.ObjectMeta.Labels[k]; exists && v == expected {
					if v, exists := mc.ObjectMeta.Labels[c.TenantKey]; exists {
						p.TenantID = v
					}
					break
				}
			}
		}

		if len(p.TenantID) > 0 {
			p.done = make(chan interface{}, 1)
			c.ch <- p
			<-p.done
		} else {
			_ = level.Warn(c.logger).Log("msg", "Ignore empty TenantID", "TenantKey", c.TenantKey)
		}
	}
}
