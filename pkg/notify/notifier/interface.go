package notifier

import "github.com/prometheus/alertmanager/template"

type Notifier interface {
	Notify(data template.Data) []error
}