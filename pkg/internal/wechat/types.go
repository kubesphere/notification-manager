package wechat

import (
	"fmt"

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
	*Config
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
		},
		ToUser:  w.ToUser,
		ToParty: w.ToParty,
		ToTag:   w.ToTag,
	}

	if w.Template != nil {
		r.Template = *w.Template
	}

	if w.TmplType != nil {
		r.TmplType = *w.TmplType
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

	if (r.ToUser == nil || len(r.ToUser) == 0) &&
		(r.ToParty == nil || len(r.ToParty) == 0) &&
		(r.ToTag == nil || len(r.ToTag) == 0) {
		return fmt.Errorf("wechat receiver: must specify one of: `toUser`, `toParty` or `toTag`")
	}

	if r.TmplType != "" && r.TmplType != constants.Text && r.TmplType != constants.Markdown {
		return fmt.Errorf("wechat receiver: tmplType must be one of: `text` or `markdown`")
	}

	if r.Config == nil {
		return fmt.Errorf("wechat receiver: config is nil")
	}

	return nil
}

func (r *Receiver) Clone() internal.Receiver {

	return &Receiver{
		Common:  r.Common,
		Config:  r.Config,
		ToUser:  r.ToUser,
		ToParty: r.ToParty,
		ToTag:   r.ToTag,
	}
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
		Common:    c.Common,
		APISecret: c.APISecret,
		CorpID:    c.CorpID,
		APIURL:    c.APIURL,
		AgentID:   c.AgentID,
	}
}
