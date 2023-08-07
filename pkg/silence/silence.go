package silence

import (
	"context"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/pkg/controller"
	"github.com/kubesphere/notification-manager/pkg/stage"
	"github.com/kubesphere/notification-manager/pkg/template"
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

	input := data.([]*template.Alert)

	_ = level.Debug(l).Log("msg", "Start silence stage", "seq", ctx.Value("seq"), "alert", len(input))

	ss, err := s.notifierCtl.GetActiveSilences(ctx, "")
	if err != nil {
		_ = level.Error(l).Log("msg", "Get silence failed", "stage", "Silence", "seq", ctx.Value("seq"), "error", err.Error())
		return ctx, nil, err
	}

	if len(ss) == 0 {
		return ctx, input, nil
	}

	var output []*template.Alert
	for _, alert := range input {
		mute := false
		for _, silence := range ss {
			ok, err := silence.Spec.Matcher.Matches(alert.Labels)
			if err != nil {
				return nil, nil, err
			}
			if ok {
				mute = true
				break
			}
		}

		if !mute {
			output = append(output, alert)
		}
	}

	return ctx, output, nil
}
