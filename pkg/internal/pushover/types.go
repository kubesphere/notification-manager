package pushover

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/modern-go/reflect2"

	"github.com/kubesphere/notification-manager/pkg/apis/v2beta2"
	"github.com/kubesphere/notification-manager/pkg/constants"
	"github.com/kubesphere/notification-manager/pkg/internal"
)

type Receiver struct {
	*internal.Common
	// Profiles are users to send notifications to.
	Profiles []*v2beta2.PushoverUserProfile `json:"profiles,omitempty"`
	// for a User Key's validation
	userKeyRegex *regexp.Regexp

	*Config
}

func NewReceiver(tenantID string, obj *v2beta2.Receiver) internal.Receiver {
	if obj.Spec.Pushover == nil {
		return nil
	}

	p := obj.Spec.Pushover
	r := &Receiver{
		Common: &internal.Common{
			Name:           obj.Name,
			TenantID:       tenantID,
			Type:           constants.Pushover,
			Labels:         obj.Labels,
			Enable:         p.Enabled,
			AlertSelector:  p.AlertSelector,
			ConfigSelector: p.PushoverConfigSelector,
			Template: internal.Template{
				TmplText: p.TmplText,
			},
		},
		Profiles: p.Profiles,
		// User keys are 30 characters long, case-sensitive, and may contain the character set [A-Za-z0-9].
		userKeyRegex: regexp.MustCompile(`^[A-Za-z0-9]{30}$`),
	}

	r.ResourceVersion, _ = strconv.ParseUint(obj.ResourceVersion, 10, 64)

	if p.Template != nil {
		r.TmplName = *p.Template
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

	if len(r.Profiles) == 0 {
		return fmt.Errorf("pushover receiver: user profiles must be specified")
	}

	// validate user keys with regex
	for _, profile := range r.Profiles {
		if profile.UserKey == nil || !r.userKeyRegex.MatchString(*profile.UserKey) {
			return fmt.Errorf("pushover receiver: invalid user key? %s", *profile.UserKey)
		}
	}

	if r.Config == nil {
		return fmt.Errorf("pushover receiver: config is nil")
	}

	return nil
}

func (r *Receiver) Clone() internal.Receiver {

	return &Receiver{
		Common:       r.Common.Clone(),
		Profiles:     r.Profiles,
		userKeyRegex: r.userKeyRegex,
		Config:       r.Config,
	}
}

type Config struct {
	*internal.Common
	// The token of a Pushover application.
	Token *v2beta2.Credential `json:"token,omitempty"`
}

func NewConfig(obj *v2beta2.Config) internal.Config {

	if obj.Spec.Pushover == nil {
		return nil
	}

	c := &Config{
		Common: &internal.Common{
			Name:   obj.Name,
			Labels: obj.Labels,
			Type:   constants.Pushover,
		},
		Token: obj.Spec.Pushover.PushoverTokenSecret,
	}

	c.ResourceVersion, _ = strconv.ParseUint(obj.ResourceVersion, 10, 64)

	return c
}

func (c *Config) Validate() error {

	if err := internal.ValidateCredential(c.Token); err != nil {
		return fmt.Errorf("pushover config: token error, %s", err.Error())
	}

	return nil
}

func (c *Config) Clone() internal.Config {

	return &Config{
		Common: c.Common.Clone(),
		Token:  c.Token,
	}
}
