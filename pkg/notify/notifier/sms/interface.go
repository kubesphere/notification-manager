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
	Register("tencent", NewTencentProvider)
}

func Register(name string, p ProviderFactory) {
	if len(availableFactoryFuncs) == 0 {
		availableFactoryFuncs = make(map[string]ProviderFactory)
	}
	availableFactoryFuncs[name] = p
}

func GetProviderFunc(name string) (ProviderFactory, error) {
	if name != "" {
		// check whether the default provider is registered
		p, ok := availableFactoryFuncs[name]
		if !ok {
			return nil, errors.New("the given default sms provider not registered")
		}
		return p, nil
	} else {
		// use the first available provider func if the default provider not given
		var providerFunc ProviderFactory
		for _, p := range availableFactoryFuncs {
			if p != nil {
				providerFunc = p
				break
			}
		}
		if providerFunc != nil {
			return providerFunc, nil
		}
		return nil, errors.New("cannot find a registered provider")
	}
}
