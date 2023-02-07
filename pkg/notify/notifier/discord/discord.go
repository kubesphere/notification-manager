package discord

import (
	"bytes"
	"context"
	"fmt"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/pkg/async"
	"github.com/kubesphere/notification-manager/pkg/constants"
	"github.com/kubesphere/notification-manager/pkg/controller"
	"github.com/kubesphere/notification-manager/pkg/internal"
	"github.com/kubesphere/notification-manager/pkg/internal/discord"
	"github.com/kubesphere/notification-manager/pkg/notify/notifier"
	"github.com/kubesphere/notification-manager/pkg/template"
	"github.com/kubesphere/notification-manager/pkg/utils"
	"net/http"
	"strings"
	"time"
)

const (
	DefaultSendTimeout  = time.Second * 3
	DefaultTextTemplate = `{{ template "nm.default.text" . }}`
	MaxContentLength    = 2000
	EmbedLimit          = 4096
)

type Notifier struct {
	notifierCtl *controller.Controller
	receiver    *discord.Receiver
	timeout     time.Duration
	logger      log.Logger
	tmpl        *template.Template

	sentSuccessfulHandler *func([]*template.Alert)
}
type Message struct {
	Content string   `json:"content"`
	Embeds  []Embeds `json:"embeds,omitempty"`
}

type Embeds struct {
	Description string `json:"description,omitempty"`
}

func NewDiscordNotifier(logger log.Logger, receiver internal.Receiver, notifierCtl *controller.Controller) (notifier.Notifier, error) {

	n := &Notifier{
		notifierCtl: notifierCtl,
		logger:      logger,
		timeout:     DefaultSendTimeout,
	}

	opts := notifierCtl.ReceiverOpts
	tmplType := constants.Text
	tmplName := ""
	if opts != nil && opts.Global != nil && !utils.StringIsNil(opts.Global.Template) {
		tmplName = opts.Global.Template
	}

	if opts != nil && opts.Discord != nil {
		if !utils.StringIsNil(opts.Discord.Template) {
			tmplName = opts.Discord.Template
		}

		if opts.Discord.NotificationTimeout != nil {
			n.timeout = time.Second * time.Duration(*opts.Discord.NotificationTimeout)
		}
	}

	n.receiver = receiver.(*discord.Receiver)
	if utils.StringIsNil(n.receiver.TmplType) {
		n.receiver.TmplType = tmplType
	}

	if utils.StringIsNil(n.receiver.TmplName) {
		if tmplName != "" {
			n.receiver.TmplName = tmplName
		} else {
			n.receiver.TmplName = DefaultTextTemplate
		}
	}

	var err error
	n.tmpl, err = notifierCtl.GetReceiverTmpl(n.receiver.TmplText)
	if err != nil {
		_ = level.Error(n.logger).Log("msg", "DiscordNotifier: create receiver template error", "error", err.Error())
		return nil, err
	}
	return n, nil
}

func (n *Notifier) SetSentSuccessfulHandler(h *func([]*template.Alert)) {
	n.sentSuccessfulHandler = h
}

func (n *Notifier) Notify(ctx context.Context, data *template.Data) error {

	mentionedUsers := n.receiver.MentionedUsers
	for i := range mentionedUsers {
		if mentionedUsers[i] == "everyone" {
			mentionedUsers[i] = "@everyone"
			continue
		}
		mentionedUsers[i] = fmt.Sprintf("<@%s>", mentionedUsers[i])
	}

	mentionedRoles := n.receiver.MentionedRoles
	for i := range mentionedRoles {
		mentionedRoles[i] = fmt.Sprintf("<@&%s>", mentionedRoles[i])
	}

	atUsers := strings.Join(mentionedUsers, "")
	atRoles := strings.Join(mentionedRoles, "")
	var length int
	if n.receiver.Type == nil || *n.receiver.Type == constants.DiscordContent {
		length = MaxContentLength - len(atUsers) - len(atRoles)
	} else {
		length = EmbedLimit - len(atUsers) - len(atRoles)
	}
	splitData, err := n.tmpl.Split(data, length, n.receiver.TmplName, "", n.logger)
	if err != nil {
		_ = level.Error(n.logger).Log("msg", "DiscordNotifier: generate message error", "error", err.Error())
		return err
	}

	group := async.NewGroup(ctx)
	if n.receiver.Webhook != nil {
		for index := range splitData {
			d := splitData[index]
			alerts := d.Alerts
			msg := d.Message
			group.Add(func(stopCh chan interface{}) {
				msg := fmt.Sprintf("%s\n%s%s", msg, atUsers, atRoles)
				err := n.sendTo(ctx, msg)
				if err == nil {
					if n.sentSuccessfulHandler != nil {
						(*n.sentSuccessfulHandler)(alerts)
					}
				}
				stopCh <- err
			})
		}
	}
	return group.Wait()
}

func (n *Notifier) sendTo(ctx context.Context, content string) error {

	message := &Message{}
	if n.receiver.Type == nil || *n.receiver.Type == constants.DiscordContent {
		message.Content = content
	} else {
		message.Embeds = []Embeds{
			{
				Description: content,
			},
		}
	}

	send := func() (bool, error) {
		url, err := n.notifierCtl.GetCredential(n.receiver.Webhook)
		if err != nil {
			return false, err
		}
		var buf bytes.Buffer
		err = utils.JsonEncode(&buf, message)
		if err != nil {
			return false, err
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
		if err != nil {
			return false, err
		}
		request.Header.Set("Content-Type", "application/json; charset=utf-8")
		client := http.Client{}
		resp, err := client.Do(request)
		if err != nil {
			return true, err
		}
		code := resp.StatusCode
		_ = level.Debug(n.logger).Log("msg", "DiscordNotifier", "response code:", code)

		if code != http.StatusNoContent {
			return false, fmt.Errorf("DiscordNotifier: send message error, code: %d", code)
		}
		return false, nil
	}

	start := time.Now()
	defer func() {
		_ = level.Debug(n.logger).Log("msg", "DiscordNotifier: send message", "used", time.Since(start).String())
	}()

	retry := 0
	MaxRetry := 3
	for {
		if retry >= MaxRetry {
			return fmt.Errorf("DiscordNotifier: send message error, retry %d times", retry)
		}
		needRetry, err := send()
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "DiscordNotifier: send notification error", "error", err.Error())
		}
		if needRetry {
			retry = retry + 1
			time.Sleep(time.Second)
			_ = level.Info(n.logger).Log("msg", "DiscordNotifier: retry to send notification", "retry", retry)
			continue
		}

		return err
	}
}
