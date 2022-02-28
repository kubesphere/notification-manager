package internal

import (
	"fmt"
	"math"
	"strconv"

	"github.com/kubesphere/notification-manager/pkg/apis/v2beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	priority = "priority"
)

type Common struct {
	Name           string                `json:"name,omitempty"`
	Type           string                `json:"type,omitempty"`
	TenantID       string                `json:"tenantID,omitempty"`
	Labels         map[string]string     `json:"labels,omitempty"`
	Enable         *bool                 `json:"enable,omitempty"`
	AlertSelector  *metav1.LabelSelector `json:"alertSelector,omitempty"`
	ConfigSelector *metav1.LabelSelector `json:"configSelector,omitempty"`
	Template       string                `json:"template,omitempty"`
	TitleTemplate  string                `json:"titleTemplate,omitempty"`
	TmplType       string                `json:"tmplType,omitempty"`
	Hash           string                `json:"hash,omitempty"`
}

func (c *Common) GetName() string {
	return c.Name
}

func (c *Common) GetTenantID() string {
	return c.TenantID
}

func (c *Common) Enabled() bool {
	if c.Enable == nil {
		return true
	}
	return *c.Enable
}

func (c *Common) GetLabels() map[string]string {
	return c.Labels
}

func (c *Common) GetPriority() int {
	if c.Labels == nil {
		return math.MaxInt32 - 1
	}

	str := c.Labels[priority]
	p, err := strconv.Atoi(str)
	if err != nil {
		return math.MaxInt32 - 1
	}

	return p
}

func (c *Common) GetType() string {
	return c.Type
}

func (c *Common) GetAlertSelector() *metav1.LabelSelector {
	return c.AlertSelector
}

func (c *Common) GetConfigSelector() *metav1.LabelSelector {
	return c.ConfigSelector
}

func (c *Common) SetHash(h string) {
	c.Hash = h
}

func (c *Common) GetHash() string {
	return c.Hash
}

func ValidateCredential(c *v2beta2.Credential) error {

	if c == nil {
		return fmt.Errorf("%s", "Credential is nil")
	}

	if len(c.Value) == 0 && c.ValueFrom == nil {
		return fmt.Errorf("%s", "must specify one of: `value` or `valueFrom`")
	}

	if len(c.Value) != 0 && c.ValueFrom != nil {
		return fmt.Errorf("valueFrom may not be specified when `value` is not empty")
	}

	if c.ValueFrom != nil {
		if c.ValueFrom.SecretKeyRef == nil {
			return fmt.Errorf("secretKeyRef must be specified when valueFrom is not nil")
		}

		if len(c.ValueFrom.SecretKeyRef.Key) == 0 {
			return fmt.Errorf("key must be specified when secretKeyRef is not nil")
		}

		if len(c.ValueFrom.SecretKeyRef.Name) == 0 {
			return fmt.Errorf("name must be specified when secretKeyRef is not nil")
		}
	}

	return nil
}
