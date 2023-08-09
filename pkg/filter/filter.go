package filter

import (
	"context"

	"github.com/kubesphere/notification-manager/pkg/apis/v2beta2"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/pkg/controller"
	"github.com/kubesphere/notification-manager/pkg/internal"
	"github.com/kubesphere/notification-manager/pkg/stage"
	"github.com/kubesphere/notification-manager/pkg/template"
	"github.com/kubesphere/notification-manager/pkg/utils"
	"github.com/modern-go/reflect2"
	"k8s.io/apimachinery/pkg/labels"
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

	alertMap := data.(map[internal.Receiver][]*template.Alert)
	res := make(map[internal.Receiver][]*template.Alert)
	for receiver, alerts := range alertMap {
		as, err := s.mute(ctx, alerts, receiver)
		if err != nil {
			_ = level.Error(l).Log("msg", "Mute failed", "stage", "Filter", "seq", ctx.Value("seq"), "tenant", receiver.GetTenantID(), "error", err.Error())
			return ctx, data, err
		}

		as, err = filter(as, receiver.GetAlertSelector())
		if err != nil {
			_ = level.Error(l).Log("msg", "Filter failed", "stage", "Filter", "seq", ctx.Value("seq"), "error", err.Error(), "receiver", receiver.GetName())
			return ctx, nil, err
		}

		res[receiver] = as
	}

	return ctx, res, nil
}

func (s *filterStage) mute(ctx context.Context, alerts []*template.Alert, receiver internal.Receiver) ([]*template.Alert, error) {

	silences, err := s.notifierCtl.GetActiveSilences(ctx, receiver.GetTenantID())
	if err != nil {
		return nil, err
	}

	if len(silences) == 0 {
		return alerts, nil
	}

	var as []*template.Alert
	for _, alert := range alerts {
		flag := false
		for _, silence := range silences {
			if !silence.IsActive() {
				continue
			}

			if utils.LabelMatchSelector(alert.Labels, silence.Spec.Matcher) {
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

// FilterAlerts filter the alerts with label selector,if the selector is not correct, return all of the alerts.
func filter(alerts []*template.Alert, selector *v2beta2.LabelSelector) ([]*template.Alert, error) {

	if selector == nil {
		return alerts, nil
	}

	labelSelector, err := utils.LabelSelectorDeal(selector)
	if err != nil {
		return alerts, err
	}

	if labelSelector.Empty() {
		return alerts, nil
	}

	var as []*template.Alert
	for _, alert := range alerts {
		if labelSelector.Matches(labels.Set(alert.Labels)) {
			as = append(as, alert)
		}
	}

	return as, nil
}
