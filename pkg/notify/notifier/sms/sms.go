package sms

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/pkg/controller"
	"github.com/kubesphere/notification-manager/pkg/internal"
	"github.com/kubesphere/notification-manager/pkg/internal/sms"
	"github.com/kubesphere/notification-manager/pkg/notify/notifier"
	"github.com/kubesphere/notification-manager/pkg/template"
	"github.com/kubesphere/notification-manager/pkg/utils"
)

const (
	DefaultSendTimeout = time.Second * 5
	DefaultTemplate    = `{{ template "nm.default.text" . }}`
)

type Notifier struct {
	notifierCtl *controller.Controller
	receiver    *sms.Receiver
	timeout     time.Duration
	logger      log.Logger
	tmpl        *template.Template

	sentSuccessfulHandler *func([]*template.Alert)
}

func NewSmsNotifier(logger log.Logger, receiver internal.Receiver, notifierCtl *controller.Controller) (notifier.Notifier, error) {

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

	if opts != nil && opts.Sms != nil {

		if opts.Sms.NotificationTimeout != nil {
			n.timeout = time.Second * time.Duration(*opts.Sms.NotificationTimeout)
		}

		if !utils.StringIsNil(opts.Sms.Template) {
			tmplName = opts.Sms.Template
		}
	}

	n.receiver = receiver.(*sms.Receiver)
	if n.receiver.Config == nil {
		_ = level.Warn(logger).Log("msg", "SmsNotifier: ignore receiver because of empty config")
		return nil, utils.Error("ignore receiver because of empty config")
	}

	if utils.StringIsNil(n.receiver.TmplName) {
		n.receiver.TmplName = tmplName
	}

	var err error
	n.tmpl, err = notifierCtl.GetReceiverTmpl(n.receiver.TmplText)
	if err != nil {
		_ = level.Error(n.logger).Log("msg", "SmsNotifier: create receiver template error", "error", err.Error())
		return nil, err
	}

	return n, nil
}

func (n *Notifier) SetSentSuccessfulHandler(h *func([]*template.Alert)) {
	n.sentSuccessfulHandler = h
}

func (n *Notifier) Notify(ctx context.Context, data *template.Data) error {

	start := time.Now()
	defer func() {
		_ = level.Debug(n.logger).Log("msg", "SmsNotifier: send message", "used", time.Since(start).String())
	}()

	msg, err := n.tmpl.Text(n.receiver.TmplName, data)
	if err != nil {
		_ = level.Error(n.logger).Log("msg", "SmsNotifier: generate message error", "error", err.Error())
		return err
	}

	// select an available provider function
	providerFunc, err := GetProviderFunc(n.receiver.DefaultProvider)
	if err != nil {
		_ = level.Error(n.logger).Log("msg", "SmsNotifier: no available provider function", "error", err.Error())
		return err
	}

	// new a provider
	provider := providerFunc(n.notifierCtl, n.receiver.Providers, n.receiver.PhoneNumbers)

	// make request by the provider
	ctx, cancel := context.WithTimeout(context.Background(), n.timeout)
	defer cancel()

	if err := provider.MakeRequest(ctx, msg); err != nil {
		_ = level.Error(n.logger).Log("msg", "SmsNotifier: send request failed", "error", err.Error())
		return err
	}

	if n.sentSuccessfulHandler != nil {
		(*n.sentSuccessfulHandler)(data.Alerts)
	}

	_ = level.Info(n.logger).Log("msg", "SmsNotifier: send request successfully")
	return nil
}

func stringValue(a *string) string {
	if a == nil {
		return ""
	}
	return *a
}

func getSha256Code(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return fmt.Sprintf("%x", h.Sum(nil))
}
