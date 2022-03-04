package store

import (
	"github.com/kubesphere/notification-manager/pkg/store/provider"
	"github.com/kubesphere/notification-manager/pkg/store/provider/memory"
)

const (
	providerMemory = "memory"
)

type AlertStore struct {
	provider.Provider
}

func NewAlertStore(provider string) *AlertStore {

	as := &AlertStore{}

	if provider == providerMemory {
		as.Provider = memory.NewProvider()
	}

	return as
}
