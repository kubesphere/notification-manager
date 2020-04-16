package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	notifyconfig "github.com/kubesphere/notification-manager/pkg/notify/config"
	"github.com/prometheus/alertmanager/notify/email"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
	"net/url"
	"time"
)

const (
	SendTimeout = time.Second * 3
)

type EmailNotifier struct {
	Email  *notifyconfig.Email
	logger log.Logger
}

func init() {
	Register("Email", NewEmailNotifier)
}

func NewEmailNotifier(logger log.Logger, val interface{}) Notifier {

	emailConfig, ok := val.(*notifyconfig.Email)
	if !ok {
		_ = level.Error(logger).Log("msg", "Notifier: value type error")
		return nil
	}

	notifier := &EmailNotifier{logger: logger, Email: emailConfig}
	if notifier.Email.EmailConfig.Headers == nil {
		notifier.Email.EmailConfig.Headers = make(map[string]string)
	}
	return notifier
}

func (en *EmailNotifier) Notify(alert template.Alert) []error {
	bs, _ := json.Marshal(alert)
	var out bytes.Buffer
	err := json.Indent(&out, bs, "", "\t")

	if err != nil {
		_ = level.Error(en.logger).Log("msg", "Notifier: marshal error", "error", err.Error())
		return []error{err}
	}
	en.Email.EmailConfig.HTML = string(bs)
	en.Email.EmailConfig.Headers["Subject"] = alert.Labels["message"]
	if len(en.Email.EmailConfig.Headers["Subject"]) == 0 {
		en.Email.EmailConfig.Headers["Subject"] = fmt.Sprintf("a %s alert", alert.Status)
	}

	tmpl, err := template.FromGlobs()
	if err != nil {
		_ = level.Error(en.logger).Log("msg", "Notifier: template error", "error", err.Error())
		return []error{err}
	}
	tmpl.ExternalURL, _ = url.Parse("http://kubesphere.io")

	var errs []error
	for _, to := range en.Email.To {
		en.Email.EmailConfig.To = to
		email2 := email.New(en.Email.EmailConfig, tmpl, log.NewNopLogger())
		ctx, cancel := context.WithTimeout(context.Background(), SendTimeout)
		_, err = email2.Notify(ctx, &types.Alert{
			Alert: model.Alert{
				Labels:   model.LabelSet{},
				StartsAt: time.Now(),
				EndsAt:   time.Now().Add(time.Hour),
			},
		})
		cancel()
		_ = level.Info(en.logger).Log("Notifier: send email to", to)
		if err != nil {
			_ = level.Error(en.logger).Log("msg", "Notifier: email notify error", "address", to, "error", err.Error())
			errs = append(errs, err)
		}
	}

	return errs
}
