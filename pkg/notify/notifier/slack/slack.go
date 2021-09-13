package slack

import (
	"bytes"
	"context"
	"net/http"
	"time"

	"github.com/kubesphere/notification-manager/pkg/internal"
	"github.com/kubesphere/notification-manager/pkg/internal/slack"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/pkg/async"
	"github.com/kubesphere/notification-manager/pkg/config"
	"github.com/kubesphere/notification-manager/pkg/notify/notifier"
	"github.com/kubesphere/notification-manager/pkg/utils"
	"github.com/prometheus/alertmanager/template"
)

const (
	DefaultSendTimeout = time.Second * 3
	URL                = "https://slack.com/api/chat.postMessage"
	DefaultTemplate    = `{{ template "nm.default.text" . }}`
)

type Notifier struct {
	notifierCfg  *config.Config
	receivers    []*slack.Receiver
	timeout      time.Duration
	logger       log.Logger
	template     *notifier.Template
	templateName string
}

type slackRequest struct {
	Channel string `json:"channel"`
	Text    string `json:"text"`
}

type slackResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

func NewSlackNotifier(logger log.Logger, receivers []internal.Receiver, notifierCfg *config.Config) notifier.Notifier {

	var path []string
	opts := notifierCfg.ReceiverOpts
	if opts != nil && opts.Global != nil {
		path = opts.Global.TemplateFiles
	}
	tmpl, err := notifier.NewTemplate(path)
	if err != nil {
		_ = level.Error(logger).Log("msg", "SlackNotifier: get template error", "error", err.Error())
		return nil
	}

	n := &Notifier{
		notifierCfg:  notifierCfg,
		timeout:      DefaultSendTimeout,
		logger:       logger,
		template:     tmpl,
		templateName: DefaultTemplate,
	}

	if opts != nil && opts.Global != nil && !utils.StringIsNil(opts.Global.Template) {
		n.templateName = opts.Global.Template
	}

	if opts != nil && opts.Slack != nil {

		if opts.Slack.NotificationTimeout != nil {
			n.timeout = time.Second * time.Duration(*opts.Slack.NotificationTimeout)
		}

		if !utils.StringIsNil(opts.Slack.Template) {
			n.templateName = opts.Slack.Template
		}
	}

	for _, r := range receivers {
		receiver, ok := r.(*slack.Receiver)
		if !ok || receiver == nil {
			continue
		}

		if receiver.Config == nil {
			_ = level.Warn(logger).Log("msg", "SlackNotifier: ignore receiver because of empty config")
			continue
		}

		if utils.StringIsNil(receiver.Template) {
			receiver.Template = n.templateName
		}

		n.receivers = append(n.receivers, receiver)
	}

	return n
}

func (n *Notifier) Notify(ctx context.Context, data template.Data) []error {

	send := func(channel string, r *slack.Receiver) error {

		start := time.Now()
		defer func() {
			_ = level.Debug(n.logger).Log("msg", "SlackNotifier: send message", "channel", channel, "used", time.Since(start).String())
		}()

		newData := utils.FilterAlerts(data, r.AlertSelector, n.logger)
		if len(newData.Alerts) == 0 {
			return nil
		}

		msg, err := n.template.TempleText(r.Template, newData, n.logger)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "SlackNotifier: generate message error", "channel", channel, "error", err.Error())
			return err
		}

		sr := &slackRequest{
			Channel: channel,
			Text:    msg,
		}

		var buf bytes.Buffer
		if err := utils.JsonEncode(&buf, sr); err != nil {
			_ = level.Error(n.logger).Log("msg", "SlackNotifier: encode message error", "channel", channel, "error", err.Error())
			return err
		}

		request, err := http.NewRequest(http.MethodPost, URL, &buf)
		if err != nil {
			return err
		}
		request.Header.Set("Content-Type", "application/json")

		token, err := n.notifierCfg.GetCredential(r.Token)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "SlackNotifier: get token secret", "channel", channel, "error", err.Error())
			return err
		}

		request.Header.Set("Authorization", "Bearer "+token)

		body, err := utils.DoHttpRequest(ctx, nil, request.WithContext(ctx))
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "SlackNotifier: do http error", "channel", channel, "error", err)
			return err
		}

		var slResp slackResponse
		if err := utils.JsonUnmarshal(body, &slResp); err != nil {
			_ = level.Error(n.logger).Log("msg", "SlackNotifier: decode response body error", "channel", channel, "error", err)
			return err
		}

		if !slResp.OK {
			_ = level.Error(n.logger).Log("msg", "SlackNotifier: send message error", "channel", channel, "error", slResp.Error)
			return utils.Error(slResp.Error)
		}

		_ = level.Debug(n.logger).Log("msg", "SlackNotifier: send message", "channel", channel)

		return nil
	}

	group := async.NewGroup(ctx)
	for _, receiver := range n.receivers {
		r := receiver
		for _, channel := range r.Channels {
			ch := channel
			group.Add(func(stopCh chan interface{}) {
				stopCh <- send(ch, r)
			})
		}
	}

	return group.Wait()
}
