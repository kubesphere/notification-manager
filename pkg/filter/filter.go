package filter

import (
	"context"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/pkg/controller"
	"github.com/kubesphere/notification-manager/pkg/internal"
	"github.com/kubesphere/notification-manager/pkg/stage"
	"github.com/kubesphere/notification-manager/pkg/utils"
	"github.com/modern-go/reflect2"
	"github.com/prometheus/common/model"
)

type filterStage struct {
	notifierCtl *controller.Controller
}

func NewStage(notifierCtl *controller.Controller) stage.Stage {
	return &filterStage{
		notifierCtl,
	}
}

func (s *filterStage) Exec(ctx context.Context, l log.Logger, data interface{}) (context.Context, interface{}, error) {

	if reflect2.IsNil(data) {
		return ctx, nil, nil
	}

	_ = level.Debug(l).Log("msg", "Start filter stage", "seq", ctx.Value("seq"))

	alertMap := data.(map[internal.Receiver][]*model.Alert)
	res := make(map[internal.Receiver][]*model.Alert)
	for receiver, alerts := range alertMap {
		as, err := s.mute(ctx, alerts, receiver)
		if err != nil {
			_ = level.Error(l).Log("msg", "Mute failed", "stage", "Filter", "seq", ctx.Value("seq"), "tenant", receiver.GetTenantID(), "error", err.Error())
			return ctx, data, err
		}

		as, err = utils.FilterAlerts(as, receiver.GetAlertSelector())
		if err != nil {
			_ = level.Error(l).Log("msg", "Filter failed", "stage", "Filter", "seq", ctx.Value("seq"), "error", err.Error(), "receiver", receiver.GetName())
			return ctx, nil, err
		}

		res[receiver] = as
	}

	return ctx, res, nil
}

func (s *filterStage) mute(ctx context.Context, alerts []*model.Alert, receiver internal.Receiver) ([]*model.Alert, error) {

	silences, err := s.notifierCtl.GetActiveSilences(ctx, receiver.GetTenantID())
	if err != nil {
		return nil, err
	}

	if len(silences) == 0 {
		return alerts, nil
	}

	var as []*model.Alert
	for _, alert := range alerts {
		flag := false
		for _, silence := range silences {
			if !silence.IsActive() {
				continue
			}

			if utils.LabelMatchSelector(utils.LabelSetToKV(alert.Labels), silence.Spec.Matcher) {
				flag = true
				break
			}
		}

		if !flag {
			as = append(as, alert)
		}
	}

	return as, err
}
