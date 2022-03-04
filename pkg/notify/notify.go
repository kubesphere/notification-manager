package notify

import (
	"context"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/pkg/async"
	"github.com/kubesphere/notification-manager/pkg/constants"
	"github.com/kubesphere/notification-manager/pkg/controller"
	"github.com/kubesphere/notification-manager/pkg/internal"
	"github.com/kubesphere/notification-manager/pkg/notify/notifier"
	"github.com/kubesphere/notification-manager/pkg/notify/notifier/dingtalk"
	"github.com/kubesphere/notification-manager/pkg/notify/notifier/email"
	"github.com/kubesphere/notification-manager/pkg/notify/notifier/pushover"
	"github.com/kubesphere/notification-manager/pkg/notify/notifier/slack"
	"github.com/kubesphere/notification-manager/pkg/notify/notifier/sms"
	"github.com/kubesphere/notification-manager/pkg/notify/notifier/webhook"
	"github.com/kubesphere/notification-manager/pkg/notify/notifier/wechat"
	"github.com/kubesphere/notification-manager/pkg/stage"
	"github.com/kubesphere/notification-manager/pkg/utils"
	"github.com/modern-go/reflect2"
	"github.com/prometheus/common/model"
)

type Factory func(logger log.Logger, receivers []internal.Receiver, notifierCtl *controller.Controller) notifier.Notifier

var (
	factories map[string]Factory
)

func init() {
	Register(constants.Email, email.NewEmailNotifier)
	Register(constants.WeChat, wechat.NewWechatNotifier)
	Register(constants.Slack, slack.NewSlackNotifier)
	Register(constants.Webhook, webhook.NewWebhookNotifier)
	Register(constants.DingTalk, dingtalk.NewDingTalkNotifier)
	Register(constants.SMS, sms.NewSmsNotifier)
	Register(constants.Pushover, pushover.NewPushoverNotifier)
}

func Register(name string, factory Factory) {
	if factories == nil {
		factories = make(map[string]Factory)
	}

	factories[name] = factory
}

type notifyStage struct {
	notifierCtl *controller.Controller
}

func NewStage(notifierCtl *controller.Controller) stage.Stage {

	return &notifyStage{
		notifierCtl: notifierCtl,
	}
}

func (s *notifyStage) Exec(ctx context.Context, l log.Logger, data interface{}) (context.Context, interface{}, error) {

	if reflect2.IsNil(data) {
		return ctx, nil, nil
	}

	_ = level.Debug(l).Log("msg", "Start notify stage", "seq", ctx.Value("seq"))

	group := async.NewGroup(ctx)

	alertMap := data.(map[internal.Receiver]map[string][]*model.Alert)

	for k, v := range alertMap {
		receiver := k
		nf := factories[receiver.GetType()](l, []internal.Receiver{receiver}, s.notifierCtl)
		if reflect2.IsNil(nf) {
			continue
		}
		alerts := v
		for key, val := range alerts {
			groupBy := key
			as := val
			group.Add(func(stopCh chan interface{}) {
				stopCh <- nf.Notify(ctx, &notifier.Alerts{
					Alerts:     as,
					GroupLabel: getGroupLables(groupBy),
				})
			})
		}
	}

	return ctx, data, group.Wait()
}

func getGroupLables(groupBy string) model.LabelSet {

	labelSet := model.LabelSet{}
	_ = utils.JsonUnmarshal([]byte(groupBy), &labelSet)
	return labelSet
}
