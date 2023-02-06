package history

import (
	"context"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/pkg/controller"
	"github.com/kubesphere/notification-manager/pkg/internal"
	"github.com/kubesphere/notification-manager/pkg/notify"
	"github.com/kubesphere/notification-manager/pkg/stage"
	"github.com/kubesphere/notification-manager/pkg/template"
	"github.com/modern-go/reflect2"
)

const (
	historyRetryMax   = 3
	historyRetryDelay = time.Second * 5
)

type historyStage struct {
	notifierCtl *controller.Controller
}

func NewStage(notifierCtl *controller.Controller) stage.Stage {
	return &historyStage{
		notifierCtl: notifierCtl,
	}
}

func (s *historyStage) Exec(ctx context.Context, l log.Logger, data interface{}) (context.Context, interface{}, error) {
	if reflect2.IsNil(data) {
		return ctx, nil, nil
	}

	receivers := s.notifierCtl.GetHistoryReceivers()
	if len(receivers) == 0 {
		return ctx, nil, nil
	}

	_ = level.Debug(l).Log("msg", "Start history stage", "seq", ctx.Value("seq"))

	input := data.(map[string]*template.Alert)
	d := &template.Data{}
	for _, alert := range input {
		if alert.NotifySuccessful {
			d.Alerts = append(d.Alerts, alert)
		}
	}

	if len(d.Alerts) == 0 {
		return ctx, nil, nil
	}

	alertMap := make(map[internal.Receiver][]*template.Data)
	for _, receiver := range receivers {
		alertMap[receiver] = []*template.Data{d}
	}

	for retry := 0; retry <= historyRetryMax; retry++ {
		notifyStage := notify.NewStage(s.notifierCtl)
		if _, _, err := notifyStage.Exec(ctx, l, alertMap); err == nil {
			return ctx, nil, nil
		}

		_ = level.Error(l).Log("msg", "Export history error", "seq", ctx.Value("seq"), "retry", retry)
		time.Sleep(historyRetryDelay)
	}

	_ = level.Error(l).Log("msg", "Export history error, all retry failed", "seq", ctx.Value("seq"))
	return ctx, data, nil
}
