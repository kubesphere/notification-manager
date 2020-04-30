package email

import (
	"context"
	"encoding/json"
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
	"net/url"
	"strings"
	"time"
)

const (
	DefaultSendTimeout     = time.Second * 3
	Bulk                   = "Bulk"
	DefaultReceiversLimits = 2
)

type Notifier struct {
	emails   map[string]*nmconfig.Email
	template *template.Template
	timeout  time.Duration
	logger   log.Logger
	// Email delivery type, single or bulk
	delivery        string
	receiversLimits int
}

func NewEmailNotifier(logger log.Logger, val interface{}, opts *nmv1alpha1.Options) notifier.Notifier {
	sv, ok := val.([]interface{})
	if !ok {
		_ = level.Error(logger).Log("msg", "Notifier: value type error")
		return nil
	}

	n := &Notifier{
		emails:          make(map[string]*nmconfig.Email),
		logger:          logger,
		timeout:         DefaultSendTimeout,
		delivery:        "",
		receiversLimits: DefaultReceiversLimits}

	tmpl, err := template.FromGlobs()
	if err != nil {
		_ = level.Error(n.logger).Log("msg", "Notifier: template error", "error", err.Error())
		return nil
	}
	n.template = tmpl

	if opts != nil && opts.NotificationTimeout != nil && opts.NotificationTimeout.Email != nil {
		n.timeout = time.Second * time.Duration(*opts.NotificationTimeout.Email)
	}

	if n.delivery == Bulk {
		for _, v := range sv {
			receiver, ok := v.(*nmconfig.Email)
			if !ok {
				_ = level.Error(logger).Log("msg", "Notifier: value type error")
				continue
			}
			c := n.clone(receiver.EmailConfig)
			notifier.JsonOut(c)
			key, err := notifier.Md5key(c)
			if err != nil {
				_ = level.Error(logger).Log("msg", "Notifier: get notifier error", "error", err.Error())
				continue
			}
			fmt.Println(key)
			e, ok := n.emails[key]
			if !ok {
				e = &nmconfig.Email{
					EmailConfig: c,
				}
			}

			e.To = append(e.To, receiver.To...)
			n.emails[key] = e
			notifier.JsonOut(n.emails)
		}
	} else {
		for _, v := range sv {
			receiver, ok := v.(*nmconfig.Email)
			if !ok {
				_ = level.Error(logger).Log("msg", "Notifier: value type error")
				continue
			}

			key, err := notifier.Md5key(receiver)
			if err != nil {
				_ = level.Error(logger).Log("msg", "Notifier: get notifier error", "error", err.Error())
				continue
			}

			n.emails[key] = &nmconfig.Email{
				To:          receiver.To,
				EmailConfig: n.clone(receiver.EmailConfig),
			}
		}
	}

	return n
}

func (n *Notifier) Notify(data template.Data) []error {
	bs, _ := json.Marshal(n)
	fmt.Println(string(bs))
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
		c.To = to
		c.HTML = `{{ template "email.default.html" . }}`
		e := email.New(c, n.template, n.logger)

		ctx, cancel := context.WithTimeout(context.Background(), n.timeout)
		ctx = notify.WithGroupLabels(ctx, notifier.KvToLabelSet(data.GroupLabels))
		ctx = notify.WithReceiverName(ctx, data.Receiver)
		defer cancel()

		_, err := e.Notify(ctx, as...)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "EmailNotifier: notify error", "subject", c.Headers["Subject"], "from", c.From, "to", c.To, "error", err.Error())
			errs = append(errs, err)
		}
		_ = level.Debug(n.logger).Log("msg", "EmailNotifier: notify error", "from", c.From, "to", c.To)
	}

	for _, e := range n.emails {
		if n.delivery == Bulk {
			size := 0
			for {
				if size >= len(e.To) {
					break
				}

				var sub []string
				if size+n.receiversLimits > len(e.To) {
					sub = e.To[size:]
				} else {
					sub = e.To[size : size+n.receiversLimits]
				}

				size += n.receiversLimits

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
