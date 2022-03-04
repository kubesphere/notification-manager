package provider

import (
	"time"

	"github.com/prometheus/common/model"
)

type Provider interface {
	Push(alert *model.Alert) error
	Pull(batchSize int, batchWait time.Duration) ([]*model.Alert, error)
	Close() error
}
