package sms

import (
	"bytes"
	"context"
	"errors"
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
	DefaultTemplate    = `{{ template "nm.default.message" . }}`
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

	if opts != nil && opts.Sms != nil {

		if opts.Sms.NotificationTimeout != nil {
			n.timeout = time.Second * time.Duration(*opts.Sms.NotificationTimeout)
		}

		if len(opts.Sms.Template) > 0 {
			n.templateName = opts.Sms.Template
		} else if opts.Global != nil && len(opts.Global.Template) > 0 {
			n.templateName = opts.Global.Template
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
		var value interface{} = newData
		if n.templateName != DefaultTemplate {
			msg, err := n.template.TempleText(n.templateName, newData, n.logger)
			if err != nil {
				_ = level.Error(n.logger).Log("msg", "SmsNotifier: generate message error", "error", err.Error())
				return err
			}

			value = msg
		}

		var buf bytes.Buffer
		if err := utils.JsonEncode(&buf, value); err != nil {
			_ = level.Error(n.logger).Log("msg", "SmsNotifier: encode message error", "error", err.Error())
			return err
		}
		messages := buf.String()

		// select an available provider function
		var selectedProviderFunc ProviderFactory
		if s.SmsConfig.DefaultProvider != "" {
			// check whether the default provider is registered
			p, ok := GetProviderFunc(s.SmsConfig.DefaultProvider)
			if !ok {
				err := errors.New("the given default sms provider not registered")
				_ = level.Error(n.logger).Log("msg", "SmsNotifier: encode message error", "error", err.Error())
				return err
			}
			selectedProviderFunc = p
		} else {
			p, err := GetFirstAvailableProviderFunc()
			if err != nil {
				_ = level.Error(n.logger).Log("msg", "SmsNotifier: no availavble sms provider support", "error", err.Error())
				return err
			}
			selectedProviderFunc = p
		}

		// new a provider
		provider := selectedProviderFunc(n.notifierCfg, s.SmsConfig.Providers, s.PhoneNumbers)

		// make request by the provider
		ctx, cancel := context.WithTimeout(context.Background(), n.timeout)
		defer cancel()

		if err := provider.MakeRequest(ctx, messages); err != nil {
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
