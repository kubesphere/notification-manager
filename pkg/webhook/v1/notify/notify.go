package notify

import (
	"github.com/go-kit/kit/log"
	"github.com/kubesphere/notification-manager/pkg/notify/config"
	"github.com/prometheus/alertmanager/template"
	"reflect"
)

type Notifier interface {
	Notify(alert template.Alert) []error
}

type Factory func(logger log.Logger, config interface{}) Notifier

var (
	factorys map[string]Factory
)

func Register(name string, factory Factory) {
	if factorys == nil {
		factorys = make(map[string]Factory)
	}

	factorys[name] = factory
}

type Integration struct {
	Notifiers []Notifier
	Alert     template.Alert
}

func NewIntegration(logger log.Logger, receivers []*config.Receiver, alert template.Alert) *Integration {

	integration := &Integration{Alert: alert}
	for _, receiver := range receivers {
		t := reflect.TypeOf(*receiver)
		v := reflect.ValueOf(*receiver)
		for i := 0; i < v.NumField(); i++ {
			// Dose the field can be export?
			if v.Field(i).CanInterface() {
				factory := factorys[t.Field(i).Name]
				if factory != nil && v.Field(i).Interface() != nil {
					notifier := factory(logger, v.Field(i).Interface())
					if notifier != nil {
						integration.Notifiers = append(integration.Notifiers, notifier)
					}
				}
			}
		}
	}

	return integration
}

func (i *Integration) Notify() []error {

	var errs []error
	for _, notifier := range i.Notifiers {
		err := notifier.Notify(i.Alert)
		if err != nil {
			errs = append(errs, err...)
		}
	}

	return errs
}
