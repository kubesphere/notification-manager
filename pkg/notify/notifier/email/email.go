package email

import (
	"context"
	"math"
	"strconv"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/pkg/async"
	nmconfig "github.com/kubesphere/notification-manager/pkg/config"
	"github.com/kubesphere/notification-manager/pkg/constants"
	"github.com/kubesphere/notification-manager/pkg/internal"
	"github.com/kubesphere/notification-manager/pkg/internal/email"
	"github.com/kubesphere/notification-manager/pkg/notify/notifier"
	"github.com/kubesphere/notification-manager/pkg/utils"
	amconfig "github.com/prometheus/alertmanager/config"
	"github.com/prometheus/alertmanager/notify"
	amemail "github.com/prometheus/alertmanager/notify/email"
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
	receivers   []*email.Receiver
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

func NewEmailNotifier(logger log.Logger, receivers []internal.Receiver, notifierCfg *nmconfig.Config) notifier.Notifier {

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
		tmplType:            constants.HTML,
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
		receiver, ok := r.(*email.Receiver)
		if !ok || receiver == nil {
			continue
		}

		if receiver.Config == nil {
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
				if receiver.TmplType == constants.HTML {
					receiver.Template = DefaultHTMLTemplate
				} else if receiver.TmplType == constants.Text {
					receiver.Template = DefaultTextTemplate
				}
			}
		}

		if utils.StringIsNil(receiver.TitleTemplate) {
			receiver.TitleTemplate = n.subjectTemplateName
		}

		n.receivers = append(n.receivers, receiver)
	}

	return n
}

func (n *Notifier) Notify(ctx context.Context, data template.Data) []error {

	sendEmail := func(r *email.Receiver, to string) error {

		start := time.Now()
		defer func() {
			_ = level.Debug(n.logger).Log("msg", "EmailNotifier: send message", "used", time.Since(start).String())
		}()

		var as []*types.Alert
		newData := utils.FilterAlerts(data, r.AlertSelector, n.logger)
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

		emailConfig, err := n.getEmailConfig(r)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "EmailNotifier: get email config error", "error", err.Error())
			return err
		}

		emailConfig.To = to

		if r.TmplType == constants.Text {
			emailConfig.Text = r.Template
		} else if r.TmplType == constants.HTML {
			emailConfig.HTML = r.Template
		} else {
			_ = level.Error(n.logger).Log("msg", "EmailNotifier: unknown message type", "type", r.TmplType)
			return utils.Errorf("Unknown message type, %s", r.TmplType)
		}

		emailConfig.HTML = r.Template
		emailConfig.Headers["Subject"] = r.TitleTemplate
		sender := amemail.New(emailConfig, n.template.Tmpl, n.logger)

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
	for _, receiver := range n.receivers {
		r := receiver
		for _, t := range r.To {
			to := t
			group.Add(func(stopCh chan interface{}) {
				stopCh <- sendEmail(r, to)
			})
		}
	}

	return group.Wait()
}

func (n *Notifier) getEmailConfig(r *email.Receiver) (*amconfig.EmailConfig, error) {

	ec := &amconfig.EmailConfig{
		From:  r.From,
		Hello: r.Hello,
		Smarthost: amconfig.HostPort{
			Host: r.SmartHost.Host,
			Port: strconv.Itoa(r.SmartHost.Port),
		},
		AuthUsername: r.AuthUsername,
		AuthPassword: "",
		AuthSecret:   "",
		AuthIdentity: r.AuthIdentify,
		RequireTLS:   &r.RequireTLS,
		Headers:      make(map[string]string),
	}

	if r.AuthPassword != nil {
		pass, err := n.notifierCfg.GetCredential(r.AuthPassword)
		if err != nil {
			return nil, err
		}

		ec.AuthPassword = amconfig.Secret(pass)
	}

	if r.AuthSecret != nil {
		secret, err := n.notifierCfg.GetCredential(r.AuthSecret)
		if err != nil {
			return nil, err
		}

		ec.AuthSecret = amconfig.Secret(secret)
	}

	if r.TLS != nil {
		tlsConfig := commoncfg.TLSConfig{
			InsecureSkipVerify: r.TLS.InsecureSkipVerify,
			ServerName:         r.TLS.ServerName,
		}

		// If a CA cert is provided then let's read it in,  so we can validate the
		// scrape target's certificate properly.
		if r.TLS.RootCA != nil {
			if ca, err := n.notifierCfg.GetCredential(r.TLS.RootCA); err != nil {
				return nil, err
			} else {
				tlsConfig.CAFile = ca
			}
		}

		// If a client cert & key is provided then configure TLS config accordingly.
		if r.TLS.ClientCertificate != nil {
			if r.TLS.Cert != nil && r.TLS.Key == nil {
				return nil, utils.Error("Client cert file specified without client key file")
			} else if r.TLS.Cert == nil && r.TLS.Key != nil {
				return nil, utils.Error("Client key file specified without client cert file")
			} else if r.TLS.Cert != nil && r.TLS.Key != nil {
				key, err := n.notifierCfg.GetCredential(r.TLS.Key)
				if err != nil {
					return nil, err
				}

				cert, err := n.notifierCfg.GetCredential(r.TLS.Cert)
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
