package config

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/pkg/apis/v2beta2"
	"github.com/modern-go/reflect2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	HTML     = "html"
	Text     = "text"
	Markdown = "markdown"
	Aliyun   = "aliyun"
	Tencent  = "tencent"
)

type Receiver interface {
	Enabled() bool
	UseDefault() bool
	SetUseDefault(b bool)
	GetType() string
	GetConfigSelector() *metav1.LabelSelector
	GetConfig() interface{}
	SetConfig(c interface{}) error
	GetAlertSelector() *metav1.LabelSelector
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
	case *v2beta2.SmsReceiver:
		return NewSmsReceiver(c, obj.(*v2beta2.SmsReceiver))
	case *v2beta2.SmsConfig:
		return NewSmsConfig(obj.(*v2beta2.SmsConfig))
	case *v2beta2.PushoverReceiver:
		return NewPushoverReceiver(c, obj.(*v2beta2.PushoverReceiver))
	case *v2beta2.PushoverConfig:
		return NewPushoverConfig(obj.(*v2beta2.PushoverConfig))
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
	case *v2beta2.SmsReceiver, *v2beta2.SmsConfig:
		return sms
	case *v2beta2.PushoverReceiver, *v2beta2.PushoverConfig:
		return pushover
	default:
		return ""
	}
}

type DingTalk struct {
	ChatIDs        []string
	ChatBot        *DingTalkChatBot
	DingTalkConfig *DingTalkConfig
	Selector       *metav1.LabelSelector
	Template       string
	TitleTemplate  string
	TmplType       string
	*common
}

type DingTalkConfig struct {
	AppKey    *v2beta2.Credential
	AppSecret *v2beta2.Credential
}

