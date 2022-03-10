package dingtalk

import (
	"fmt"

	"github.com/kubesphere/notification-manager/pkg/constants"
	"github.com/kubesphere/notification-manager/pkg/internal"
	"github.com/modern-go/reflect2"

	"github.com/kubesphere/notification-manager/pkg/apis/v2beta2"
)

type Receiver struct {
	*internal.Common

	ChatIDs []string `json:"chatIDs,omitempty"`
	ChatBot *ChatBot `json:"chatBot,omitempty"`

	*Config
}

type ChatBot struct {
	Webhook   *v2beta2.Credential `json:"webhook,omitempty"`
	Keywords  []string            `json:"keywords,omitempty"`
	Secret    *v2beta2.Credential `json:"secret,omitempty"`
	AtMobiles []string            `json:"atMobiles,omitempty"`
	AtUsers   []string            `json:"atUsers,omitempty"`
	AtAll     bool                `json:"atAll,omitempty"`
}

func NewReceiver(tenantID string, obj *v2beta2.Receiver) internal.Receiver {

	if obj.Spec.DingTalk == nil {
		return nil
	}

	dingtalk := obj.Spec.DingTalk

	r := &Receiver{
		Common: &internal.Common{
			Name:           obj.Name,
			TenantID:       tenantID,
			Type:           constants.DingTalk,
			Labels:         obj.Labels,
			Enable:         dingtalk.Enabled,
			AlertSelector:  dingtalk.AlertSelector,
			ConfigSelector: dingtalk.DingTalkConfigSelector,
			Template: internal.Template{
				TmplText: dingtalk.TmplText,
			},
		},
	}

	if dingtalk.Template != nil {
		r.TmplName = *dingtalk.Template
	}

	if dingtalk.TitleTemplate != nil {
		r.TitleTmplName = *dingtalk.TitleTemplate
	}

	if dingtalk.TmplType != nil {
		r.TmplType = *dingtalk.TmplType
	}

	if dingtalk.Conversation != nil {
		r.ChatIDs = dingtalk.Conversation.ChatIDs
	}

	if dingtalk.ChatBot != nil {
		r.ChatBot = &ChatBot{
			Webhook:   dingtalk.ChatBot.Webhook,
			Keywords:  dingtalk.ChatBot.Keywords,
			Secret:    dingtalk.ChatBot.Secret,
			AtMobiles: dingtalk.ChatBot.AtMobiles,
			AtUsers:   dingtalk.ChatBot.AtUsers,
			AtAll:     dingtalk.ChatBot.AtAll,
		}
	}

	return r
}

func (r *Receiver) SetConfig(c internal.Config) {
	if reflect2.IsNil(c) {
		return
	}

	if nc, ok := c.(*Config); ok {
		r.Config = nc
	}
}

func (r *Receiver) Validate() error {

	if r.TmplType != "" && r.TmplType != constants.Text && r.TmplType != constants.Markdown {
		return fmt.Errorf("DingTalk Receiver: tmplType must be one of: `text` or `markdown`")
	}

	if r.ChatBot == nil && len(r.ChatIDs) == 0 {
		return fmt.Errorf("%s", "DingTalk Receiver: must specify one of: `chatbot` or `chatIDs`")
	}

	if r.ChatBot != nil {
		if err := internal.ValidateCredential(r.ChatBot.Webhook); err != nil {
			return fmt.Errorf("DingTalk Receiver: chatbot webhook error, %s", err.Error())
		}

		if r.ChatBot.Secret != nil {
			if err := internal.ValidateCredential(r.ChatBot.Secret); err != nil {
				return fmt.Errorf("DingTalk Receiver: chatbot secret error, %s", err.Error())
			}
		}
	}

	if len(r.ChatIDs) > 0 {
		if r.Config == nil {
			return fmt.Errorf("DingTalk Receiver: config is nil")
		}
	}

	return nil
}

func (r *Receiver) Clone() internal.Receiver {

	return &Receiver{
		Common:  r.Common,
		ChatIDs: r.ChatIDs,
		ChatBot: r.ChatBot,
		Config:  r.Config,
	}
}

type Config struct {
	*internal.Common
	AppKey    *v2beta2.Credential `json:"appKey,omitempty"`
	AppSecret *v2beta2.Credential `json:"appSecret,omitempty"`
}

func NewConfig(obj *v2beta2.Config) internal.Config {

	if obj.Spec.DingTalk == nil {
		return nil
	}

	dingtalk := obj.Spec.DingTalk

	c := &Config{
		Common: &internal.Common{
			Name:   obj.Name,
			Type:   constants.DingTalk,
			Labels: obj.Labels,
		},
	}

	if dingtalk.Conversation != nil {
		c.AppKey = dingtalk.Conversation.AppKey
		c.AppSecret = dingtalk.Conversation.AppSecret
	}

	return c
}

func (c *Config) Validate() error {

	if c.AppKey != nil {
		if err := internal.ValidateCredential(c.AppKey); err != nil {
			return fmt.Errorf("DingTalk Config: appkey error, %s", err.Error())
		}
	}

	if c.AppKey != nil {
		if err := internal.ValidateCredential(c.AppSecret); err != nil {
			return fmt.Errorf("DingTalk Config: appsecret error, %s", err.Error())
		}
	}

	return nil
}

func (c *Config) Clone() internal.Config {

	return &Config{
		Common:    c.Common,
		AppKey:    c.AppKey,
		AppSecret: c.AppSecret,
	}
}
