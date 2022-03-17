package notifier

import (
	"context"

	"github.com/kubesphere/notification-manager/pkg/template"
)

type Notifier interface {
	Notify(ctx context.Context, data *template.Data) error
}
