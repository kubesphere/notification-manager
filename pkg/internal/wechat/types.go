package wechat

import (
	"fmt"
	"strconv"

	"github.com/kubesphere/notification-manager/pkg/apis/v2beta2"
	"github.com/kubesphere/notification-manager/pkg/constants"
	"github.com/kubesphere/notification-manager/pkg/internal"
	"github.com/modern-go/reflect2"
)

type Receiver struct {
	*internal.Common
	ToUser  []string `json:"toUser,omitempty"`
	ToParty []string `json:"toParty,omitempty"`
	ToTag   []string `json:"toTag,omitempty"`
	ChatBot *ChatBot `json:"chatbot,omitempty"`
	*Config
}

type ChatBot struct {
	Webhook   *v2beta2.Credential `json:"webhook"`
	AtUsers   []string            `json:"atUsers,omitempty"`
	AtMobiles []string            `json:"atMobiles,omitempty"`
}

func NewReceiver(tenantID string, obj *v2beta2.Receiver) internal.Receiver {
	if obj.Spec.Wechat == nil {
		return nil
	}
	w := obj.Spec.Wechat
	r := &Receiver{
		Common: &internal.Common{
			Name:           obj.Name,
			TenantID:       tenantID,
			Type:           constants.WeChat,
			Labels:         obj.Labels,
			Enable:         w.Enabled,
			AlertSelector:  w.AlertSelector,
			ConfigSelector: w.WechatConfigSelector,
			Template: internal.Template{
				TmplText: w.TmplText,
			},
		},
		ToUser:  w.ToUser,
		ToParty: w.ToParty,
		ToTag:   w.ToTag,
	}

	r.ResourceVersion, _ = strconv.ParseUint(obj.ResourceVersion, 10, 64)

	if w.Template != nil {
		r.TmplName = *w.Template
	}

	if w.TmplType != nil {
		r.TmplType = *w.TmplType
	}

	if w.ChatBot != nil {
		r.ChatBot = &ChatBot{
			Webhook:   w.ChatBot.Webhook,
			AtUsers:   w.ChatBot.AtUsers,
			AtMobiles: w.ChatBot.AtMobiles,
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

	if len(r.ToUser) == 0 && len(r.ToParty) == 0 && len(r.ToTag) == 0 && r.ChatBot == nil {
		return fmt.Errorf("wechat receiver: must specify one of: `toUser`, `toParty`, `toTag` or `chatbot`")
	}

	if r.TmplType != "" && r.TmplType != constants.Text && r.TmplType != constants.Markdown {
		return fmt.Errorf("wechat receiver: tmplType must be one of: `text` or `markdown`")
	}

	if len(r.ToUser) > 0 || len(r.ToParty) > 0 || len(r.ToTag) > 0 {
		if r.Config == nil {
			return fmt.Errorf("wechat receiver: config is nil")
		}
	}

	return nil
}

func (r *Receiver) Clone() internal.Receiver {

	out := &Receiver{
		Common:  r.Common.Clone(),
		Config:  r.Config,
		ChatBot: r.ChatBot,
	}

	out.ToParty = append(out.ToParty, r.ToParty...)
	out.ToTag = append(out.ToTag, r.ToTag...)
	out.ToUser = append(out.ToUser, r.ToUser...)

	return out
}

type Config struct {
	*internal.Common
	APISecret *v2beta2.Credential `json:"apiSecret,omitempty"`
	CorpID    string              `json:"corpID,omitempty"`
	APIURL    string              `json:"apiurl,omitempty"`
	AgentID   string              `json:"agentID,omitempty"`
}

func NewConfig(obj *v2beta2.Config) internal.Config {
	if obj.Spec.Wechat == nil {
		return nil
	}
	w := obj.Spec.Wechat
	c := &Config{
		Common: &internal.Common{
			Name:   obj.Name,
			Labels: obj.Labels,
			Type:   constants.WeChat,
		},
		APIURL:    w.WechatApiUrl,
		AgentID:   w.WechatApiAgentId,
		CorpID:    w.WechatApiCorpId,
		APISecret: w.WechatApiSecret,
	}

	c.ResourceVersion, _ = strconv.ParseUint(obj.ResourceVersion, 10, 64)

	return c
}

func (c *Config) Validate() error {

	if len(c.CorpID) == 0 {
		return fmt.Errorf("wechat config: corpid must be specified")
	}

	if len(c.AgentID) == 0 {
		return fmt.Errorf("wechat config: agentid must be specified")
	}

	if err := internal.ValidateCredential(c.APISecret); err != nil {
		return fmt.Errorf("wechat config: apisecret error, %s", err.Error())
	}

	return nil
}

func (c *Config) Clone() internal.Config {

	return &Config{
		Common:    c.Common.Clone(),
		APISecret: c.APISecret,
		CorpID:    c.CorpID,
		APIURL:    c.APIURL,
		AgentID:   c.AgentID,
	}
}
