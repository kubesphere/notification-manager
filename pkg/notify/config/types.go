package config

import (
	"errors"
	"fmt"
	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/pkg/apis/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type factory struct {
	key string

	// newReceiverFunc returns a new receiver of this resource
	newReceiverFunc func() Receiver

	// newReceiverObjectFunc returns a new receiver instance of this resource
	newReceiverObjectFunc func() runtime.Object

	// NewListFunc returns a new receiver list of this resource
	newReceiverObjectListFunc func() runtime.Object

	// NewFunc returns a new config instance of this resource
	newConfigObjectFunc func() runtime.Object

	// NewListFunc returns a new config list of this resource
	newConfigObjectListFunc func() runtime.Object
}

type Receiver interface {
	UseDefault() bool
	SetUseDefault(b bool)
	GetConfig() interface{}
	SetConfig(c interface{}) error
	GenerateConfig(c *Config, obj interface{})
	GenerateReceiver(c *Config, obj interface{})
}

type common struct {
	// True means receiver use the default config.
	useDefault bool
}

func (c *common) UseDefault() bool {
	return c.useDefault
}

func (c *common) SetUseDefault(b bool) {
	c.useDefault = b
}

type DingTalk struct {
	DingTalkConfig *DingTalkConfig
	Selector       *metav1.LabelSelector
	*common
}

type DingTalkConfig struct {
	ChatBot      *DingTalkChatBot
	Conversation *DingTalkConversation
}

// Configuration of ChatBot
type DingTalkChatBot struct {
	Webhook  *v2.SecretKeySelector
	Keywords []string
	Secret   *v2.SecretKeySelector
}

// Configuration of conversation
type DingTalkConversation struct {
	AppKey    *v2.SecretKeySelector
	AppSecret *v2.SecretKeySelector
	ChatID    string
}

func NewDingTalkReceiver() Receiver {
	return &DingTalk{
		common: &common{},
	}
}

func (d *DingTalk) GetConfig() interface{} {
	return d.DingTalkConfig
}

func (d *DingTalk) SetConfig(obj interface{}) error {

	if obj == nil {
		d.DingTalkConfig = nil
		return nil
	}

	c, ok := obj.(*DingTalkConfig)
	if !ok {
		return errors.New("set dingtalk config error, wrong config type")
	}

	d.DingTalkConfig = c
	return nil
}

func (d *DingTalk) GenerateConfig(c *Config, obj interface{}) {

	dc, ok := obj.(*v2.DingTalkConfig)
	if !ok {
		_ = level.Warn(c.logger).Log("msg", "generate dingtalk config error, wrong config type")
		return
	}

	dingtalkConfig := &DingTalkConfig{}

	if dc.Spec.ChatBot != nil {
		dingtalkConfig.ChatBot = &DingTalkChatBot{
			Webhook:  dc.Spec.ChatBot.Webhook,
			Keywords: dc.Spec.ChatBot.Keywords,
			Secret:   dc.Spec.ChatBot.Secret,
		}
	}

	if dc.Spec.Conversation != nil {
		dingtalkConfig.Conversation = &DingTalkConversation{
			ChatID:    dc.Spec.Conversation.ChatID,
			AppKey:    dc.Spec.Conversation.AppKey,
			AppSecret: dc.Spec.Conversation.AppSecret,
		}
	}

	d.DingTalkConfig = dingtalkConfig
	return
}

func (d *DingTalk) GenerateReceiver(c *Config, obj interface{}) {

	dr, ok := obj.(*v2.DingTalkReceiver)
	if !ok {
		_ = level.Warn(c.logger).Log("msg", "generate dingtalk receiver error, wrong receiver type")
		return
	}

	d.Selector = dr.Spec.NotificationSelector

	dcList := v2.DingTalkConfigList{}
	dcSel, _ := metav1.LabelSelectorAsSelector(dr.Spec.DingTalkConfigSelector)
	if err := c.cache.List(c.ctx, &dcList, client.MatchingLabelsSelector{Selector: dcSel}); client.IgnoreNotFound(err) != nil {
		_ = level.Error(c.logger).Log("msg", "Unable to list DingTalkConfig", "err", err)
		return
	}

	for _, dc := range dcList.Items {
		d.GenerateConfig(c, &dc)
		if d.DingTalkConfig != nil {
			break
		}
	}
	return
}

type Email struct {
	To          []string
	EmailConfig *EmailConfig
	Selector    *metav1.LabelSelector
	*common
}

type EmailConfig struct {
	From         string
	SmartHost    v2.HostPort
	Hello        string
	AuthUsername string
	AuthIdentify string
	AuthPassword *v2.SecretKeySelector
	AuthSecret   *v2.SecretKeySelector
	RequireTLS   bool
	TLS          *v2.TLSConfig
}

func NewEmailReceiver() Receiver {
	return &Email{
		common: &common{},
	}
}

func NewEmail(to []string) *Email {
	return &Email{
		To:     to,
		common: &common{},
	}
}

func (e *Email) GetConfig() interface{} {
	return e.EmailConfig
}

func (e *Email) SetConfig(obj interface{}) error {

	if obj == nil {
		e.EmailConfig = nil
		return nil
	}

	c, ok := obj.(*EmailConfig)
	if !ok {
		return errors.New("set email config error, wrong config type")
	}

	e.EmailConfig = c
	return nil
}

func (e *Email) GenerateConfig(c *Config, obj interface{}) {

	ec, ok := obj.(*v2.EmailConfig)
	if !ok {
		_ = level.Warn(c.logger).Log("msg", "generate email config error, wrong config type")
		return
	}

	emailConfig := &EmailConfig{
		From:         ec.Spec.From,
		SmartHost:    ec.Spec.SmartHost,
		AuthPassword: ec.Spec.AuthPassword,
		AuthSecret:   ec.Spec.AuthSecret,
	}

	if ec.Spec.Hello != nil {
		emailConfig.Hello = *ec.Spec.Hello
	}

	if ec.Spec.AuthIdentify != nil {
		emailConfig.AuthIdentify = *ec.Spec.AuthIdentify
	}

	if ec.Spec.RequireTLS != nil {
		emailConfig.RequireTLS = *ec.Spec.RequireTLS
	}

	if ec.Spec.TLS != nil {
		emailConfig.TLS = ec.Spec.TLS
	}

	if ec.Spec.AuthUsername != nil {
		emailConfig.AuthUsername = *ec.Spec.AuthUsername
	}

	e.EmailConfig = emailConfig
	return
}

func (e *Email) GenerateReceiver(c *Config, obj interface{}) {

	er, ok := obj.(*v2.EmailReceiver)
	if !ok {
		_ = level.Warn(c.logger).Log("msg", "generate email receiver error, wrong receiver type")
		return
	}

	e.To = er.Spec.To
	e.Selector = er.Spec.NotificationSelector

	ecList := v2.EmailConfigList{}
	ecSel, _ := metav1.LabelSelectorAsSelector(er.Spec.EmailConfigSelector)
	if err := c.cache.List(c.ctx, &ecList, client.MatchingLabelsSelector{Selector: ecSel}); client.IgnoreNotFound(err) != nil {
		_ = level.Error(c.logger).Log("msg", "Unable to list EmailConfig", "err", err)
		return
	}

	for _, ec := range ecList.Items {
		e.GenerateConfig(c, &ec)
		if e.EmailConfig != nil {
			break
		}
	}
}

type Slack struct {
	// The channel or user to send notifications to.
	Channel     string
	SlackConfig *SlackConfig
	Selector    *metav1.LabelSelector
	*common
}

type SlackConfig struct {
	// The token of user or bot.
	Token *v2.SecretKeySelector
}

func NewSlackReceiver() Receiver {
	return &Slack{
		common: &common{},
	}
}

func (s *Slack) GetConfig() interface{} {
	return s.SlackConfig
}

func (s *Slack) SetConfig(obj interface{}) error {

	if obj == nil {
		s.SlackConfig = nil
		return nil
	}

	c, ok := obj.(*SlackConfig)
	if !ok {
		return errors.New("set slack config error, wrong config type")
	}

	s.SlackConfig = c
	return nil
}

func (s *Slack) GenerateConfig(c *Config, obj interface{}) {

	sc, ok := obj.(*v2.SlackConfig)
	if !ok {
		_ = level.Warn(c.logger).Log("msg", "generate slack config error, wrong config type")
		return
	}

	if sc.Spec.SlackTokenSecret == nil {
		_ = level.Error(c.logger).Log("msg", "ignore slack config because of empty token", "name", sc.Name)
		return
	}

	s.SlackConfig = &SlackConfig{
		Token: sc.Spec.SlackTokenSecret,
	}

	return
}

func (s *Slack) GenerateReceiver(c *Config, obj interface{}) {

	sr, ok := obj.(*v2.SlackReceiver)
	if !ok {
		_ = level.Warn(c.logger).Log("msg", "generate slack receiver error, wrong receiver type")
		return
	}

	scList := v2.SlackConfigList{}
	scSel, _ := metav1.LabelSelectorAsSelector(sr.Spec.SlackConfigSelector)
	if err := c.cache.List(c.ctx, &scList, client.MatchingLabelsSelector{Selector: scSel}); client.IgnoreNotFound(err) != nil {
		_ = level.Error(c.logger).Log("msg", "Unable to list SlackConfig", "err", err)
		return
	}

	s.Channel = sr.Spec.Channel
	s.Selector = sr.Spec.NotificationSelector

	for _, sc := range scList.Items {
		s.GenerateConfig(c, &sc)
		if s.SlackConfig != nil {
			break
		}
	}

	return
}

type Webhook struct {
	WebhookConfig *WebhookConfig
	Selector      *metav1.LabelSelector
	*common
}

type WebhookConfig struct {
	// `url` gives the location of the webhook, in standard URL form.
	URL        string
	HttpConfig *v2.HTTPClientConfig
}

func NewWebhookReceiver() Receiver {
	return &Webhook{
		common: &common{},
	}
}

func (w *Webhook) GetConfig() interface{} {
	return w.WebhookConfig
}

func (w *Webhook) SetConfig(obj interface{}) error {

	if obj == nil {
		w.WebhookConfig = nil
		return nil
	}

	c, ok := obj.(*WebhookConfig)
	if !ok {
		return errors.New("set webhook config error, wrong config type")
	}

	w.WebhookConfig = c
	return nil
}

func (w *Webhook) GenerateConfig(c *Config, obj interface{}) {

	wc, ok := obj.(*v2.WebhookConfig)
	if !ok {
		_ = level.Warn(c.logger).Log("msg", "generate webhook config error, wrong config type")
		return
	}

	webhookConfig := &WebhookConfig{
		HttpConfig: wc.Spec.HTTPConfig,
	}

	if wc.Spec.URL != nil {
		webhookConfig.URL = *wc.Spec.URL
	} else if wc.Spec.Service != nil {
		service := wc.Spec.Service
		if service.Scheme == nil || len(*service.Scheme) == 0 {
			webhookConfig.URL = fmt.Sprintf("http://%s.%s", service.Name, service.Namespace)
		} else {
			webhookConfig.URL = fmt.Sprintf("%s://%s.%s", *service.Scheme, service.Name, service.Namespace)
		}

		if service.Port != nil {
			webhookConfig.URL = fmt.Sprintf("%s:%d/", webhookConfig.URL, *service.Port)
		}

		if service.Path != nil {
			webhookConfig.URL = fmt.Sprintf("%s%s", webhookConfig.URL, *service.Path)
		}
	} else {
		_ = level.Error(c.logger).Log("msg", "ignore webhook config because of empty config", "name", wc.Name)
		return
	}

	w.WebhookConfig = webhookConfig
}

func (w *Webhook) GenerateReceiver(c *Config, obj interface{}) {

	wr, ok := obj.(*v2.WebhookReceiver)
	if !ok {
		_ = level.Warn(c.logger).Log("msg", "generate webhook receiver error, wrong receiver type")
		return
	}

	w.Selector = wr.Spec.NotificationSelector

	wcList := v2.WebhookConfigList{}
	wcSel, _ := metav1.LabelSelectorAsSelector(wr.Spec.WebhookConfigSelector)
	if err := c.cache.List(c.ctx, &wcList, client.MatchingLabelsSelector{Selector: wcSel}); client.IgnoreNotFound(err) != nil {
		_ = level.Error(c.logger).Log("msg", "Unable to list WebhookConfig", "err", err)
		return
	}

	for _, wc := range wcList.Items {
		w.GenerateConfig(c, &wc)
		if w.WebhookConfig != nil {
			break
		}
	}
}

type Wechat struct {
	ToUser       string
	ToParty      string
	ToTag        string
	WechatConfig *WechatConfig
	Selector     *metav1.LabelSelector
	*common
}

type WechatConfig struct {
	APISecret *v2.SecretKeySelector
	CorpID    string
	APIURL    string
	AgentID   string
}

func NewWechatReceiver() Receiver {
	return &Wechat{
		common: &common{},
	}
}

func (w *Wechat) GetConfig() interface{} {
	return w.WechatConfig
}

func (w *Wechat) SetConfig(obj interface{}) error {

	if obj == nil {
		w.WechatConfig = nil
		return nil
	}

	c, ok := obj.(*WechatConfig)
	if !ok {
		return errors.New("set wechat config error, wrong config type")
	}

	w.WechatConfig = c
	return nil
}

func (w *Wechat) GenerateConfig(c *Config, obj interface{}) {
	wc, ok := obj.(*v2.WechatConfig)
	if !ok {
		_ = level.Warn(c.logger).Log("msg", "generate wechat config error, wrong config type")
		return
	}

	if wc.Spec.WechatApiSecret == nil {
		_ = level.Error(c.logger).Log("msg", "ignore wechat config because of empty api secret", "name", wc.Name)
		return
	}

	w.WechatConfig = &WechatConfig{
		APIURL:    wc.Spec.WechatApiUrl,
		AgentID:   wc.Spec.WechatApiAgentId,
		CorpID:    wc.Spec.WechatApiCorpId,
		APISecret: wc.Spec.WechatApiSecret,
	}
}

func (w *Wechat) GenerateReceiver(c *Config, obj interface{}) {

	wr, ok := obj.(*v2.WechatReceiver)
	if !ok {
		_ = level.Warn(c.logger).Log("msg", "generate wechat receiver error, wrong receiver type")
		return
	}

	wcList := v2.WechatConfigList{}
	wcSel, _ := metav1.LabelSelectorAsSelector(wr.Spec.WechatConfigSelector)
	if err := c.cache.List(c.ctx, &wcList, client.MatchingLabelsSelector{Selector: wcSel}); client.IgnoreNotFound(err) != nil {
		_ = level.Error(c.logger).Log("msg", "Unable to list WechatConfig", "err", err)
		return
	}

	w.ToUser = wr.Spec.ToUser
	w.ToParty = wr.Spec.ToParty
	w.ToTag = wr.Spec.ToTag
	w.Selector = wr.Spec.NotificationSelector

	for _, wc := range wcList.Items {
		w.GenerateConfig(c, &wc)
		if w.WechatConfig != nil {
			break
		}
	}
}

func (w *Wechat) Clone() *Wechat {

	return &Wechat{
		common: &common{},
		WechatConfig: &WechatConfig{
			APISecret: w.WechatConfig.APISecret,
			CorpID:    w.WechatConfig.CorpID,
			APIURL:    w.WechatConfig.APIURL,
			AgentID:   w.WechatConfig.AgentID,
		},
		ToUser:  w.ToUser,
		ToParty: w.ToParty,
		ToTag:   w.ToTag,
	}
}
