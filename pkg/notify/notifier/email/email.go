package email

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/pkg/async"
	nmconfig "github.com/kubesphere/notification-manager/pkg/notify/config"
	"github.com/kubesphere/notification-manager/pkg/notify/notifier"
	"github.com/kubesphere/notification-manager/pkg/utils"
	"github.com/prometheus/alertmanager/config"
	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/notify/email"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/alertmanager/types"
	commoncfg "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
)

const (
	MaxEmailReceivers       = math.MaxInt32
	DefaultSendTimeout      = time.Second * 3
	DefaultHTMLTemplate     = `{{ template "nm.default.html" . }}`
	DefaultTextTemplate     = `{{ template "nm.default.text" . }}`
	DefaultTSubjectTemplate = `{{ template "nm.default.subject" . }}`
)

type Notifier struct {
	notifierCfg *nmconfig.Config
	email       []*nmconfig.Email
	template    *notifier.Template
	// The name of template to generate email message.
	templateName string
	// The name of template to generate email subject.
	subjectTemplateName string
	tmplType            string
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
		logger:              logger,
		timeout:             DefaultSendTimeout,
		maxEmailReceivers:   MaxEmailReceivers,
		template:            tmpl,
		subjectTemplateName: DefaultTSubjectTemplate,
		tmplType:            nmconfig.HTML,
	}

	if opts != nil && opts.Global != nil && !utils.StringIsNil(opts.Global.Template) {
		n.templateName = opts.Global.Template
	}

	if opts != nil && opts.Email != nil {
		if opts.Email.NotificationTimeout != nil {
			n.timeout = time.Second * time.Duration(*opts.Email.NotificationTimeout)
		}

		if opts.Email.MaxEmailReceivers > 0 {
			n.maxEmailReceivers = opts.Email.MaxEmailReceivers
		}

		if !utils.StringIsNil(opts.Email.Template) {
			n.templateName = opts.Email.Template
		}

		if !utils.StringIsNil(opts.Email.TmplType) {
			n.tmplType = opts.Email.TmplType
		}

		if !utils.StringIsNil(opts.Email.SubjectTemplate) {
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

		if utils.StringIsNil(receiver.TmplType) {
			receiver.TmplType = n.tmplType
		}

		if utils.StringIsNil(receiver.Template) {
			if n.templateName != "" {
				receiver.Template = n.templateName
			} else {
				if receiver.TmplType == nmconfig.HTML {
					receiver.Template = DefaultHTMLTemplate
				} else if receiver.TmplType == nmconfig.Text {
					receiver.Template = DefaultTextTemplate
				}
			}
		}

		if utils.StringIsNil(receiver.SubjectTemplate) {
			receiver.SubjectTemplate = n.subjectTemplateName
		}

		e := nmconfig.NewEmail(receiver)
		_ = e.SetConfig(n.clone(receiver.EmailConfig))
		n.email = append(n.email, e)
	}

	return n
}

