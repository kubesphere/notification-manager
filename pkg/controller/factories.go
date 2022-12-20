package controller

import (
	"fmt"
	"github.com/kubesphere/notification-manager/pkg/internal/discord"

	"github.com/kubesphere/notification-manager/pkg/apis/v2beta2"
	"github.com/kubesphere/notification-manager/pkg/constants"
	"github.com/kubesphere/notification-manager/pkg/internal"
	"github.com/kubesphere/notification-manager/pkg/internal/dingtalk"
	"github.com/kubesphere/notification-manager/pkg/internal/email"
	"github.com/kubesphere/notification-manager/pkg/internal/feishu"
	"github.com/kubesphere/notification-manager/pkg/internal/pushover"
	"github.com/kubesphere/notification-manager/pkg/internal/slack"
	"github.com/kubesphere/notification-manager/pkg/internal/sms"
	"github.com/kubesphere/notification-manager/pkg/internal/webhook"
	"github.com/kubesphere/notification-manager/pkg/internal/wechat"
	"github.com/kubesphere/notification-manager/pkg/utils"
	"github.com/modern-go/reflect2"
)

type receiverFactory = func(tenantID string, obj *v2beta2.Receiver) internal.Receiver
type configFactory = func(obj *v2beta2.Config) internal.Config

var receiverFactories map[string]receiverFactory
var configFactories map[string]configFactory

func init() {
	receiverFactories = make(map[string]receiverFactory)
	receiverFactories[constants.DingTalk] = dingtalk.NewReceiver
	receiverFactories[constants.Email] = email.NewReceiver
	receiverFactories[constants.Feishu] = feishu.NewReceiver
	receiverFactories[constants.Pushover] = pushover.NewReceiver
	receiverFactories[constants.Slack] = slack.NewReceiver
	receiverFactories[constants.SMS] = sms.NewReceiver
	receiverFactories[constants.Webhook] = webhook.NewReceiver
	receiverFactories[constants.WeChat] = wechat.NewReceiver
	receiverFactories[constants.Discord] = discord.NewReceiver

	configFactories = make(map[string]configFactory)
	configFactories[constants.DingTalk] = dingtalk.NewConfig
	configFactories[constants.Email] = email.NewConfig
	configFactories[constants.Feishu] = feishu.NewConfig
	configFactories[constants.Pushover] = pushover.NewConfig
	configFactories[constants.Slack] = slack.NewConfig
	configFactories[constants.SMS] = sms.NewConfig
	configFactories[constants.Webhook] = webhook.NewConfig
	configFactories[constants.WeChat] = wechat.NewConfig
	configFactories[constants.Discord] = discord.NewConfig
}

func NewReceivers(tenantID string, obj *v2beta2.Receiver) map[string]internal.Receiver {

	if obj == nil {
		return nil
	}

	m := make(map[string]internal.Receiver)
	for k, fn := range receiverFactories {
		if r := fn(tenantID, obj); !reflect2.IsNil(r) {
			r.SetHash(utils.Hash(r))
			m[fmt.Sprintf("%s/%s", k, obj.Name)] = r
		}
	}

	return m
}

func NewConfigs(obj *v2beta2.Config) map[string]internal.Config {

	if obj == nil {
		return nil
	}

	m := make(map[string]internal.Config)
	for k, fn := range configFactories {
		if c := fn(obj); !reflect2.IsNil(c) {
			m[fmt.Sprintf("%s/%s", k, obj.Name)] = c
		}
	}

	return m
}
