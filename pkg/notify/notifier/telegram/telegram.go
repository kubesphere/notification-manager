package telegram

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/pkg/async"
	"github.com/kubesphere/notification-manager/pkg/controller"
	"github.com/kubesphere/notification-manager/pkg/internal"
	"github.com/kubesphere/notification-manager/pkg/internal/telegram"
	"github.com/kubesphere/notification-manager/pkg/notify/notifier"
	"github.com/kubesphere/notification-manager/pkg/template"
	"github.com/kubesphere/notification-manager/pkg/utils"
)

const (
	DefaultSendTimeout = time.Second * 3
	URL                = "https://api.telegram.org/bot%s/sendMessage"
	DefaultTemplate    = `{{ template "nm.default.text" . }}`
)

type Notifier struct {
	notifierCtl *controller.Controller
	receiver    *telegram.Receiver
	timeout     time.Duration
	logger      log.Logger
	tmpl        *template.Template

	sentSuccessfulHandler *func([]*template.Alert)
}

type telegramRequest struct {
	ChatID string `json:"chat_id"`
	Text   string `json:"text"`
}

type telegramResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description,omitempty"`
	ErrorCode   int    `json:"error_code,omitempty"`
}

func NewTelegramNotifier(logger log.Logger, receiver internal.Receiver, notifierCtl *controller.Controller) (notifier.Notifier, error) {

	n := &Notifier{
		notifierCtl: notifierCtl,
		timeout:     DefaultSendTimeout,
		logger:      logger,
	}

	opts := notifierCtl.ReceiverOpts
	tmplName := DefaultTemplate
	if opts != nil && opts.Global != nil && !utils.StringIsNil(opts.Global.Template) {
		tmplName = opts.Global.Template
	}

	if opts != nil && opts.Telegram != nil {

		if opts.Telegram.NotificationTimeout != nil {
			n.timeout = time.Second * time.Duration(*opts.Telegram.NotificationTimeout)
		}

		if !utils.StringIsNil(opts.Telegram.Template) {
			tmplName = opts.Telegram.Template
		}
	}

	n.receiver = receiver.(*telegram.Receiver)
	if n.receiver.Config == nil {
		_ = level.Warn(logger).Log("msg", "TelegramNotifier: ignore receiver because of empty config")
		return nil, utils.Error("ignore receiver because of empty config")
	}

	if utils.StringIsNil(n.receiver.TmplName) {
		n.receiver.TmplName = tmplName
	}

	var err error
	n.tmpl, err = notifierCtl.GetReceiverTmpl(n.receiver.TmplText)
	if err != nil {
		_ = level.Error(n.logger).Log("msg", "TelegramNotifier: create receiver template error", "error", err.Error())
		return nil, err
	}

	return n, nil
}

func (n *Notifier) SetSentSuccessfulHandler(h *func([]*template.Alert)) {
	n.sentSuccessfulHandler = h
}

func (n *Notifier) Notify(ctx context.Context, data *template.Data) error {

	msg, err := n.tmpl.Text(n.receiver.TmplName, data)
	if err != nil {
		_ = level.Error(n.logger).Log("msg", "TelegramNotifier: generate message error", "error", err.Error())
		return err
	}

	if len(n.receiver.MentionedUsers) > 0 {
		msg += "\n"
		for _, mentionedUser := range n.receiver.MentionedUsers {
			msg += "@" + mentionedUser + " "
		}
	}

	token, err := n.notifierCtl.GetCredential(n.receiver.Token)
	if err != nil {
		_ = level.Error(n.logger).Log("msg", "TelegramNotifier: get token secret", "error", err.Error())
		return err
	}

	send := func(channel string) error {

		start := time.Now()
		defer func() {
			_ = level.Debug(n.logger).Log("msg", "TelegramNotifier: send message", "channel", channel, "used", time.Since(start).String())
		}()

		sr := &telegramRequest{
			ChatID: channel,
			Text:   msg,
		}

		var buf bytes.Buffer
		if err := utils.JsonEncode(&buf, sr); err != nil {
			_ = level.Error(n.logger).Log("msg", "TelegramNotifier: encode message error", "channel", channel, "error", err.Error())
			return err
		}

		request, err := http.NewRequest(http.MethodPost, fmt.Sprintf(URL, token), &buf)
		if err != nil {
			return err
		}
		request.Header.Set("Content-Type", "application/json")

		body, err := utils.DoHttpRequest(ctx, nil, request.WithContext(ctx))
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "TelegramNotifier: do http error", "channel", channel, "error", err)
			return err
		}

		var resp telegramResponse
		if err := utils.JsonUnmarshal(body, &resp); err != nil {
			_ = level.Error(n.logger).Log("msg", "TelegramNotifier: decode response body error", "channel", channel, "error", err)
			return err
		}

		if !resp.OK {
			_ = level.Error(n.logger).Log("msg", "TelegramNotifier: send message error", "channel", channel, "error", resp.Description)
			return utils.Error(resp.Description)
		}

		_ = level.Debug(n.logger).Log("msg", "TelegramNotifier: send message", "channel", channel)

		return nil
	}

	group := async.NewGroup(ctx)
	for _, channel := range n.receiver.Channels {
		ch := channel
		group.Add(func(stopCh chan interface{}) {
			err := send(ch)
			if err == nil {
				if n.sentSuccessfulHandler != nil {
					(*n.sentSuccessfulHandler)(data.Alerts)
				}
			}
			stopCh <- err
		})
	}

	return group.Wait()
}