func (n *Notifier) Notify(ctx context.Context, data template.Data) []error {

	sendEmail := func(e *nmconfig.Email, to string) error {

		start := time.Now()
		defer func() {
			_ = level.Debug(n.logger).Log("msg", "EmailNotifier: send message", "used", time.Since(start).String())
		}()

		var as []*types.Alert
		newData := utils.FilterAlerts(data, e.Selector, n.logger)
		if len(newData.Alerts) == 0 {
			return nil
		}

		for _, a := range newData.Alerts {
			as = append(as, &types.Alert{
				Alert: model.Alert{
					Labels:       utils.KvToLabelSet(a.Labels),
					Annotations:  utils.KvToLabelSet(a.Annotations),
					StartsAt:     a.StartsAt,
					EndsAt:       a.EndsAt,
					GeneratorURL: a.GeneratorURL,
				},
			})
		}

		emailConfig, err := n.getEmailConfig(e)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "EmailNotifier: get email config error", "error", err.Error())
			return err
		}

		emailConfig.To = to

		if e.TmplType == nmconfig.Text {
			emailConfig.Text = e.Template
		} else if e.TmplType == nmconfig.HTML {
			emailConfig.HTML = e.Template
		} else {
			err = fmt.Errorf("unkown message type, %s", e.TmplType)
			_ = level.Error(n.logger).Log("msg", "EmailNotifier: get email config error", "error", err.Error())
			return err
		}

		emailConfig.HTML = e.Template
		emailConfig.Headers["Subject"] = e.SubjectTemplate
		sender := email.New(emailConfig, n.template.Tmpl, n.logger)

		ctx, cancel := context.WithTimeout(context.Background(), n.timeout)
		ctx = notify.WithGroupLabels(ctx, utils.KvToLabelSet(data.GroupLabels))
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
		for _, t := range e.To {
			to := t
			group.Add(func(stopCh chan interface{}) {
				stopCh <- sendEmail(e, to)
			})
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
		TLS:          ec.TLS,
	}
}

func (n *Notifier) getEmailConfig(e *nmconfig.Email) (*config.EmailConfig, error) {

	ec := &config.EmailConfig{
		From:  e.EmailConfig.From,
		Hello: e.EmailConfig.Hello,
		Smarthost: config.HostPort{
			Host: e.EmailConfig.SmartHost.Host,
			Port: strconv.Itoa(e.EmailConfig.SmartHost.Port),
		},
		AuthUsername: e.EmailConfig.AuthUsername,
		AuthPassword: "",
		AuthSecret:   "",
		AuthIdentity: e.EmailConfig.AuthIdentify,
		RequireTLS:   &e.EmailConfig.RequireTLS,
		Headers:      make(map[string]string),
	}

	if e.EmailConfig.AuthPassword != nil {
		pass, err := n.notifierCfg.GetCredential(e.EmailConfig.AuthPassword)
		if err != nil {
			return nil, err
		}

		ec.AuthPassword = config.Secret(pass)
	}

	if e.EmailConfig.AuthSecret != nil {
		secret, err := n.notifierCfg.GetCredential(e.EmailConfig.AuthSecret)
		if err != nil {
			return nil, err
		}

		ec.AuthSecret = config.Secret(secret)
	}

	if e.EmailConfig.TLS != nil {
		tlsConfig := commoncfg.TLSConfig{
			InsecureSkipVerify: e.EmailConfig.TLS.InsecureSkipVerify,
			ServerName:         e.EmailConfig.TLS.ServerName,
		}

		// If a CA cert is provided then let's read it in so we can validate the
		// scrape target's certificate properly.
		if e.EmailConfig.TLS.RootCA != nil {
			if ca, err := n.notifierCfg.GetCredential(e.EmailConfig.TLS.RootCA); err != nil {
				return nil, err
			} else {
				tlsConfig.CAFile = ca
			}
		}

		// If a client cert & key is provided then configure TLS config accordingly.
		if e.EmailConfig.TLS.ClientCertificate != nil {
			if e.EmailConfig.TLS.Cert != nil && e.EmailConfig.TLS.Key == nil {
				return nil, fmt.Errorf("client cert file specified without client key file")
			} else if e.EmailConfig.TLS.Cert == nil && e.EmailConfig.TLS.Key != nil {
				return nil, fmt.Errorf("client key file specified without client cert file")
			} else if e.EmailConfig.TLS.Cert != nil && e.EmailConfig.TLS.Key != nil {
				key, err := n.notifierCfg.GetCredential(e.EmailConfig.TLS.Key)
				if err != nil {
					return nil, err
				}

				cert, err := n.notifierCfg.GetCredential(e.EmailConfig.TLS.Cert)
				if err != nil {
					return nil, err
				}

				tlsConfig.KeyFile = key
				tlsConfig.CertFile = cert
			}
		}

		ec.TLSConfig = tlsConfig
	}

	return ec, nil
}
