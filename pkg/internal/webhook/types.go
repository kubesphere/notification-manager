package webhook

import (
	"fmt"

	"github.com/kubesphere/notification-manager/pkg/apis/v2beta2"
	"github.com/kubesphere/notification-manager/pkg/constants"
	"github.com/kubesphere/notification-manager/pkg/internal"
)

type Receiver struct {
	*internal.Common
	// `url` gives the location of the webhook, in standard URL form.
	URL        string                    `json:"url,omitempty"`
	HttpConfig *v2beta2.HTTPClientConfig `json:"httpConfig,omitempty"`
	*Config
}

func NewReceiver(tenantID string, obj *v2beta2.Receiver) internal.Receiver {
	if obj.Spec.Webhook == nil {
		return nil
	}
	w := obj.Spec.Webhook
	r := &Receiver{
		Common: &internal.Common{
			Name:          obj.Name,
			TenantID:      tenantID,
			Type:          constants.Webhook,
			Labels:        obj.Labels,
			Enable:        w.Enabled,
			AlertSelector: w.AlertSelector,
		},
		HttpConfig: w.HTTPConfig,
	}

	if w.Template != nil {
		r.Template = *w.Template
	}

	if w.URL != nil {
		r.URL = *w.URL
	} else if w.Service != nil {
		service := w.Service
		if service.Scheme == nil || len(*service.Scheme) == 0 {
			r.URL = fmt.Sprintf("http://%s.%s", service.Name, service.Namespace)
		} else {
			r.URL = fmt.Sprintf("%s://%s.%s", *service.Scheme, service.Name, service.Namespace)
		}

		if service.Port != nil {
			r.URL = fmt.Sprintf("%s:%d/", r.URL, *service.Port)
		}

		if service.Path != nil {
			r.URL = fmt.Sprintf("%s%s", r.URL, *service.Path)
		}
	}

	return r
}

func (r *Receiver) SetConfig(_ internal.Config) {
	return
}

func (r *Receiver) Validate() error {

	if len(r.URL) == 0 {
		return fmt.Errorf("webhook rceiver: url is nil")
	}

	return nil
}

func (r *Receiver) Clone() internal.Receiver {

	return &Receiver{
		Common:     r.Common,
		URL:        r.URL,
		HttpConfig: r.HttpConfig,
		Config:     r.Config,
	}
}

type Config struct {
	*internal.Common
}

func NewConfig(_ *v2beta2.Config) internal.Config {
	return nil
}

func (c *Config) Validate() error {
	return nil
}

func (c *Config) Clone() internal.Config {

	return nil
}
