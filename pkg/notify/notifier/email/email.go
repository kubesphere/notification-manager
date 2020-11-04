package email

import (
	"context"
	"fmt"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/pkg/async"
	nmconfig "github.com/kubesphere/notification-manager/pkg/notify/config"
	"github.com/kubesphere/notification-manager/pkg/notify/notifier"
	"github.com/prometheus/alertmanager/config"
	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/notify/email"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
	"math"
	"strings"
	"time"
)

const (
	Bulk                    = "Bulk"
	MaxEmailReceivers       = math.MaxInt32
	DefaultSendTimeout      = time.Second * 3
	DefaultTemplate         = `{{ template "nm.default.html" . }}`
	DefaultTSubjectTemplate = `{{ template "nm.default.subject" . }}`
)

type Notifier struct {
	notifierCfg *nmconfig.Config
	email       map[string]*nmconfig.Email
	template    *notifier.Template
	// The name of template to generate email message.
	templateName string
	// The name of template to generate email subject.
	subjectTemplateName string
	timeout             time.Duration
	logger              log.Logger
	// Email delivery type, single or bulk.
	delivery string
	// The maximum size of receivers in one email.
	maxEmailReceivers int
}

func NewEmailNotifier(logger log.Logger, receivers []nmconfig.Receiver, notifierCfg *nmconfig.Config) notifier.Notifier {

	var path []string
	opts := notifierCfg.ReceiverOpts
	if opts != nil && opts.Global != nil {
		path = opts.Global.TemplateFiles
	}
	tmpl, err := notifier.NewTemplate(path)
	if err != nil {
		_ = level.Error(logger).Log("msg", "EmailNotifier: get template error", "error", err.Error())
		return nil
	}

	n := &Notifier{
		notifierCfg:         notifierCfg,
		email:               make(map[string]*nmconfig.Email),
		logger:              logger,
		timeout:             DefaultSendTimeout,
		delivery:            Bulk,
		maxEmailReceivers:   MaxEmailReceivers,
		template:            tmpl,
		templateName:        DefaultTemplate,
		subjectTemplateName: DefaultTSubjectTemplate,
	}

	if opts != nil && opts.Email != nil {
		if opts.Email.NotificationTimeout != nil {
			n.timeout = time.Second * time.Duration(*opts.Email.NotificationTimeout)
		}

		if opts.Email.MaxEmailReceivers > 0 {
			n.maxEmailReceivers = opts.Email.MaxEmailReceivers
		}

		if len(opts.Email.DeliveryType) > 0 {
			n.delivery = opts.Email.DeliveryType
		}

		if len(opts.Email.Template) > 0 {
			n.templateName = opts.Email.Template
		} else if opts.Global != nil && len(opts.Global.Template) > 0 {
			n.templateName = opts.Global.Template
		}

		if len(opts.Email.SubjectTemplate) > 0 {
			n.subjectTemplateName = opts.Email.SubjectTemplate
		}
	}

	for _, r := range receivers {
		receiver, ok := r.(*nmconfig.Email)
		if !ok || receiver == nil {
			continue
		}

		if receiver.EmailConfig == nil {
			_ = level.Warn(logger).Log("msg", "EmailNotifier: ignore receiver because of empty config")
			continue
		}

		if n.delivery == Bulk {
			c := n.clone(receiver.EmailConfig)
			key, err := notifier.Md5key(c)
			if err != nil {
				_ = level.Error(logger).Log("msg", "EmailNotifier: get notifier error", "error", err.Error())
				continue
			}

			e, ok := n.email[key]
			if !ok {
				e = nmconfig.NewEmail(nil)
				_ = e.SetConfig(c)
				e.SetNamespace(receiver.GetNamespace())
			}

			e.To = append(e.To, receiver.To...)
			n.email[key] = e
		} else {
			key, err := notifier.Md5key(receiver)
			if err != nil {
				_ = level.Error(logger).Log("msg", "EmailNotifier: get notifier error", "error", err.Error())
				continue
			}

			e := nmconfig.NewEmail(receiver.To)
			_ = e.SetConfig(n.clone(receiver.EmailConfig))
			e.SetNamespace(receiver.GetNamespace())
			n.email[key] = e
		}
	}

	return n
}

