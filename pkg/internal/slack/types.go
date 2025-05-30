package slack

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
	// The channel or user to send notifications to.
	Channels []string `json:"channels,omitempty"`
	*Config
}

func NewReceiver(tenantID string, obj *v2beta2.Receiver) internal.Receiver {
	if obj.Spec.Slack == nil {
		return nil
	}
	s := obj.Spec.Slack
	r := &Receiver{
		Common: &internal.Common{
			Name:           obj.Name,
			TenantID:       tenantID,
			Type:           constants.Slack,
			Labels:         obj.Labels,
			Enable:         s.Enabled,
			AlertSelector:  s.AlertSelector,
			ConfigSelector: s.SlackConfigSelector,
			Template: internal.Template{
				TmplText: s.TmplText,
			},
		},
		Channels: s.Channels,
	}

	r.ResourceVersion, _ = strconv.ParseUint(obj.ResourceVersion, 10, 64)

	if s.Template != nil {
		r.TmplName = *s.Template
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

	if len(r.Channels) == 0 {
		return fmt.Errorf("slack receiver: channel must be specified")
	}

	if r.Config == nil {
		return fmt.Errorf("slack receiver: Config is nil")
	}

	return nil
}

func (r *Receiver) Clone() internal.Receiver {

	return &Receiver{
		Common:   r.Common.Clone(),
		Channels: r.Channels,
		Config:   r.Config,
	}
}

func (r *Receiver) GetChannels() (string, interface{}) {
	return r.Type, r.Channels
}

type Config struct {
	*internal.Common
	// The token of user or bot.
	Token *v2beta2.Credential `json:"token,omitempty"`
}

func NewConfig(obj *v2beta2.Config) internal.Config {
	if obj.Spec.Slack == nil {
		return nil
	}

	c := &Config{
		Common: &internal.Common{
			Name:   obj.Name,
			Labels: obj.Labels,
			Type:   constants.Slack,
		},
		Token: obj.Spec.Slack.SlackTokenSecret,
	}

	c.ResourceVersion, _ = strconv.ParseUint(obj.ResourceVersion, 10, 64)

	return c
}

func (c *Config) Validate() error {

	if err := internal.ValidateCredential(c.Token); err != nil {
		return fmt.Errorf("slack config: token error, %s", err.Error())
	}

	return nil
}

func (c *Config) Clone() internal.Config {

	return &Config{
		Common: c.Common.Clone(),
		Token:  c.Token,
	}
}
