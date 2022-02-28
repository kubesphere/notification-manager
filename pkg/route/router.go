package route

import (
	"context"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/pkg/apis/v2beta2"
	"github.com/kubesphere/notification-manager/pkg/constants"
	"github.com/kubesphere/notification-manager/pkg/controller"
	"github.com/kubesphere/notification-manager/pkg/internal"
	"github.com/kubesphere/notification-manager/pkg/stage"
	"github.com/kubesphere/notification-manager/pkg/utils"
	"github.com/modern-go/reflect2"
	"github.com/prometheus/common/model"
)

const (
	RouterPolicyAll = "All"
	RouterFirst     = "RouterFirst"
)

type routeStage struct {
	notifierCtl *controller.Controller
}

type packet struct {
	receiver internal.Receiver
	alerts   []*model.Alert
}

func NewStage(notifierCtl *controller.Controller) stage.Stage {
	return &routeStage{
		notifierCtl: notifierCtl,
	}
}

func (s *routeStage) Exec(ctx context.Context, l log.Logger, data interface{}) (context.Context, interface{}, error) {

	if reflect2.IsNil(data) {
		return ctx, nil, nil
	}

	alertArray := data.([]*model.Alert)

	_ = level.Debug(l).Log("msg", "RouteStage: start", "seq", ctx.Value("seq"), "alert", len(alertArray))

	routers, err := s.notifierCtl.GetActiveRouters(ctx)
	if err != nil {
		_ = level.Error(l).Log("msg", "RouteStage: get router failed", "error", "seq", ctx.Value("seq"), err.Error())
		return ctx, nil, err
	}

	// Grouping alerts by namespace
	alertMap := make(map[string][]*model.Alert)
	for _, alert := range alertArray {
		ns := string(alert.Labels[constants.Namespace])
		as := alertMap[ns]
		as = append(as, alert)
		alertMap[ns] = as
	}

	m := make(map[string]*packet)
	routePolicy := s.notifierCtl.GetRoutePolicy()
	for ns, alerts := range alertMap {
		flag := false
		var tenantRcvs []internal.Receiver
		for _, alert := range alerts {
			rcvs := s.rcvsFromRouter(alert, routers)
			if routePolicy == RouterPolicyAll || (routePolicy == RouterFirst && len(rcvs) == 0) {
				if len(tenantRcvs) == 0 && !flag {
					tenantRcvs = s.notifierCtl.RcvsFromNs(&ns)
					flag = true
				}
				rcvs = append(rcvs, tenantRcvs...)
			}

			rcvs = deduplication(rcvs)
			for _, rcv := range rcvs {
				hash := rcv.GetHash()
				p := m[hash]
				if p == nil {
					p = &packet{
						receiver: rcv,
					}
				}
				p.alerts = append(p.alerts, alert)
				m[hash] = p
			}
		}
	}

	if len(m) == 0 {
		return ctx, nil, nil
	}

	res := make(map[internal.Receiver][]*model.Alert)
	for _, p := range m {
		res[p.receiver] = p.alerts
	}

	return ctx, res, nil
}

func (s *routeStage) rcvsFromRouter(alert *model.Alert, routers []v2beta2.Router) []internal.Receiver {

	var rcvs []internal.Receiver
	for _, router := range routers {
		if !utils.LabelMatchSelector(utils.LabelSetToKV(alert.Labels), router.Spec.AlertSelector) {
			continue
		}

		if len(router.Spec.Receivers.Name) > 0 || !utils.StringIsNil(router.Spec.Receivers.RegexName) {
			rcvs = append(rcvs, s.notifierCtl.RcvsFromName(router.Spec.Receivers.Name, router.Spec.Receivers.RegexName, router.Spec.Receivers.Type)...)
		}
		if router.Spec.Receivers.Selector != nil {
			rcvs = append(rcvs, s.notifierCtl.RcvsFromSelector(router.Spec.Receivers.Selector, router.Spec.Receivers.Type)...)
		}
	}

	return rcvs
}

func deduplication(rcvs []internal.Receiver) []internal.Receiver {

	m := make(map[string]internal.Receiver)
	for _, rcv := range rcvs {
		m[rcv.GetHash()] = rcv
	}

	var res []internal.Receiver
	for _, v := range m {
		res = append(res, v)
	}

	return res
}
