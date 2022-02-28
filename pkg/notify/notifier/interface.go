package notifier

import (
	"context"

	"github.com/prometheus/common/model"
)

type Alerts struct {
	Alerts     []*model.Alert
	GroupLabel model.LabelSet
}

type Notifier interface {
	Notify(ctx context.Context, alerts *Alerts) error
}