// DingTalkChatBot is the configuration of ChatBot
type DingTalkChatBot struct {
	Webhook   *v2beta2.Credential
	Keywords  []string
	Secret    *v2beta2.Credential
	AtMobiles []string
	AtUsers   []string
	AtAll     bool
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

	if dr.Template != nil {
		d.Template = *dr.Template
	}

	if dr.TitleTemplate != nil {
		d.TitleTemplate = *dr.TitleTemplate
	}

	if dr.TmplType != nil {
		d.TmplType = *dr.TmplType
	}

	if dr.Conversation != nil {
		d.ChatIDs = dr.Conversation.ChatIDs
	}

	if dr.ChatBot != nil {
		d.ChatBot = &DingTalkChatBot{
			Webhook:   dr.ChatBot.Webhook,
			Keywords:  dr.ChatBot.Keywords,
			Secret:    dr.ChatBot.Secret,
			AtMobiles: dr.ChatBot.AtMobiles,
			AtUsers:   dr.ChatBot.AtUsers,
			AtAll:     dr.ChatBot.AtAll,
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

func (d *DingTalk) GetAlertSelector() *metav1.LabelSelector {
	return d.Selector
}

func (d *DingTalk) Validate() error {

	if d.TmplType != "" && d.TmplType != Text && d.TmplType != Markdown {
		return fmt.Errorf("dingtalk tmplType must be one of: `text` or `markdown`")
	}

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
	Template        string
	SubjectTemplate string
	TmplType        string
	To              []string
	EmailConfig     *EmailConfig
	Selector        *metav1.LabelSelector
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

	if er.Template != nil {
		e.Template = *er.Template
	}

	if er.SubjectTemplate != nil {
		e.SubjectTemplate = *er.SubjectTemplate
	}

	if er.TmplType != nil {
		e.TmplType = *er.TmplType
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

func NewEmail(e *Email) *Email {
	return &Email{
		Template:        e.Template,
		TmplType:        e.TmplType,
		SubjectTemplate: e.SubjectTemplate,
		Selector:        e.Selector,
		To:              e.To,
		common:          &common{},
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

func (e *Email) GetAlertSelector() *metav1.LabelSelector {
	return e.Selector
}

func (e *Email) Validate() error {

	if e.TmplType != "" && e.TmplType != Text && e.TmplType != HTML {
		return fmt.Errorf("email tmplType must be one of: `text` or `html`")
	}

	if e.To == nil || len(e.To) == 0 {
		return fmt.Errorf("email receivers is nil")
	}

	if e.EmailConfig == nil {
		return fmt.Errorf("config is nil")
	}

	return nil
}

type Slack struct {
	Template string
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

	if sr.Template != nil {
		s.Template = *sr.Template
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

func (s *Slack) GetAlertSelector() *metav1.LabelSelector {
	return s.Selector
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
	Template string
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

	if wr.Template != nil {
		w.Template = *wr.Template
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

func (w *Webhook) GetAlertSelector() *metav1.LabelSelector {
	return w.Selector
}

func (w *Webhook) Validate() error {

	if len(w.URL) == 0 {
		return fmt.Errorf("url is nil")
	}

	return nil
}

type Wechat struct {
	Template     string
	TmplType     string
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

	if wr.Template != nil {
		w.Template = *wr.Template
	}

	if wr.TmplType != nil {
		w.TmplType = *wr.TmplType
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

func (w *Wechat) GetAlertSelector() *metav1.LabelSelector {
	return w.Selector
}

func (w *Wechat) Validate() error {

	if (w.ToUser == nil || len(w.ToUser) == 0) &&
		(w.ToParty == nil || len(w.ToParty) == 0) &&
		(w.ToTag == nil || len(w.ToTag) == 0) {
		return fmt.Errorf("must specify one of: `toUser`, `toParty` or `toTag`")
	}

	if w.TmplType != "" && w.TmplType != Text && w.TmplType != Markdown {
		return fmt.Errorf("wechat tmplType must be one of: `text` or `markdown`")
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
		ToUser:   w.ToUser,
		ToParty:  w.ToParty,
		ToTag:    w.ToTag,
		TmplType: w.TmplType,
		Template: w.Template,
		Selector: w.Selector,
	}
}

type Pushover struct {
	Template string
	// Profiles are users to send notifications to.
	Profiles       []*v2beta2.PushoverUserProfile
	PushoverConfig *PushoverConfig
	Selector       *metav1.LabelSelector
	userKeyRegex   *regexp.Regexp // for an User Key's validation
	*common
}

type PushoverConfig struct {
	// The token of a Pushover application.
	Token *v2beta2.Credential
}

func NewPushoverReceiver(c *Config, pr *v2beta2.PushoverReceiver) Receiver {
	p := &Pushover{
		common: &common{
			enabled:        pr.Enabled,
			receiverType:   pushover,
			configSelector: pr.PushoverConfigSelector,
		},
		Profiles: pr.Profiles,
		Selector: pr.AlertSelector,
		// User keys are 30 characters long, case-sensitive, and may contain the character set [A-Za-z0-9].
		userKeyRegex: regexp.MustCompile(`^[A-Za-z0-9]{30}$`),
	}

	if pr.Template != nil {
		p.Template = *pr.Template
	}

	configs := listConfigs(c, pr.PushoverConfigSelector)
	if configs == nil {
		return p
	}

	for _, item := range configs {
		p.generateConfig(item.Spec.Pushover)
		if p.PushoverConfig != nil {
			break
		}
	}

	return p
}

func NewPushoverConfig(sc *v2beta2.PushoverConfig) Receiver {
	p := &Pushover{
		common: &common{
			receiverType: pushover,
		},
	}

	p.generateConfig(sc)
	return p
}

func (p *Pushover) generateConfig(sc *v2beta2.PushoverConfig) {

	if sc == nil || sc.PushoverTokenSecret == nil {
		return
	}

	p.PushoverConfig = &PushoverConfig{
		Token: sc.PushoverTokenSecret,
	}

	return
}

func (p *Pushover) GetConfig() interface{} {
	return p.PushoverConfig
}

func (p *Pushover) SetConfig(obj interface{}) error {

	if obj == nil {
		p.PushoverConfig = nil
		return nil
	}

	c, ok := obj.(*PushoverConfig)
	if !ok {
		return errors.New("set pushover config error, wrong config type")
	}

	p.PushoverConfig = c
	return nil
}

func (p *Pushover) GetAlertSelector() *metav1.LabelSelector {
	return p.Selector
}

func (p *Pushover) Validate() error {

	if p.Profiles == nil || len(p.Profiles) == 0 {
		return fmt.Errorf("user profiles must be specified")
	}

	// validate user keys with regex
	for _, profile := range p.Profiles {
		if profile.UserKey == nil || !p.userKeyRegex.MatchString(*profile.UserKey) {
			return fmt.Errorf("invalid user key： %s", *profile.UserKey)
		}
	}

	if p.PushoverConfig == nil {
		return fmt.Errorf("config is nil")
	}

	if err := validateCredential(p.PushoverConfig.Token); err != nil {
		return fmt.Errorf("pushover token error, %s", err.Error())
	}

	return nil
}

func listConfigs(c *Config, selector *metav1.LabelSelector) []v2beta2.Config {
	configList := v2beta2.ConfigList{}
	configSel, _ := metav1.LabelSelectorAsSelector(selector)
	if err := c.cache.List(c.ctx, &configList, client.MatchingLabelsSelector{Selector: configSel}); client.IgnoreNotFound(err) != nil {
		_ = level.Error(c.logger).Log("msg", "Unable to list Config", "err", err)
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

type Sms struct {
	Template     string
	PhoneNumbers []string
	SmsConfig    *SmsConfig
	Selector     *metav1.LabelSelector
	*common
}

type SmsConfig struct {
	// The default sms provider
	// optional, if not given, use the first available ones.
	DefaultProvider string `json:"defaultProvider,omitempty"`
	// All sms providers
	Providers *v2beta2.Providers `json:"providers"`
}

func NewSmsConfig(sc *v2beta2.SmsConfig) Receiver {
	s := &Sms{
		common: &common{
			receiverType: sms,
		},
	}

	s.generateConfig(sc)
	return s
}

func (s *Sms) generateConfig(sc *v2beta2.SmsConfig) {

	if s == nil {
		return
	}

	s.SmsConfig = &SmsConfig{
		DefaultProvider: sc.DefaultProvider,
		Providers:       sc.Providers,
	}
}

func (s *Sms) GetConfig() interface{} {
	return s.SmsConfig
}

func (s *Sms) SetConfig(obj interface{}) error {

	if obj == nil {
		s.SmsConfig = nil
		return nil
	}

	c, ok := obj.(*SmsConfig)
	if !ok {
		return errors.New("set sms config error, wrong config type")
	}

	s.SmsConfig = c
	return nil
}

func (s *Sms) GetAlertSelector() *metav1.LabelSelector {
	return s.Selector
}

func (s *Sms) Validate() error {
	if len(s.PhoneNumbers) == 0 {
		return fmt.Errorf("`phoneNumbers` must not be empty")
	}

	for _, phoneNumber := range s.PhoneNumbers {
		if verifyPhoneFormat(phoneNumber) {
			return fmt.Errorf("phoneNumber:%s is not a valid phone number, pls check it", phoneNumber)
		}
	}

	providers := s.SmsConfig.Providers
	defaultProvider := s.SmsConfig.DefaultProvider
	if defaultProvider == Aliyun && providers.Aliyun == nil {
		return errors.New("cannot find default provider:aliyun from providers")
	}
	if defaultProvider == Tencent && providers.Tencent == nil {
		return errors.New("cannot find default provider:tencent from providers")
	}

	// Sms aliyun provider parameters validation
	if providers.Aliyun != nil {
		if providers.Aliyun.AccessKeyId != nil {
			if err := validateCredential(providers.Aliyun.AccessKeyId); err != nil {
				return fmt.Errorf("aliyun provider parameters:accessKeyId error, %s", err.Error())
			}
		}
		if providers.Aliyun.AccessKeySecret != nil {
			if err := validateCredential(providers.Aliyun.AccessKeySecret); err != nil {
				return fmt.Errorf("aliyun provider parameters:accessKeySecret error, %s", err.Error())
			}
		}
	}

	// Sms tencent provider parameters validation
	if providers.Tencent != nil {
		if providers.Tencent.SecretId != nil {
			if err := validateCredential(providers.Tencent.SecretId); err != nil {
				return fmt.Errorf("tencent provider parameters:secretId error, %s", err.Error())
			}
		}
		if providers.Tencent.SecretKey != nil {
			if err := validateCredential(providers.Tencent.SecretKey); err != nil {
				return fmt.Errorf("tencent provider parameters:secretKey error, %s", err.Error())
			}
		}
	}

	// Sms huawei provider parameters validation
	if providers.Huawei != nil {
		if providers.Huawei.AppKey != nil {
			if err := validateCredential(providers.Huawei.AppKey); err != nil {
				return fmt.Errorf("huawei provider parameters:appkey error, %s", err.Error())
			}
		}
		if providers.Huawei.AppSecret != nil {
			if err := validateCredential(providers.Huawei.AppSecret); err != nil {
				return fmt.Errorf("huawei provider parameters:appSecret error, %s", err.Error())
			}
		}
	}

	return nil
}

func NewSmsReceiver(c *Config, sr *v2beta2.SmsReceiver) Receiver {
	s := &Sms{
		common: &common{
			enabled:        sr.Enabled,
			receiverType:   sms,
			configSelector: sr.SmsConfigSelector,
		},
		PhoneNumbers: sr.PhoneNumbers,
		Selector:     sr.AlertSelector,
	}

	if sr.Template != nil {
		s.Template = *sr.Template
	}

	if sr.SmsConfigSelector == nil {
		sr.SmsConfigSelector = &metav1.LabelSelector{
			MatchLabels: map[string]string{"type": "default"},
		}
	}

	configs := listConfigs(c, sr.SmsConfigSelector)

	for _, item := range configs {
		s.generateConfig(item.Spec.Sms)
		if s.SmsConfig != nil {
			break
		}
	}

	return s
}

func verifyPhoneFormat(phoneNumber string) bool {
	regular := `(\+)?[\d*\s*]+`

	reg := regexp.MustCompile(regular)
	return reg.MatchString(phoneNumber)
}
