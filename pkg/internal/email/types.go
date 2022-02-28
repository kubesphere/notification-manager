package email

import (
	"fmt"

	"github.com/kubesphere/notification-manager/pkg/apis/v2beta2"
	"github.com/kubesphere/notification-manager/pkg/constants"
	"github.com/kubesphere/notification-manager/pkg/internal"
	"github.com/modern-go/reflect2"
)

type Receiver struct {
	*internal.Common

	To []string `json:"to,omitempty"`

	*Config
}

func NewReceiver(tenantID string, obj *v2beta2.Receiver) internal.Receiver {
	if obj.Spec.Email == nil {
		return nil
	}

	e := obj.Spec.Email
	r := &Receiver{
		Common: &internal.Common{
			Name:           obj.Name,
			TenantID:       tenantID,
			Type:           constants.Email,
			Labels:         obj.Labels,
			Enable:         e.Enabled,
			AlertSelector:  e.AlertSelector,
			ConfigSelector: e.EmailConfigSelector,
		},
		To: e.To,
	}

	if e.Template != nil {
		r.Template = *e.Template
	}

	if e.SubjectTemplate != nil {
		r.TitleTemplate = *e.SubjectTemplate
	}

	if e.TmplType != nil {
		r.TmplType = *e.TmplType
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

	if r.TmplType != "" && r.TmplType != constants.Text && r.TmplType != constants.HTML {
		return fmt.Errorf("email receiver: tmplType must be one of: `text` or `html`")
	}

	if len(r.To) == 0 {
		return fmt.Errorf("email receiver: receivers is empty")
	}

	if r.Config == nil {
		return fmt.Errorf("email receiver: config is nil")
	}

	return nil
}

func (r *Receiver) Clone() internal.Receiver {

	return &Receiver{
		Common: r.Common,
		To:     r.To,
		Config: r.Config,
	}
}

type Config struct {
	*internal.Common
	From         string              `json:"from,omitempty"`
	SmartHost    v2beta2.HostPort    `json:"smartHost,omitempty"`
	Hello        string              `json:"hello,omitempty"`
	AuthUsername string              `json:"authUsername,omitempty"`
	AuthIdentify string              `json:"authIdentify,omitempty"`
	AuthPassword *v2beta2.Credential `json:"authPassword,omitempty"`
	AuthSecret   *v2beta2.Credential `json:"authSecret,omitempty"`
	RequireTLS   bool                `json:"requireTLS,omitempty"`
	TLS          *v2beta2.TLSConfig  `json:"tls,omitempty"`
}

func NewConfig(obj *v2beta2.Config) internal.Config {
	if obj.Spec.Email == nil {
		return nil
	}

	e := obj.Spec.Email
	c := &Config{
		Common: &internal.Common{
			Name:   obj.Name,
			Labels: obj.Labels,
			Type:   constants.Email,
		},
		From:         e.From,
		SmartHost:    e.SmartHost,
		AuthPassword: e.AuthPassword,
		AuthSecret:   e.AuthSecret,
	}

	if e.Hello != nil {
		c.Hello = *e.Hello
	}

	if e.AuthIdentify != nil {
		c.AuthIdentify = *e.AuthIdentify
	}

	if e.RequireTLS != nil {
		c.RequireTLS = *e.RequireTLS
	}

	if e.TLS != nil {
		c.TLS = e.TLS
	}

	if e.AuthUsername != nil {
		c.AuthUsername = *e.AuthUsername
	}

	return c
}

func (c *Config) Validate() error {
	return nil
}

func (c *Config) Clone() internal.Config {

	return &Config{
		Common:       c.Common,
		From:         c.From,
		SmartHost:    c.SmartHost,
		Hello:        c.Hello,
		AuthUsername: c.AuthUsername,
		AuthIdentify: c.AuthIdentify,
		AuthPassword: c.AuthPassword,
		AuthSecret:   c.AuthSecret,
		RequireTLS:   c.RequireTLS,
		TLS:          c.TLS,
	}
}
