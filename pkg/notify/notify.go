package notify

import (
	"github.com/go-kit/kit/log"
	nmv1alpha1 "github.com/kubesphere/notification-manager/pkg/apis/v1alpha1"
	"github.com/kubesphere/notification-manager/pkg/notify/config"
	"github.com/kubesphere/notification-manager/pkg/notify/notifier"
	"github.com/kubesphere/notification-manager/pkg/notify/notifier/email"
	"github.com/prometheus/alertmanager/template"
	"reflect"
)

type Factory func(logger log.Logger, val interface{}, opts *nmv1alpha1.Options) notifier.Notifier

var (
	factories map[string]Factory
)

func init() {
	Register("Email", email.NewEmailNotifier)
}

func Register(name string, factory Factory) {
	if factories == nil {
		factories = make(map[string]Factory)
	}

	factories[name] = factory
}

type Notification struct {
	Notifiers []notifier.Notifier
	Data      template.Data
}

func NewNotification(logger log.Logger, receivers []*config.Receiver, opts *nmv1alpha1.Options, data template.Data) *Notification {

	m := make(map[string][]interface{})
	for _, receiver := range receivers {
		t := reflect.TypeOf(*receiver)
		v := reflect.ValueOf(*receiver)
		for i := 0; i < v.NumField(); i++ {
			// Dose the field can be export?
			if v.Field(i).CanInterface() {
				key := t.Field(i).Name
				val := v.Field(i).Interface()
				if val == nil {
					continue
				}

				l, ok := m[key]
				if !ok {
					l = []interface{}{}
				}

				l = append(l, val)
				m[key] = l
			}
		}
	}

	n := &Notification{Data: data}
	for k, v := range m {
		factory := factories[k]
		if factory != nil && v != nil {
			n.Notifiers = append(n.Notifiers, factory(logger, v, opts))
		}
	}

	return n
}

func (n *Notification) Notify() []error {

	var errs []error
	for _, nr := range n.Notifiers {

		if nr == nil {
			continue
		}

		err := nr.Notify(n.Data)
		if err != nil {
			errs = append(errs, err...)
		}
	}

	return errs
}
