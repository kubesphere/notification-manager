package notify

import (
	"github.com/go-kit/kit/log"
	nmv1alpha1 "github.com/kubesphere/notification-manager/pkg/apis/v1alpha1"
	"github.com/kubesphere/notification-manager/pkg/notify/config"
	"github.com/prometheus/alertmanager/template"
	"reflect"
)

type Notifier interface {
	Notify(data []template.Data) []error
}

type Factory func(logger log.Logger, receiver interface{}, opts *nmv1alpha1.Options) Notifier

var (
	factorys map[string]Factory
)

func Register(name string, factory Factory) {
	if factorys == nil {
		factorys = make(map[string]Factory)
	}

	factorys[name] = factory
}

type Notification struct {
	Notifiers []Notifier
	Data      []template.Data
}

func NewNotification(logger log.Logger, receivers []*config.Receiver, opts *nmv1alpha1.Options, data []template.Data) *Notification {

	n := &Notification{Data: data}
	for _, receiver := range receivers {
		t := reflect.TypeOf(*receiver)
		v := reflect.ValueOf(*receiver)
		for i := 0; i < v.NumField(); i++ {
			// Dose the field can be export?
			if v.Field(i).CanInterface() {
				factory := factorys[t.Field(i).Name]
				if factory != nil && v.Field(i).Interface() != nil {
					notifier := factory(logger, v.Field(i).Interface(), opts)
					if notifier != nil {
						n.Notifiers = append(n.Notifiers, notifier)
					}
				}
			}
		}
	}

	return n
}

func (n *Notification) Notify() []error {

	var errs []error
	for _, notifier := range n.Notifiers {

		if notifier == nil {
			continue
		}

		err := notifier.Notify(n.Data)
		if err != nil {
			errs = append(errs, err...)
		}
	}

	return errs
}
