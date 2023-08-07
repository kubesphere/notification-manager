package internal

import (
	"github.com/kubesphere/notification-manager/pkg/apis/v2beta2"
)

type Receiver interface {
	GetTenantID() string
	GetName() string
	GetResourceVersion() uint64
	Enabled() bool
	GetType() string
	GetLabels() map[string]string
	GetAlertSelector() *v2beta2.LabelSelector
	GetConfigSelector() *v2beta2.LabelSelector
	SetConfig(c Config)
	Validate() error
	Clone() Receiver
	GetHash() string
	SetHash(h string)
}

type Config interface {
	GetResourceVersion() uint64
	GetLabels() map[string]string
	GetPriority() int
	Validate() error
	Clone() Config
}
