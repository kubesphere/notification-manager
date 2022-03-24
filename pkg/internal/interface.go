package internal

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type Receiver interface {
	GetTenantID() string
	GetName() string
	GetResourceVersion() uint64
	Enabled() bool
	GetType() string
	GetLabels() map[string]string
	GetAlertSelector() *metav1.LabelSelector
	GetConfigSelector() *metav1.LabelSelector
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
