package notify

import (
	"context"
	"github.com/go-kit/kit/log"
	"github.com/kubesphere/notification-manager/pkg/async"
	"github.com/kubesphere/notification-manager/pkg/notify/config"
	"github.com/kubesphere/notification-manager/pkg/notify/notifier"
	"github.com/kubesphere/notification-manager/pkg/notify/notifier/dingtalk"
	"github.com/kubesphere/notification-manager/pkg/notify/notifier/email"
	"github.com/kubesphere/notification-manager/pkg/notify/notifier/slack"
	"github.com/kubesphere/notification-manager/pkg/notify/notifier/webhook"
	"github.com/kubesphere/notification-manager/pkg/notify/notifier/wechat"
	"github.com/prometheus/alertmanager/template"
)

type Factory func(logger log.Logger, receivers []config.Receiver, notifierCfg *config.Config) notifier.Notifier

var (
	factories map[string]Factory
)

func init() {
	Register("Email", email.NewEmailNotifier)
	Register("Wechat", wechat.NewWechatNotifier)
	Register("Slack", slack.NewSlackNotifier)
	Register("Webhook", webhook.NewWebhookNotifier)
	Register("DingTalk", dingtalk.NewDingTalkNotifier)
}

func Register(name string, factory Factory) {
	if factories == nil {
		factories = make(map[string]Factory)
	}

	factories[name] = factory
}

type Notification struct {
	Notifiers []notifier.Notifier
	Data      template.Data
}

func NewNotification(logger log.Logger, receivers []config.Receiver, notifierCfg *config.Config, data template.Data) *Notification {

	n := &Notification{Data: data}

	if receivers == nil || len(receivers) == 0 {
		return n
	}

	for _, f := range factories {
		if f != nil {
			n.Notifiers = append(n.Notifiers, f(logger, receivers, notifierCfg))
		}
	}

	return n
}

func (n *Notification) Notify(ctx context.Context) []error {

	group := async.NewGroup(ctx)
	for _, notify := range n.Notifiers {
		if notify != nil {
			nf := notify
			group.Add(func(stopCh chan interface{}) {
				stopCh <- nf.Notify(ctx, n.Data)
			})
		}
	}

	return group.Wait()
}
