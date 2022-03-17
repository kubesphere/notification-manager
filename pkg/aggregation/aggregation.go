package aggregation

import (
	"context"
	"encoding/json"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/pkg/controller"
	"github.com/kubesphere/notification-manager/pkg/internal"
	"github.com/kubesphere/notification-manager/pkg/stage"
	"github.com/kubesphere/notification-manager/pkg/template"
	"github.com/kubesphere/notification-manager/pkg/utils"
	"github.com/modern-go/reflect2"
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

	alertMap := data.(map[internal.Receiver][]*template.Alert)

	res := make(map[internal.Receiver][]*template.Data)
	for receiver, alerts := range alertMap {
		m := make(map[string][]*template.Alert)
		for _, alert := range alerts {
			group := labelToGroupKey(groupLabel, alert)
			as := m[group]
			as = append(as, alert)
			m[group] = as
		}

		var ds []*template.Data
		for k, v := range m {
			d := &template.Data{
				GroupLabels: groupKeyToLabel(k),
				Alerts:      v,
			}
			ds = append(ds, d.Format())
		}

		res[receiver] = ds
	}

	return ctx, res, nil
}

func labelToGroupKey(groupLabel []string, alert *template.Alert) string {

	m := make(map[string]string)
	for _, k := range groupLabel {
		m[k] = alert.Labels[k]
	}

	bs, _ := json.Marshal(m)

	return string(bs)
}

func groupKeyToLabel(groupKey string) template.KV {

	label := template.KV{}
	_ = utils.JsonUnmarshal([]byte(groupKey), &label)
	return label
}
