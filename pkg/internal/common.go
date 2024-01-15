package internal

import (
	"fmt"
	"math"
	"strconv"

	"github.com/kubesphere/notification-manager/apis/v2beta2"
)

const (
	priority = "priority"
)

type Template struct {
	TmplName      string                        `json:"tmplName,omitempty"`
	TitleTmplName string                        `json:"titleTmplName,omitempty"`
	TmplType      string                        `json:"tmplType,omitempty"`
	TmplText      *v2beta2.ConfigmapKeySelector `json:"tmplText,omitempty"`
}

type Common struct {
	Name            string                 `json:"name,omitempty"`
	ResourceVersion uint64                 `json:"resourceVersion,omitempty"`
	Type            string                 `json:"type,omitempty"`
	TenantID        string                 `json:"tenantID,omitempty"`
	Labels          map[string]string      `json:"labels,omitempty"`
	Enable          *bool                  `json:"enable,omitempty"`
	AlertSelector   *v2beta2.LabelSelector `json:"alertSelector,omitempty"`
	ConfigSelector  *v2beta2.LabelSelector `json:"configSelector,omitempty"`
	Hash            string                 `json:"hash,omitempty"`
	Template        `json:"template,omitempty"`
}

func (c *Common) GetName() string {
	return c.Name
}

func (c *Common) GetResourceVersion() uint64 {
	return c.ResourceVersion
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

func (c *Common) GetAlertSelector() *v2beta2.LabelSelector {
	return c.AlertSelector
}

func (c *Common) GetConfigSelector() *v2beta2.LabelSelector {
	return c.ConfigSelector
}

func (c *Common) SetHash(h string) {
	c.Hash = h
}

func (c *Common) GetHash() string {
	return c.Hash
}

func (c *Common) Clone() *Common {

	return &Common{
		Name:           c.Name,
		Type:           c.Type,
		TenantID:       c.TenantID,
		Labels:         c.Labels,
		Enable:         c.Enable,
		AlertSelector:  c.AlertSelector,
		ConfigSelector: c.ConfigSelector,
		Hash:           c.Hash,
		Template: Template{
			TmplName:      c.TmplName,
			TitleTmplName: c.TitleTmplName,
			TmplType:      c.TmplType,
			TmplText:      c.TmplText,
		},
	}
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
