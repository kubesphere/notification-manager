package feishu

import (
	"fmt"
	"strconv"

	"github.com/kubesphere/notification-manager/apis/v2beta2"
	"github.com/kubesphere/notification-manager/pkg/constants"
	"github.com/kubesphere/notification-manager/pkg/internal"
	"github.com/modern-go/reflect2"
)

type Receiver struct {
	*internal.Common
	User       []string `json:"user,omitempty"`
	Department []string `json:"department,omitempty"`
	ChatBot    *ChatBot `json:"chatbot,omitempty"`
	*Config
}

type ChatBot struct {
	Webhook  *v2beta2.Credential `json:"webhook,omitempty"`
	Keywords []string            `json:"keywords,omitempty"`
	Secret   *v2beta2.Credential `json:"secret,omitempty"`
}

func NewReceiver(tenantID string, obj *v2beta2.Receiver) internal.Receiver {
	if obj.Spec.Feishu == nil {
		return nil
	}
	f := obj.Spec.Feishu
	r := &Receiver{
		Common: &internal.Common{
			Name:           obj.Name,
			TenantID:       tenantID,
			Type:           constants.Feishu,
			Labels:         obj.Labels,
			Enable:         f.Enabled,
			AlertSelector:  f.AlertSelector,
			ConfigSelector: f.FeishuConfigSelector,
			Template: internal.Template{
				TmplText: f.TmplText,
			},
		},
		User:       f.User,
		Department: f.Department,
	}

	r.ResourceVersion, _ = strconv.ParseUint(obj.ResourceVersion, 10, 64)

	if f.Template != nil {
		r.TmplName = *f.Template
	}

	if f.TmplType != nil {
		r.TmplType = *f.TmplType
	}

	if f.ChatBot != nil {
		r.ChatBot = &ChatBot{
			Webhook:  f.ChatBot.Webhook,
			Secret:   f.ChatBot.Secret,
			Keywords: f.ChatBot.Keywords,
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

	if len(r.User) == 0 && len(r.Department) == 0 && r.ChatBot == nil {
		return fmt.Errorf("feishu receiver: must specify one of: `user`, `department` or `chatbot`")
	}

	if r.TmplType != "" && r.TmplType != constants.Text && r.TmplType != constants.Post {
		return fmt.Errorf("feishu Receiver: tmplType must be one of: `text` or `post`")
	}

	if (len(r.User) > 0 || len(r.Department) > 0) && r.Config == nil {
		return fmt.Errorf("feishu receiver: config is nil")
	}

	return nil
}

func (r *Receiver) Clone() internal.Receiver {

	return &Receiver{
		Common:     r.Common.Clone(),
		Config:     r.Config,
		User:       r.User,
		Department: r.Department,
		ChatBot:    r.ChatBot,
	}
}

type Config struct {
	*internal.Common
	AppID     *v2beta2.Credential `json:"appID,omitempty"`
	AppSecret *v2beta2.Credential `json:"appSecret,omitempty"`
}

func NewConfig(obj *v2beta2.Config) internal.Config {
	if obj.Spec.Feishu == nil {
		return nil
	}

	f := obj.Spec.Feishu
	c := &Config{
		Common: &internal.Common{
			Name:   obj.Name,
			Labels: obj.Labels,
			Type:   constants.Feishu,
		},
		AppID:     f.AppID,
		AppSecret: f.AppSecret,
	}

	c.ResourceVersion, _ = strconv.ParseUint(obj.ResourceVersion, 10, 64)

	return c
}

func (c *Config) Validate() error {

	if err := internal.ValidateCredential(c.AppID); err != nil {
		return fmt.Errorf("feishu config: appID error, %s", err.Error())
	}

	if err := internal.ValidateCredential(c.AppSecret); err != nil {
		return fmt.Errorf("feishu config: appSecret error, %s", err.Error())
	}

	return nil
}

func (c *Config) Clone() internal.Config {

	return &Config{
		Common:    c.Common.Clone(),
		AppSecret: c.AppSecret,
		AppID:     c.AppID,
	}
}
