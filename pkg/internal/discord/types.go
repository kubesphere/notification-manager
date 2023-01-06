package discord

import (
	"fmt"
	"github.com/kubesphere/notification-manager/pkg/apis/v2beta2"
	"github.com/kubesphere/notification-manager/pkg/constants"
	"github.com/kubesphere/notification-manager/pkg/internal"
)

type Receiver struct {
	*internal.Common
	Webhook        *v2beta2.Credential `json:"webhook"`
	Type           *string             `json:"type,omitempty"`
	MentionedUsers []string            `json:"mentionedUsers,omitempty"`
	MentionedRoles []string            `json:"mentionedRoles,omitempty"`
}

func NewReceiver(tenantID string, obj *v2beta2.Receiver) internal.Receiver {
	if obj.Spec.Discord == nil {
		return nil
	}
	discord := obj.Spec.Discord
	r := &Receiver{
		Common: &internal.Common{
			Name:          obj.Name,
			TenantID:      tenantID,
			Type:          constants.Discord,
			Labels:        obj.Labels,
			Enable:        discord.Enabled,
			AlertSelector: discord.AlertSelector,
			Template: internal.Template{
				TmplName: *discord.Template,
				TmplText: discord.TmplText,
			},
		},
		MentionedRoles: discord.MentionedRoles,
		MentionedUsers: discord.MentionedUsers,
		Type:           discord.Type,
	}

	if discord.Webhook != nil {
		r.Webhook = discord.Webhook
	}

	return r
}

func (r *Receiver) SetConfig(c internal.Config) {
	return
}

func (r *Receiver) Validate() error {
	if r.Type != nil {
		if *r.Type != constants.DiscordContent && *r.Type != constants.DiscordEmbed {
			return fmt.Errorf("discord receiver: type must be one of: `content` or `embed`")
		}
	}
	return nil
}

func (r *Receiver) Clone() internal.Receiver {

	out := &Receiver{
		Common:  r.Common.Clone(),
		Webhook: r.Webhook,
	}

	out.Type = r.Type
	out.MentionedUsers = append(out.MentionedUsers, r.MentionedUsers...)
	out.MentionedRoles = append(out.MentionedRoles, r.MentionedRoles...)
	return out
}

type Config struct {
	*internal.Common
}

func NewConfig(obj *v2beta2.Config) internal.Config {
	return nil
}

func (c *Config) Validate() error {
	return nil
}

func (c *Config) Clone() internal.Config {
	return nil
}
