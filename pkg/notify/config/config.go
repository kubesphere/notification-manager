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
	"strings"
)

const (
	user              = "User"
	scope             = "scope"
	global            = "global"
	tenant            = "tenant"
	globalEmailConfig = "global-email-config"
	emailReceiver     = "email"
	emailConfig       = "email-config"
	wechatReceiver    = "wechat"
	wechatConfig      = "wechat-config"
	slackReceiver     = "slack"
	slackConfig       = "slack-config"
	webhookReceiver   = "webhook"
	webhookConfig     = "webhook-config"
	opAdd             = "add"
	opUpdate          = "update"
	opDel             = "delete"
	opGet             = "get"
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
	// Label selector to filter valid Receiver CR
	ReceiverSelector *metav1.LabelSelector
	// Receiver config for each tenant user, in form of map[TenantID]map[Type/Namespace/Name]*Receiver
	Receivers map[string]map[string]*Receiver
	// Channel to receive receiver create/update/delete operations and then update Receivers
	ch chan *param
}

type Email struct {
	To          string
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
	Op                string
	TenantID          string
	Type              string
	Namespace         string
	Name              string
	GlobalEmailConfig *config.GlobalConfig
	TenantKey         string
	ReceiverSelector  *metav1.LabelSelector
	Receiver          *Receiver
	done              chan interface{}
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
		ctx:                 ctx,
		logger:              logger,
		cache:               c,
		GlobalEmailConfig:   nil,
		GlobalWechatConfig:  nil,
		GlobalSlackConfig:   nil,
		GlobalWebhookConfig: nil,
		TenantKey:           "namespace",
		ReceiverSelector:    nil,
		Receivers:           make(map[string]map[string]*Receiver),
		ch:                  make(chan *param, ConfigChannelCapacity),
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
		// Return all receivers of the specified tenant (*map[Type/Namespace/Name]*Receiver)
		// via the done channel if exists
		if v, exist := c.Receivers[p.TenantID]; exist {
			p.done <- &v
			// Return empty struct if receivers of the specified tenant cannot be found
		} else {
			p.done <- struct{}{}
		}
		return
	case opAdd:
		switch p.Type {
		case globalEmailConfig:
			c.GlobalEmailConfig = p.GlobalEmailConfig
			c.TenantKey = p.TenantKey
			c.ReceiverSelector = p.ReceiverSelector
		case emailReceiver:
			rcvKey := fmt.Sprintf("%s/%s/%s", emailReceiver, p.Namespace, p.Name)
			if _, exist := c.Receivers[p.TenantID]; exist {
				c.Receivers[p.TenantID][rcvKey] = p.Receiver
			} else if len(p.TenantID) > 0 {
				c.Receivers[p.TenantID] = make(map[string]*Receiver)
				c.Receivers[p.TenantID][rcvKey] = p.Receiver
			}
		case emailConfig:
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
		case globalEmailConfig:
			c.GlobalEmailConfig = nil
			c.TenantKey = "namespace"
			c.ReceiverSelector = nil
		case emailReceiver:
			rcvKey := fmt.Sprintf("%s/%s/%s", emailReceiver, p.Namespace, p.Name)
			if _, exist := c.Receivers[p.TenantID]; exist {
				delete(c.Receivers[p.TenantID], rcvKey)
				if len(c.Receivers[p.TenantID]) <= 0 {
					delete(c.Receivers, p.TenantID)
				}
			}
		case emailConfig:
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

func (c *Config) mailRcvsFromNs(namespace *string) []*nmv1alpha1.EmailReceiver {
	rcvs := make([]*nmv1alpha1.EmailReceiver, 0)
	// For notification without a namespace label, use global email receivers
	// For notifications with a namespace label, find tenantID "User" in that namespace's rolebindings
	// and then find EmailReceiver for that tenantID
	if namespace == nil {
		rcvList := nmv1alpha1.EmailReceiverList{}
		labels := make(map[string]string)
		labels[scope] = global
		ls := metav1.LabelSelector{}
		ls.MatchLabels = labels
		selector, _ := metav1.LabelSelectorAsSelector(&ls)
		if err := c.cache.List(c.ctx, &rcvList, client.MatchingLabelsSelector{Selector: selector}); err != nil {
			_ = level.Error(c.logger).Log("msg", "Failed to list global EmailReceiver", "err", err)
		}
		for _, r := range rcvList.Items {
			rcvs = append(rcvs, &r)
		}

	} else {
		if tenantIDs, err := c.TenantIDFromNs(*namespace); err != nil {
			_ = level.Error(c.logger).Log("msg", "Unable to find tenantID", "err", err)
		} else {
			for _, v := range tenantIDs {
				rcvList := nmv1alpha1.EmailReceiverList{}
				labels := make(map[string]string)
				labels[c.TenantKey] = v
				labels[scope] = tenant
				ls := metav1.LabelSelector{}
				ls.MatchLabels = labels
				selector, _ := metav1.LabelSelectorAsSelector(&ls)
				if err := c.cache.List(c.ctx, &rcvList, client.MatchingLabelsSelector{Selector: selector}); err != nil {
					_ = level.Error(c.logger).Log("msg", "Unable to list EmailReceiver", "err", err)
					continue
				}

				for _, r := range rcvList.Items {
					rcvs = append(rcvs, &r)
				}
			}
		}

	}
	return rcvs
}

func (c *Config) RcvsFromNs(namespace *string) []*Receiver {
	rcvs := make([]*Receiver, 0)
	// Get all EmailReceivers in specified namespace
	// and then generate Receivers from these EmailReceivers
	mailRcvs := c.mailRcvsFromNs(namespace)
	for _, v := range mailRcvs {
		rcvs = append(rcvs, c.generateMailReceiver(v))
	}
	// TODO: Add receiver generation logic for wechat, slack and webhook
	return rcvs
}

func (c *Config) generateEmailGlobalConfig(nm *nmv1alpha1.NotificationManager) (*config.GlobalConfig, error) {
	global := &config.GlobalConfig{}
	mcList := nmv1alpha1.EmailConfigList{}
	selector, _ := metav1.LabelSelectorAsSelector(nm.Spec.GlobalConfigSelector)
	if err := c.cache.List(c.ctx, &mcList, client.MatchingLabelsSelector{Selector: selector}); err != nil {
		_ = level.Error(c.logger).Log("msg", "Unable to list EmailConfig", "err", err)
		return nil, err
	}

	for _, mc := range mcList.Items {
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
		break
	}
	return global, nil
}

func (c *Config) OnNmAdd(obj interface{}) {
	if nm, ok := obj.(*nmv1alpha1.NotificationManager); ok {
		p := &param{}
		p.Op = opAdd
		p.Name = nm.Name
		p.Namespace = nm.Namespace
		p.Type = globalEmailConfig
		p.GlobalEmailConfig, _ = c.generateEmailGlobalConfig(nm)
		p.TenantKey = nm.Spec.Receivers.TenantKey
		p.ReceiverSelector = nm.Spec.Receivers.ReceiverSelector
		p.done = make(chan interface{}, 1)
		c.ch <- p
		<-p.done
	}
}

func (c *Config) OnNmDel(obj interface{}) {
	if _, ok := obj.(*nmv1alpha1.NotificationManager); ok {
		p := &param{}
		p.Op = opDel
		p.Type = globalEmailConfig
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
		break
	}

	to := ""
	for _, v := range mr.Spec.To {
		to += v + ","
	}
	to = strings.TrimSuffix(to, ",")
	rcv.Email.To = to

	return rcv
}

func (c *Config) OnMailRcvAdd(obj interface{}) {
	if mr, ok := obj.(*nmv1alpha1.EmailReceiver); ok {
		p := &param{}
		p.Op = opAdd
		p.TenantID = mr.ObjectMeta.Labels[c.TenantKey]
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
		p.TenantID = mr.ObjectMeta.Labels[c.TenantKey]
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

	return rcv
}

func (c *Config) OnMailConfAdd(obj interface{}) {
	if mc, ok := obj.(*nmv1alpha1.EmailConfig); ok {
		p := &param{}
		p.Op = opAdd

		if _, exists := mc.ObjectMeta.Labels[scope]; exists {
			switch mc.ObjectMeta.Labels[scope] {
			case global:
				p.Type = globalEmailConfig
			case tenant:
				if _, exists := mc.ObjectMeta.Labels[c.TenantKey]; exists {
					p.Type = emailConfig
					p.TenantID = mc.ObjectMeta.Labels[c.TenantKey]
					p.Receiver = c.generateMailConfig(mc)
				} else {
					return
				}
			default:
				return
			}
		} else {
			return
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
		if _, exists := mc.ObjectMeta.Labels[scope]; exists {
			switch mc.ObjectMeta.Labels[scope] {
			case global:
				p.Type = globalEmailConfig
			case tenant:
				if _, exists := mc.ObjectMeta.Labels[c.TenantKey]; exists {
					p.Type = emailConfig
					p.TenantID = mc.ObjectMeta.Labels[c.TenantKey]
					p.Receiver = c.generateMailConfig(mc)
				} else {
					return
				}
			default:
				return
			}
		} else {
			return
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
