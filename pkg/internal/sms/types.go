package sms

import (
	"errors"
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
	PhoneNumbers []string `json:"phoneNumbers,omitempty"`
	*Config
}

func NewReceiver(tenantID string, obj *v2beta2.Receiver) internal.Receiver {
	if obj.Spec.Sms == nil {
		return nil
	}
	s := obj.Spec.Sms
	r := &Receiver{
		Common: &internal.Common{
			Name:           obj.Name,
			TenantID:       tenantID,
			Type:           constants.SMS,
			Labels:         obj.Labels,
			Enable:         s.Enabled,
			AlertSelector:  s.AlertSelector,
			ConfigSelector: s.SmsConfigSelector,
			Template: internal.Template{
				TmplText: s.TmplText,
			},
		},
		PhoneNumbers: s.PhoneNumbers,
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
	if len(r.PhoneNumbers) == 0 {
		return fmt.Errorf("sms receiver: `phoneNumbers` must not be empty")
	}

	for _, phoneNumber := range r.PhoneNumbers {
		if verifyPhoneFormat(phoneNumber) {
			return fmt.Errorf("sms receiver: %s is not a valid phone number", phoneNumber)
		}
	}

	if r.Config == nil {
		return fmt.Errorf("sms Receiver: Config is nil")
	}

	return nil
}

func (r *Receiver) Clone() internal.Receiver {

	return &Receiver{
		Common:       r.Common.Clone(),
		PhoneNumbers: r.PhoneNumbers,
		Config:       r.Config,
	}
}

type Config struct {
	*internal.Common
	// The default sms provider
	// optional, if not given, use the first available ones.
	DefaultProvider string `json:"defaultProvider,omitempty"`
	// All sms providers
	Providers *v2beta2.Providers `json:"providers"`
}

func NewConfig(obj *v2beta2.Config) internal.Config {

	if obj.Spec.Sms == nil {
		return nil
	}

	c := &Config{
		Common: &internal.Common{
			Name:   obj.Name,
			Labels: obj.Labels,
			Type:   constants.SMS,
		},
		Providers:       obj.Spec.Sms.Providers,
		DefaultProvider: obj.Spec.Sms.DefaultProvider,
	}

	c.ResourceVersion, _ = strconv.ParseUint(obj.ResourceVersion, 10, 64)

	return c
}

func (c *Config) Validate() error {

	providers := c.Providers
	defaultProvider := c.DefaultProvider
	if defaultProvider == constants.Aliyun && providers.Aliyun == nil {
		return errors.New("sms config: cannot find default provider:aliyun from providers")
	}
	if defaultProvider == constants.Tencent && providers.Tencent == nil {
		return errors.New("sms config: cannot find default provider:tencent from providers")
	}

	// Sms aliyun provider parameters validation
	if providers.Aliyun != nil {
		if providers.Aliyun.AccessKeyId != nil {
			if err := internal.ValidateCredential(providers.Aliyun.AccessKeyId); err != nil {
				return fmt.Errorf("sms config: accessKeyId error, %s", err.Error())
			}
		}
		if providers.Aliyun.AccessKeySecret != nil {
			if err := internal.ValidateCredential(providers.Aliyun.AccessKeySecret); err != nil {
				return fmt.Errorf("sms config: accessKeySecret error, %s", err.Error())
			}
		}
	}

	// Sms tencent provider parameters validation
	if providers.Tencent != nil {
		if providers.Tencent.SecretId != nil {
			if err := internal.ValidateCredential(providers.Tencent.SecretId); err != nil {
				return fmt.Errorf("sms config: secretId error, %s", err.Error())
			}
		}
		if providers.Tencent.SecretKey != nil {
			if err := internal.ValidateCredential(providers.Tencent.SecretKey); err != nil {
				return fmt.Errorf("sms config: secretKey error, %s", err.Error())
			}
		}
	}

	// Sms huawei provider parameters validation
	if providers.Huawei != nil {
		if providers.Huawei.AppKey != nil {
			if err := internal.ValidateCredential(providers.Huawei.AppKey); err != nil {
				return fmt.Errorf("sms config: appkey error, %s", err.Error())
			}
		}
		if providers.Huawei.AppSecret != nil {
			if err := internal.ValidateCredential(providers.Huawei.AppSecret); err != nil {
				return fmt.Errorf("sms config: appSecret error, %s", err.Error())
			}
		}
	}

	return nil
}

func (c *Config) Clone() internal.Config {

	return &Config{
		Common:          c.Common.Clone(),
		DefaultProvider: c.DefaultProvider,
		Providers:       c.Providers,
	}
}

func verifyPhoneFormat(phoneNumber string) bool {
	regular := `(\+)?[\d*\s*]+`

	reg := regexp.MustCompile(regular)
	return reg.MatchString(phoneNumber)
}
