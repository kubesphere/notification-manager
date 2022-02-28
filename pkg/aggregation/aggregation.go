package aggregation

import (
	"context"
	"encoding/json"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/pkg/controller"
	"github.com/kubesphere/notification-manager/pkg/internal"
	"github.com/kubesphere/notification-manager/pkg/stage"
	"github.com/kubesphere/notification-manager/pkg/utils"
	"github.com/modern-go/reflect2"
	"github.com/prometheus/common/model"
)

type aggregationStage struct {
	notifierCtl *controller.Controller
}

func NewStage(notifierCtl *controller.Controller) stage.Stage {
	return &aggregationStage{
		notifierCtl: notifierCtl,
	}
}

func (s *aggregationStage) Exec(ctx context.Context, l log.Logger, data interface{}) (context.Context, interface{}, error) {

	if reflect2.IsNil(data) {
		return ctx, nil, nil
	}

	groupLabel := s.notifierCtl.GetGroupLabels()
	_ = level.Debug(l).Log("msg", "Start aggregation stage", "seq", ctx.Value("seq"), "group by", utils.ArrayToString(groupLabel, ","))

	alertMap := data.(map[internal.Receiver][]*model.Alert)

	res := make(map[internal.Receiver]map[string][]*model.Alert)
	for receiver, alerts := range alertMap {
		m := make(map[string][]*model.Alert)
		for _, alert := range alerts {
			group := getGroup(groupLabel, alert)
			as := m[group]
			as = append(as, alert)
			m[group] = as
		}

		res[receiver] = m
	}

	return ctx, res, nil
}

func getGroup(groupLabel []string, alert *model.Alert) string {

	m := make(map[string]string)
	for _, k := range groupLabel {
		m[k] = string(alert.Labels[model.LabelName(k)])
	}

	bs, _ := json.Marshal(m)

	return string(bs)
}
