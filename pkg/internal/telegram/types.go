package telegram

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
	*Config

	// The channel or user to send notifications to.
	Channels       []string `json:"channels,omitempty"`
	MentionedUsers []string `json:"mentionedUsers,omitempty"`
}

func NewReceiver(tenantID string, obj *v2beta2.Receiver) internal.Receiver {
	if obj.Spec.Telegram == nil {
		return nil
	}
	telegram := obj.Spec.Telegram
	r := &Receiver{
		Common: &internal.Common{
			Name:           obj.Name,
			TenantID:       tenantID,
			Type:           constants.Telegram,
			Labels:         obj.Labels,
			Enable:         telegram.Enabled,
			AlertSelector:  telegram.AlertSelector,
			ConfigSelector: telegram.TelegramConfigSelector,
			Template: internal.Template{
				TmplName: *telegram.Template,
				TmplText: telegram.TmplText,
			},
		},
		Channels:       telegram.Channels,
		MentionedUsers: telegram.MentionedUsers,
	}

	r.ResourceVersion, _ = strconv.ParseUint(obj.ResourceVersion, 10, 64)

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
		return fmt.Errorf("telegram receiver: channel must be specified")
	}

	if r.Config == nil {
		return fmt.Errorf("telegram receiver: Config is nil")
	}

	return nil
}

func (r *Receiver) Clone() internal.Receiver {

	return &Receiver{
		Common:         r.Common.Clone(),
		Channels:       r.Channels,
		MentionedUsers: r.MentionedUsers,
		Config:         r.Config,
	}
}

type Config struct {
	*internal.Common
	// The token of user or bot.
	Token *v2beta2.Credential `json:"token,omitempty"`
}

func NewConfig(obj *v2beta2.Config) internal.Config {
	if obj.Spec.Telegram == nil {
		return nil
	}

	c := &Config{
		Common: &internal.Common{
			Name:   obj.Name,
			Labels: obj.Labels,
			Type:   constants.Telegram,
		},
		Token: obj.Spec.Telegram.TelegramTokenSecret,
	}

	c.ResourceVersion, _ = strconv.ParseUint(obj.ResourceVersion, 10, 64)

	return c
}

func (c *Config) Validate() error {

	if err := internal.ValidateCredential(c.Token); err != nil {
		return fmt.Errorf("telegram config: token error, %s", err.Error())
	}

	return nil
}

func (c *Config) Clone() internal.Config {

	return &Config{
		Common: c.Common.Clone(),
		Token:  c.Token,
	}
}
