package config

import (
	"errors"
	"fmt"

	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/pkg/apis/v2beta2"
	"github.com/modern-go/reflect2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Receiver interface {
	Enabled() bool
	UseDefault() bool
	SetUseDefault(b bool)
	GetType() string
	GetConfigSelector() *metav1.LabelSelector
	GetConfig() interface{}
	SetConfig(c interface{}) error
	Validate() error
}

type common struct {
	enabled *bool
	// True means receiver use the default config.
	useDefault bool
	// The type of receiver.
	receiverType string
	// The config selector of receiver
	configSelector *metav1.LabelSelector
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

func (c *common) GetType() string {
	return c.receiverType
}

func (c *common) GetConfigSelector() *metav1.LabelSelector {
	return c.configSelector
}

func NewReceiver(c *Config, obj interface{}) Receiver {

	if reflect2.IsNil(obj) {
		return nil
	}

	switch obj.(type) {
	case *v2beta2.DingTalkReceiver:
		return NewDingTalkReceiver(c, obj.(*v2beta2.DingTalkReceiver))
	case *v2beta2.DingTalkConfig:
		return NewDingTalkConfig(obj.(*v2beta2.DingTalkConfig))
	case *v2beta2.EmailReceiver:
		return NewEmailReceiver(c, obj.(*v2beta2.EmailReceiver))
	case *v2beta2.EmailConfig:
		return NewEmailConfig(obj.(*v2beta2.EmailConfig))
	case *v2beta2.SlackReceiver:
		return NewSlackReceiver(c, obj.(*v2beta2.SlackReceiver))
	case *v2beta2.SlackConfig:
		return NewSlackConfig(obj.(*v2beta2.SlackConfig))
	case *v2beta2.WebhookReceiver:
		return NewWebhookReceiver(c, obj.(*v2beta2.WebhookReceiver))
	case *v2beta2.WebhookConfig:
		return NewWebhookConfig(obj.(*v2beta2.WebhookConfig))
	case *v2beta2.WechatReceiver:
		return NewWechatReceiver(c, obj.(*v2beta2.WechatReceiver))
	case *v2beta2.WechatConfig:
		return NewWechatConfig(obj.(*v2beta2.WechatConfig))
	default:
		return nil
	}
}

func getOpType(obj interface{}) string {

	if reflect2.IsNil(obj) {
		return ""
	}

	switch obj.(type) {
	case *v2beta2.DingTalkReceiver, *v2beta2.DingTalkConfig:
		return dingtalk
	case *v2beta2.EmailReceiver, *v2beta2.EmailConfig:
		return email
	case *v2beta2.SlackReceiver, *v2beta2.SlackConfig:
		return slack
	case *v2beta2.WebhookReceiver, *v2beta2.WebhookConfig:
		return webhook
	case *v2beta2.WechatReceiver, *v2beta2.WechatConfig:
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
	AppKey    *v2beta2.Credential
	AppSecret *v2beta2.Credential
}

// DingTalkChatBot is the configuration of ChatBot
type DingTalkChatBot struct {
	Webhook  *v2beta2.Credential
	Keywords []string
	Secret   *v2beta2.Credential
}

func NewDingTalkReceiver(c *Config, dr *v2beta2.DingTalkReceiver) Receiver {

	d := &DingTalk{
		common: &common{
			enabled:        dr.Enabled,
			receiverType:   dingtalk,
			configSelector: dr.DingTalkConfigSelector,
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

func NewDingTalkConfig(dc *v2beta2.DingTalkConfig) Receiver {
	d := &DingTalk{
		common: &common{
			receiverType: dingtalk,
		},
	}

	d.generateConfig(dc)
	return d
}

func (d *DingTalk) generateConfig(dc *v2beta2.DingTalkConfig) {

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

func (d *DingTalk) Validate() error {

	if d.ChatBot == nil && (d.ChatIDs == nil || len(d.ChatIDs) == 0) {
		return fmt.Errorf("%s", "must specify one of: `chatbot` or `chatIDs`")
	}

	if d.ChatBot != nil {
		if err := validateCredential(d.ChatBot.Webhook); err != nil {
			return fmt.Errorf("chatbot webhook error, %s", err.Error())
		}

		if d.ChatBot.Secret != nil {
			if err := validateCredential(d.ChatBot.Secret); err != nil {
				return fmt.Errorf("chatbot secret error, %s", err.Error())
			}
		}
	}

	if d.ChatIDs != nil && len(d.ChatIDs) > 0 {
		if d.DingTalkConfig == nil {
			return fmt.Errorf("config is nil")
		}

		if err := validateCredential(d.DingTalkConfig.AppKey); err != nil {
			return fmt.Errorf("appkey error, %s", err.Error())
		}

		if err := validateCredential(d.DingTalkConfig.AppSecret); err != nil {
			return fmt.Errorf("appsecret error, %s", err.Error())
		}
	}

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
	SmartHost    v2beta2.HostPort
	Hello        string
	AuthUsername string
	AuthIdentify string
	AuthPassword *v2beta2.Credential
	AuthSecret   *v2beta2.Credential
	RequireTLS   bool
	TLS          *v2beta2.TLSConfig
}

func NewEmailReceiver(c *Config, er *v2beta2.EmailReceiver) Receiver {
	e := &Email{
		common: &common{
			enabled:        er.Enabled,
			receiverType:   email,
			configSelector: er.EmailConfigSelector,
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

func NewEmailConfig(ec *v2beta2.EmailConfig) Receiver {
	e := &Email{
		common: &common{
			receiverType: email,
		},
	}

	e.generateConfig(ec)
	return e
}

func (e *Email) generateConfig(ec *v2beta2.EmailConfig) {

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

func (e *Email) Validate() error {

	if e.To == nil || len(e.To) == 0 {
		return fmt.Errorf("email receivers is nil")
	}

	if e.EmailConfig == nil {
		return fmt.Errorf("config is nil")
	}

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
	Token *v2beta2.Credential
}

func NewSlackReceiver(c *Config, sr *v2beta2.SlackReceiver) Receiver {
	s := &Slack{
		common: &common{
			enabled:        sr.Enabled,
			receiverType:   slack,
			configSelector: sr.SlackConfigSelector,
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

func NewSlackConfig(sc *v2beta2.SlackConfig) Receiver {
	s := &Slack{
		common: &common{
			receiverType: slack,
		},
	}

	s.generateConfig(sc)
	return s
}

func (s *Slack) generateConfig(sc *v2beta2.SlackConfig) {

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

func (s *Slack) Validate() error {

	if s.Channels == nil || len(s.Channels) == 0 {
		return fmt.Errorf("channel must be specified")
	}

	if s.SlackConfig == nil {
		return fmt.Errorf("config is nil")
	}

	if err := validateCredential(s.SlackConfig.Token); err != nil {
		return fmt.Errorf("slack token error, %s", err.Error())
	}

	return nil
}

type Webhook struct {
	// `url` gives the location of the webhook, in standard URL form.
	URL           string
	HttpConfig    *v2beta2.HTTPClientConfig
	WebhookConfig *WebhookConfig
	Selector      *metav1.LabelSelector
	*common
}

type WebhookConfig struct {
}

func NewWebhookReceiver(_ *Config, wr *v2beta2.WebhookReceiver) Receiver {
	w := &Webhook{
		common: &common{
			enabled:        wr.Enabled,
			receiverType:   webhook,
			configSelector: wr.WebhookConfigSelector,
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

func NewWebhookConfig(_ *v2beta2.WebhookConfig) Receiver {
	return &Webhook{
		common: &common{
			receiverType: webhook,
		},
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

func (w *Webhook) Validate() error {

	if len(w.URL) == 0 {
		return fmt.Errorf("url is nil")
	}

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
	APISecret *v2beta2.Credential
	CorpID    string
	APIURL    string
	AgentID   string
}

func NewWechatReceiver(c *Config, wr *v2beta2.WechatReceiver) Receiver {
	w := &Wechat{
		common: &common{
			enabled:        wr.Enabled,
			receiverType:   wechat,
			configSelector: wr.WechatConfigSelector,
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

func NewWechatConfig(wc *v2beta2.WechatConfig) Receiver {
	w := &Wechat{
		common: &common{
			receiverType: wechat,
		},
	}

	w.generateConfig(wc)
	return w
}

func (w *Wechat) generateConfig(wc *v2beta2.WechatConfig) {

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

func (w *Wechat) Validate() error {

	if (w.ToUser == nil || len(w.ToUser) == 0) &&
		(w.ToParty == nil || len(w.ToParty) == 0) &&
		(w.ToTag == nil || len(w.ToTag) == 0) {
		return fmt.Errorf("must specify one of: `toUser`, `toParty` or `toTag`")
	}

	if w.WechatConfig == nil {
		return fmt.Errorf("config is nil")
	}

	if len(w.WechatConfig.CorpID) == 0 {
		return fmt.Errorf("corpid must be specified")
	}

	if len(w.WechatConfig.AgentID) == 0 {
		return fmt.Errorf("agentid must be specified")
	}

	if err := validateCredential(w.WechatConfig.APISecret); err != nil {
		return fmt.Errorf("apisecret error, %s", err.Error())
	}

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

func listConfigs(c *Config, selector *metav1.LabelSelector) []v2beta2.Config {
	configList := v2beta2.ConfigList{}
	configSel, _ := metav1.LabelSelectorAsSelector(selector)
	if err := c.cache.List(c.ctx, &configList, client.MatchingLabelsSelector{Selector: configSel}); client.IgnoreNotFound(err) != nil {
		_ = level.Error(c.logger).Log("msg", "Unable to list DingTalkConfig", "err", err)
		return nil
	}

	return configList.Items
}

func validateCredential(c *v2beta2.Credential) error {

	if c == nil {
		return fmt.Errorf("%s", "Credential is nil")
	}

	if len(c.Value) == 0 && c.ValueFrom == nil {
		return fmt.Errorf("%s", "must specify one of: `value` or `valueFrom`")
	}

	if len(c.Value) != 0 && c.ValueFrom != nil {
		return fmt.Errorf("valueFrom may not be specified when `value` is not empty")
	}

	if c.ValueFrom != nil {
		if c.ValueFrom.SecretKeyRef == nil {
			return fmt.Errorf("secretKeyRef must be specified when valueFrom is not nil")
		}

		if len(c.ValueFrom.SecretKeyRef.Key) == 0 {
			return fmt.Errorf("key must be specified when secretKeyRef is not nil")
		}

		if len(c.ValueFrom.SecretKeyRef.Name) == 0 {
			return fmt.Errorf("name must be specified when secretKeyRef is not nil")
		}
	}

	return nil
}
