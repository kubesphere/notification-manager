package notifier

import (
	"context"
	"github.com/prometheus/alertmanager/template"
)

type Notifier interface {
	Notify(ctx context.Context, data template.Data) []error
}
