package sms

import (
	"context"
	"errors"

	"github.com/kubesphere/notification-manager/pkg/apis/v2beta2"
	"github.com/kubesphere/notification-manager/pkg/notify/config"
)

type Provider interface {
	MakeRequest(ctx context.Context, messages string) error
}

type ProviderFactory func(c *config.Config, providers *v2beta2.Providers, phoneNumbers []string) Provider

var availableFactoryFuncs = map[string]ProviderFactory{}

// register providers here
func init() {
	Register("aliyun", NewAliyunProvider)
}

func Register(name string, p ProviderFactory) {
	if len(availableFactoryFuncs) == 0 {
		availableFactoryFuncs = make(map[string]ProviderFactory)
	}
	availableFactoryFuncs[name] = p
}

func GetProviderFunc(name string) (ProviderFactory, bool) {
	p, ok := availableFactoryFuncs[name]
	return p, ok
}

func GetFirstAvailableProviderFunc() (ProviderFactory, error) {
	for _, p := range availableFactoryFuncs {
		if p != nil {
			return p, nil
		}
	}
	return nil, errors.New("cannot find a registered provider")
}
