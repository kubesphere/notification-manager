package config

import (
	"errors"
	"fmt"
	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/pkg/apis/v2beta1"
	"github.com/modern-go/reflect2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Receiver interface {
	Enabled() bool
	UseDefault() bool
	SetUseDefault(b bool)
	GetConfig() interface{}
	SetConfig(c interface{}) error
}

type common struct {
	enabled *bool
	// True means receiver use the default config.
	useDefault bool
}

func (c *common) Enabled() bool {
	if c.enabled == nil {
		return true
	}

	return *c.enabled
}

func (c *common) UseDefault() bool {
	return c.useDefault
}

func (c *common) SetUseDefault(b bool) {
	c.useDefault = b
}

func NewReceiver(c *Config, obj interface{}) Receiver {

	if reflect2.IsNil(obj) {
		return nil
	}

	switch obj.(type) {
	case *v2beta1.DingTalkReceiver:
		return NewDingTalkReceiver(c, obj.(*v2beta1.DingTalkReceiver))
	case *v2beta1.DingTalkConfig:
		return NewDingTalkConfig(obj.(*v2beta1.DingTalkConfig))
	case *v2beta1.EmailReceiver:
		return NewEmailReceiver(c, obj.(*v2beta1.EmailReceiver))
	case *v2beta1.EmailConfig:
		return NewEmailConfig(obj.(*v2beta1.EmailConfig))
	case *v2beta1.SlackReceiver:
		return NewSlackReceiver(c, obj.(*v2beta1.SlackReceiver))
	case *v2beta1.SlackConfig:
		return NewSlackConfig(obj.(*v2beta1.SlackConfig))
	case *v2beta1.WebhookReceiver:
		return NewWebhookReceiver(c, obj.(*v2beta1.WebhookReceiver))
	case *v2beta1.WebhookConfig:
		return NewWebhookConfig(obj.(*v2beta1.WebhookConfig))
	case *v2beta1.WechatReceiver:
		return NewWechatReceiver(c, obj.(*v2beta1.WechatReceiver))
	case *v2beta1.WechatConfig:
		return NewWechatConfig(obj.(*v2beta1.WechatConfig))
	default:
		return nil
	}
}

func getOpType(obj interface{}) string {

	if reflect2.IsNil(obj) {
		return ""
	}

	switch obj.(type) {
	case *v2beta1.DingTalkReceiver, *v2beta1.DingTalkConfig:
		return dingtalk
	case *v2beta1.EmailReceiver, *v2beta1.EmailConfig:
		return email
	case *v2beta1.SlackReceiver, *v2beta1.SlackConfig:
		return slack
	case *v2beta1.WebhookReceiver, *v2beta1.WebhookConfig:
		return webhook
	case *v2beta1.WechatReceiver, *v2beta1.WechatConfig:
		return wechat
	default:
		return ""
	}
}

type DingTalk struct {
	ChatIDs        []string
	ChatBot        *DingTalkChatBot
	DingTalkConfig *DingTalkConfig
	Selector       *metav1.LabelSelector
	*common
}

type DingTalkConfig struct {
	AppKey    *v2beta1.SecretKeySelector
	AppSecret *v2beta1.SecretKeySelector
}

// Configuration of ChatBot
type DingTalkChatBot struct {
	Webhook  *v2beta1.SecretKeySelector
	Keywords []string
	Secret   *v2beta1.SecretKeySelector
}

func NewDingTalkReceiver(c *Config, dr *v2beta1.DingTalkReceiver) Receiver {

	d := &DingTalk{
		common: &common{
			enabled: dr.Enabled,
		},
		Selector: dr.AlertSelector,
	}

	if dr.Conversation != nil {
		d.ChatIDs = dr.Conversation.ChatIDs
	}

	if dr.ChatBot != nil {
		d.ChatBot = &DingTalkChatBot{
			Webhook:  dr.ChatBot.Webhook,
			Keywords: dr.ChatBot.Keywords,
			Secret:   dr.ChatBot.Secret,
		}
	}

	configs := listConfigs(c, dr.DingTalkConfigSelector)
	if configs == nil {
		return d
	}

	for _, config := range configs {
		dc := config.Spec.DingTalk
		d.generateConfig(dc)
		if d.DingTalkConfig != nil {
			break
		}
	}

	return d
}

