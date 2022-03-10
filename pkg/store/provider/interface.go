package provider

import (
	"time"

	"github.com/kubesphere/notification-manager/pkg/template"
)

type Provider interface {
	Push(alert *template.Alert) error
	Pull(batchSize int, batchWait time.Duration) ([]*template.Alert, error)
	Close() error
}
