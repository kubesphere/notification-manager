package silence

import (
	"context"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/pkg/controller"
	"github.com/kubesphere/notification-manager/pkg/stage"
	"github.com/kubesphere/notification-manager/pkg/template"
	"github.com/kubesphere/notification-manager/pkg/utils"
	"github.com/modern-go/reflect2"
)

type silenceStage struct {
	notifierCtl *controller.Controller
}

func NewStage(notifierCtl *controller.Controller) stage.Stage {
	return &silenceStage{
		notifierCtl,
	}
}

func (s *silenceStage) Exec(ctx context.Context, l log.Logger, data interface{}) (context.Context, interface{}, error) {

	if reflect2.IsNil(data) {
		return ctx, nil, nil
	}

	alerts := data.([]*template.Alert)

	_ = level.Debug(l).Log("msg", "Start silence stage", "seq", ctx.Value("seq"), "alert", len(alerts))

	ss, err := s.notifierCtl.GetActiveSilences(ctx, "")
	if err != nil {
		_ = level.Error(l).Log("msg", "Get silence failed", "stage", "Silence", "seq", ctx.Value("seq"), "error", err.Error())
		return ctx, nil, err
	}

	if len(ss) == 0 {
		return ctx, alerts, nil
	}

	var as []*template.Alert
	for _, alert := range alerts {
		mute := false
		for _, silence := range ss {
			if utils.LabelMatchSelector(alert.Labels, silence.Spec.Matcher) {
				mute = true
				break
			}
		}

		if !mute {
			as = append(as, alert)
		}
	}

	return ctx, as, nil
}