func NewDingTalkConfig(dc *v2beta1.DingTalkConfig) Receiver {
	d := &DingTalk{
		common: &common{},
	}

	d.generateConfig(dc)
	return d
}

func (d *DingTalk) generateConfig(dc *v2beta1.DingTalkConfig) {

	if dc == nil {
		return
	}

	dingtalkConfig := &DingTalkConfig{}

	if dc.Conversation != nil {
		if dc.Conversation.AppKey != nil {
			dingtalkConfig.AppKey = dc.Conversation.AppKey
		}

		if dc.Conversation.AppSecret != nil {
			dingtalkConfig.AppSecret = dc.Conversation.AppSecret
		}
	}

	d.DingTalkConfig = dingtalkConfig
	return
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

type Email struct {
	To          []string
	EmailConfig *EmailConfig
	Selector    *metav1.LabelSelector
	*common
}

type EmailConfig struct {
	From         string
	SmartHost    v2beta1.HostPort
	Hello        string
	AuthUsername string
	AuthIdentify string
	AuthPassword *v2beta1.SecretKeySelector
	AuthSecret   *v2beta1.SecretKeySelector
	RequireTLS   bool
	TLS          *v2beta1.TLSConfig
}

func NewEmailReceiver(c *Config, er *v2beta1.EmailReceiver) Receiver {
	e := &Email{
		common: &common{
			enabled: er.Enabled,
		},
		To:       er.To,
		Selector: er.AlertSelector,
	}

	configs := listConfigs(c, er.EmailConfigSelector)
	if configs == nil {
		return e
	}

	for _, item := range configs {
		e.generateConfig(item.Spec.Email)
		if e.EmailConfig != nil {
			break
		}
	}

	return e
}

func NewEmailConfig(ec *v2beta1.EmailConfig) Receiver {
	e := &Email{
		common: &common{},
	}

	e.generateConfig(ec)
	return e
}

func (e *Email) generateConfig(ec *v2beta1.EmailConfig) {

	if ec == nil {
		return
	}

	emailConfig := &EmailConfig{
		From:         ec.From,
		SmartHost:    ec.SmartHost,
		AuthPassword: ec.AuthPassword,
		AuthSecret:   ec.AuthSecret,
	}

	if ec.Hello != nil {
		emailConfig.Hello = *ec.Hello
	}

	if ec.AuthIdentify != nil {
		emailConfig.AuthIdentify = *ec.AuthIdentify
	}

	if ec.RequireTLS != nil {
		emailConfig.RequireTLS = *ec.RequireTLS
	}

	if ec.TLS != nil {
		emailConfig.TLS = ec.TLS
	}

	if ec.AuthUsername != nil {
		emailConfig.AuthUsername = *ec.AuthUsername
	}

	e.EmailConfig = emailConfig
	return
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

type Slack struct {
	// The channel or user to send notifications to.
	Channels    []string
	SlackConfig *SlackConfig
	Selector    *metav1.LabelSelector
	*common
}

type SlackConfig struct {
	// The token of user or bot.
	Token *v2beta1.SecretKeySelector
}

func NewSlackReceiver(c *Config, sr *v2beta1.SlackReceiver) Receiver {
	s := &Slack{
		common: &common{
			enabled: sr.Enabled,
		},
		Channels: sr.Channels,
		Selector: sr.AlertSelector,
	}

	configs := listConfigs(c, sr.SlackConfigSelector)
	if configs == nil {
		return s
	}

	for _, item := range configs {
		s.generateConfig(item.Spec.Slack)
		if s.SlackConfig != nil {
			break
		}
	}

	return s
}

func NewSlackConfig(sc *v2beta1.SlackConfig) Receiver {
	s := &Slack{
		common: &common{},
	}

	s.generateConfig(sc)
	return s
}

func (s *Slack) generateConfig(sc *v2beta1.SlackConfig) {

	if sc == nil || sc.SlackTokenSecret == nil {
		return
	}

	s.SlackConfig = &SlackConfig{
		Token: sc.SlackTokenSecret,
	}

	return
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

type Webhook struct {
	// `url` gives the location of the webhook, in standard URL form.
	URL           string
	HttpConfig    *v2beta1.HTTPClientConfig
	WebhookConfig *WebhookConfig
	Selector      *metav1.LabelSelector
	*common
}

type WebhookConfig struct {
}

func NewWebhookReceiver(_ *Config, wr *v2beta1.WebhookReceiver) Receiver {
	w := &Webhook{
		common: &common{
			enabled: wr.Enabled,
		},
		Selector:   wr.AlertSelector,
		HttpConfig: wr.HTTPConfig,
	}

	if wr.URL != nil {
		w.URL = *wr.URL
	} else if wr.Service != nil {
		service := wr.Service
		if service.Scheme == nil || len(*service.Scheme) == 0 {
			w.URL = fmt.Sprintf("http://%s.%s", service.Name, service.Namespace)
		} else {
			w.URL = fmt.Sprintf("%s://%s.%s", *service.Scheme, service.Name, service.Namespace)
		}

		if service.Port != nil {
			w.URL = fmt.Sprintf("%s:%d/", w.URL, *service.Port)
		}

		if service.Path != nil {
			w.URL = fmt.Sprintf("%s%s", w.URL, *service.Path)
		}
	}

	return w
}

func NewWebhookConfig(_ *v2beta1.WebhookConfig) Receiver {
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

type Wechat struct {
	ToUser       []string
	ToParty      []string
	ToTag        []string
	WechatConfig *WechatConfig
	Selector     *metav1.LabelSelector
	*common
}

type WechatConfig struct {
	APISecret *v2beta1.SecretKeySelector
	CorpID    string
	APIURL    string
	AgentID   string
}

func NewWechatReceiver(c *Config, wr *v2beta1.WechatReceiver) Receiver {
	w := &Wechat{
		common: &common{
			enabled: wr.Enabled,
		},
		ToUser:   wr.ToUser,
		ToParty:  wr.ToParty,
		ToTag:    wr.ToTag,
		Selector: wr.AlertSelector,
	}

	configs := listConfigs(c, wr.WechatConfigSelector)

	for _, item := range configs {
		w.generateConfig(item.Spec.Wechat)
		if w.WechatConfig != nil {
			break
		}
	}

	return w
}

func NewWechatConfig(wc *v2beta1.WechatConfig) Receiver {
	w := &Wechat{
		common: &common{},
	}

	w.generateConfig(wc)
	return w
}

func (w *Wechat) generateConfig(wc *v2beta1.WechatConfig) {

	if wc == nil || wc.WechatApiSecret == nil {
		return
	}

	w.WechatConfig = &WechatConfig{
		APIURL:    wc.WechatApiUrl,
		AgentID:   wc.WechatApiAgentId,
		CorpID:    wc.WechatApiCorpId,
		APISecret: wc.WechatApiSecret,
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

func listConfigs(c *Config, selector *metav1.LabelSelector) []v2beta1.Config {
	configList := v2beta1.ConfigList{}
	configSel, _ := metav1.LabelSelectorAsSelector(selector)
	if err := c.cache.List(c.ctx, &configList, client.MatchingLabelsSelector{Selector: configSel}); client.IgnoreNotFound(err) != nil {
		_ = level.Error(c.logger).Log("msg", "Unable to list DingTalkConfig", "err", err)
		return nil
	}

	return configList.Items
}
