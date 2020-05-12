package email

import (
	"context"
	"fmt"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	nmv1alpha1 "github.com/kubesphere/notification-manager/pkg/apis/v1alpha1"
	nmconfig "github.com/kubesphere/notification-manager/pkg/notify/config"
	"github.com/kubesphere/notification-manager/pkg/notify/notifier"
	"github.com/prometheus/alertmanager/config"
	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/notify/email"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
	"math"
	"net/url"
	"strings"
	"time"
)

const (
	Bulk               = "Bulk"
	MaxEmailReceivers  = math.MaxInt32
	DefaultSendTimeout = time.Second * 3
)

type Notifier struct {
	email    map[string]*nmconfig.Email
	template *template.Template
	timeout  time.Duration
	logger   log.Logger
	// Email delivery type, single or bulk
	delivery string
	// The maximum size of receivers in one email.
	maxEmailReceivers int
}

func NewEmailNotifier(logger log.Logger, val interface{}, opts *nmv1alpha1.Options) notifier.Notifier {
	sv, ok := val.([]interface{})
	if !ok {
		_ = level.Error(logger).Log("msg", "EmailNotifier: value type error")
		return nil
	}

	n := &Notifier{
		email:             make(map[string]*nmconfig.Email),
		logger:            logger,
		timeout:           DefaultSendTimeout,
		delivery:          Bulk,
		maxEmailReceivers: MaxEmailReceivers}

	tmpl, err := template.FromGlobs()
	if err != nil {
		_ = level.Error(n.logger).Log("msg", "EmailNotifier: template error", "error", err.Error())
		return nil
	}
	n.template = tmpl

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
	}

	for _, v := range sv {
		ev, ok := v.(*nmconfig.Email)
		if !ok || ev == nil {
			_ = level.Error(logger).Log("msg", "EmailNotifier: value type error")
			continue
		}

		if n.delivery == Bulk {
			c := n.clone(ev.EmailConfig)
			key, err := notifier.Md5key(c)
			if err != nil {
				_ = level.Error(logger).Log("msg", "EmailNotifier: get notifier error", "error", err.Error())
				continue
			}

			e, ok := n.email[key]
			if !ok {
				e = &nmconfig.Email{
					EmailConfig: c,
				}
			}

			e.To = append(e.To, ev.To...)
			n.email[key] = e
		} else {
			key, err := notifier.Md5key(ev)
			if err != nil {
				_ = level.Error(logger).Log("msg", "EmailNotifier: get notifier error", "error", err.Error())
				continue
			}

			n.email[key] = &nmconfig.Email{
				To:          ev.To,
				EmailConfig: n.clone(ev.EmailConfig),
			}
		}
	}

	return n
}

func (n *Notifier) Notify(data template.Data) []error {
	n.template.ExternalURL, _ = url.Parse(data.ExternalURL)

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

	var errs []error
	sendEmail := func(c *config.EmailConfig, to string) {
		cc := n.clone(c)
		cc.To = to
		cc.HTML = `{{ template "email.default.html" . }}`
		e := email.New(cc, n.template, n.logger)

		ctx, cancel := context.WithTimeout(context.Background(), n.timeout)
		ctx = notify.WithGroupLabels(ctx, notifier.KvToLabelSet(data.GroupLabels))
		ctx = notify.WithReceiverName(ctx, data.Receiver)
		defer cancel()

		_, err := e.Notify(ctx, as...)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "EmailNotifier: notify error", "from", cc.From, "to", cc.To, "error", err.Error())
			errs = append(errs, err)
		}
		_ = level.Debug(n.logger).Log("msg", "EmailNotifier: send email", "from", cc.From, "to", cc.To)
	}

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
				sendEmail(e.EmailConfig, to)
			}
		} else {
			for _, to := range e.To {
				sendEmail(e.EmailConfig, to)
			}
		}
	}

	return errs
}

func (n *Notifier) clone(ec *config.EmailConfig) *config.EmailConfig {

	if ec == nil {
		return nil
	}

	emailConfig := &config.EmailConfig{
		NotifierConfig: config.NotifierConfig{},
		To:             "",
		From:           ec.From,
		Hello:          ec.Hello,
		Smarthost:      ec.Smarthost,
		AuthUsername:   ec.AuthUsername,
		AuthPassword:   ec.AuthPassword,
		AuthSecret:     ec.AuthSecret,
		AuthIdentity:   ec.AuthIdentity,
		Headers:        make(map[string]string),
		HTML:           ec.HTML,
		Text:           ec.Text,
		RequireTLS:     &(*ec.RequireTLS),
		TLSConfig:      ec.TLSConfig,
	}

	return emailConfig
}