func (n *Notifier) Notify(ctx context.Context, data template.Data) []error {

	var as []*types.Alert
	for _, a := range data.Alerts {
		as = append(as, &types.Alert{
			Alert: model.Alert{
				Labels:       notifier.KvToLabelSet(a.Labels),
				Annotations:  notifier.KvToLabelSet(a.Annotations),
				StartsAt:     a.StartsAt,
				EndsAt:       a.EndsAt,
				GeneratorURL: a.GeneratorURL,
			},
		})
	}

	sendEmail := func(e *nmconfig.Email, to string) error {

		start := time.Now()
		defer func() {
			_ = level.Debug(n.logger).Log("msg", "EmailNotifier: send message", "used", time.Since(start).String())
		}()

		emailConfig, err := n.getEmailConfig(e)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "EmailNotifier: get email config error", "error", err.Error())
			return err
		}
		emailConfig.To = to
		emailConfig.HTML = n.templateName
		emailConfig.Headers["Subject"] = n.subjectTemplateName
		sender := email.New(emailConfig, n.template.Tmpl, n.logger)

		ctx, cancel := context.WithTimeout(context.Background(), n.timeout)
		ctx = notify.WithGroupLabels(ctx, notifier.KvToLabelSet(data.GroupLabels))
		ctx = notify.WithReceiverName(ctx, data.Receiver)
		defer cancel()

		_, err = sender.Notify(ctx, as...)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "EmailNotifier: notify error", "from", emailConfig.From, "to", emailConfig.To, "error", err.Error())
			return err
		}
		_ = level.Debug(n.logger).Log("msg", "EmailNotifier: send message", "from", emailConfig.From, "to", emailConfig.To)
		return nil
	}

	group := async.NewGroup(ctx)
	for _, e := range n.email {
		if n.delivery == Bulk {
			size := 0
			for {
				if size >= len(e.To) {
					break
				}

				var sub []string
				if size+n.maxEmailReceivers > len(e.To) {
					sub = e.To[size:]
				} else {
					sub = e.To[size : size+n.maxEmailReceivers]
				}

				size += n.maxEmailReceivers

				to := ""
				for _, t := range sub {
					to += fmt.Sprintf("%s,", t)
				}
				to = strings.TrimSuffix(to, ",")
				group.Add(func(stopCh chan interface{}) {
					stopCh <- sendEmail(e, to)
				})
			}
		} else {
			for _, t := range e.To {
				to := t
				group.Add(func(stopCh chan interface{}) {
					stopCh <- sendEmail(e, to)
				})
			}
		}
	}

	return group.Wait()
}

func (n *Notifier) clone(ec *nmconfig.EmailConfig) *nmconfig.EmailConfig {

	if ec == nil {
		return nil
	}

	return &nmconfig.EmailConfig{
		From:         ec.From,
		SmartHost:    ec.SmartHost,
		Hello:        ec.Hello,
		AuthUsername: ec.AuthUsername,
		AuthIdentify: ec.AuthIdentify,
		AuthPassword: ec.AuthPassword,
		AuthSecret:   ec.AuthSecret,
		RequireTLS:   ec.RequireTLS,
	}
}

func (n *Notifier) getEmailConfig(e *nmconfig.Email) (*config.EmailConfig, error) {

	ec := &config.EmailConfig{
		From:  e.EmailConfig.From,
		Hello: e.EmailConfig.Hello,
		Smarthost: config.HostPort{
			Host: e.EmailConfig.SmartHost.Host,
			Port: e.EmailConfig.SmartHost.Port,
		},
		AuthUsername: e.EmailConfig.AuthUsername,
		AuthPassword: "",
		AuthSecret:   "",
		AuthIdentity: e.EmailConfig.AuthIdentify,
		RequireTLS:   &e.EmailConfig.RequireTLS,
		Headers:      make(map[string]string),
	}

	if e.EmailConfig.AuthPassword != nil {
		pass, err := n.notifierCfg.GetSecretData(e.GetNamespace(), e.EmailConfig.AuthPassword)
		if err != nil {
			return nil, err
		}

		ec.AuthPassword = config.Secret(pass)
	}

	if e.EmailConfig.AuthSecret != nil {
		secret, err := n.notifierCfg.GetSecretData(e.GetNamespace(), e.EmailConfig.AuthSecret)
		if err != nil {
			return nil, err
		}

		ec.AuthSecret = config.Secret(secret)
	}

	return ec, nil
}
