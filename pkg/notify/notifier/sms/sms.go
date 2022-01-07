package sms

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/pkg/async"
	"github.com/kubesphere/notification-manager/pkg/notify/config"
	"github.com/kubesphere/notification-manager/pkg/notify/notifier"
	"github.com/kubesphere/notification-manager/pkg/utils"
	"github.com/prometheus/alertmanager/template"
)

const (
	DefaultSendTimeout = time.Second * 5
	DefaultTemplate    = `{{ template "nm.default.text" . }}`
)

type Notifier struct {
	notifierCfg  *config.Config
	smss         []*config.Sms
	timeout      time.Duration
	logger       log.Logger
	template     *notifier.Template
	templateName string
}

func NewSmsNotifier(logger log.Logger, receivers []config.Receiver, notifierCfg *config.Config) notifier.Notifier {

	var path []string
	opts := notifierCfg.ReceiverOpts
	if opts != nil && opts.Global != nil {
		path = opts.Global.TemplateFiles
	}
	tmpl, err := notifier.NewTemplate(path)
	if err != nil {
		_ = level.Error(logger).Log("msg", "SmsNotifier: get template error", "error", err.Error())
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

	if opts != nil && opts.Sms != nil {

		if opts.Sms.NotificationTimeout != nil {
			n.timeout = time.Second * time.Duration(*opts.Sms.NotificationTimeout)
		}

		if !utils.StringIsNil(opts.Sms.Template) {
			n.templateName = opts.Sms.Template
		}
	}

	for _, r := range receivers {
		receiver, ok := r.(*config.Sms)
		if !ok || receiver == nil {
			continue
		}

		if receiver.SmsConfig == nil {
			_ = level.Warn(logger).Log("msg", "SmsNotifier: ignore receiver because of empty config")
			continue
		}

		if utils.StringIsNil(receiver.Template) {
			receiver.Template = n.templateName
		}

		n.smss = append(n.smss, receiver)
	}

	return n
}

func (n *Notifier) Notify(ctx context.Context, data template.Data) []error {

	send := func(s *config.Sms) error {

		start := time.Now()
		defer func() {
			_ = level.Debug(n.logger).Log("msg", "SmsNotifier: send message", "used", time.Since(start).String())
		}()

		newData := utils.FilterAlerts(data, s.Selector, n.logger)
		if len(newData.Alerts) == 0 {
			return nil
		}

		if err := n.notifierCfg.EnqueueHistory(newData); err != nil {
			_ = level.Error(n.logger).Log("msg", "Notification history in queue error", "error", err.Error())
		}

		msg, err := n.template.TempleText(s.Template, newData, n.logger)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "SmsNotifier: generate message error", "error", err.Error())
			return err
		}

		// select an available provider function
		providerFunc, err := GetProviderFunc(s.SmsConfig.DefaultProvider)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "SmsNotifier: no available provider function", "error", err.Error())
			return err
		}

		// new a provider
		provider := providerFunc(n.notifierCfg, s.SmsConfig.Providers, s.PhoneNumbers)

		// make request by the provider
		ctx, cancel := context.WithTimeout(context.Background(), n.timeout)
		defer cancel()

		if err := provider.MakeRequest(ctx, msg); err != nil {
			_ = level.Error(n.logger).Log("msg", "SmsNotifier: send request failed", "error", err.Error())
			return err
		}
		_ = level.Info(n.logger).Log("msg", "SmsNotifier: send request successfully")

		return nil

	}

	group := async.NewGroup(ctx)
	for _, sms := range n.smss {
		s := sms
		group.Add(func(stopCh chan interface{}) {
			stopCh <- send(s)
		})
	}

	return group.Wait()
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
