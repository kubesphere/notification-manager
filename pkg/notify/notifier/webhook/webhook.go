package webhook

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/pkg/constants"
	"github.com/kubesphere/notification-manager/pkg/controller"
	"github.com/kubesphere/notification-manager/pkg/internal"
	"github.com/kubesphere/notification-manager/pkg/internal/webhook"
	"github.com/kubesphere/notification-manager/pkg/notify/notifier"
	"github.com/kubesphere/notification-manager/pkg/template"
	"github.com/kubesphere/notification-manager/pkg/utils"
	"github.com/mwitkow/go-conntrack"
)

const (
	DefaultSendTimeout = time.Second * 5

	Status           = "status"
	StartsAt         = "startsAt"
	EndsAt           = "endsAt"
	NotificationTime = "notificationTime"
	RunbookURL       = "runbook_url"
	Message          = "message"
	Summary          = "summary"
	SummaryCn        = "summaryCn"
)

type Notifier struct {
	notifierCtl *controller.Controller
	receiver    *webhook.Receiver
	timeout     time.Duration
	logger      log.Logger
	tmpl        *template.Template

	sentSuccessfulHandler *func([]*template.Alert)
}

func NewWebhookNotifier(logger log.Logger, receiver internal.Receiver, notifierCtl *controller.Controller) (notifier.Notifier, error) {

	n := &Notifier{
		notifierCtl: notifierCtl,
		timeout:     DefaultSendTimeout,
		logger:      logger,
	}

	opts := notifierCtl.ReceiverOpts
	tmplName := constants.DefaultWebhookTemplate
	if opts != nil && opts.Global != nil && !utils.StringIsNil(opts.Global.Template) {
		tmplName = opts.Global.Template
	}

	if opts != nil && opts.Webhook != nil {

		if opts.Webhook.NotificationTimeout != nil {
			n.timeout = time.Second * time.Duration(*opts.Webhook.NotificationTimeout)
		}

		if !utils.StringIsNil(opts.Webhook.Template) {
			tmplName = opts.Webhook.Template
		}
	}

	n.receiver = receiver.(*webhook.Receiver)
	if utils.StringIsNil(n.receiver.TmplName) {
		n.receiver.TmplName = tmplName
	}

	var err error
	n.tmpl, err = notifierCtl.GetReceiverTmpl(n.receiver.TmplText)
	if err != nil {
		_ = level.Error(n.logger).Log("msg", "WebhookNotifier: create receiver template error", "error", err.Error())
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
		_ = level.Debug(n.logger).Log("msg", "WebhookNotifier: send message", "used", time.Since(start).String())
	}()

	var buf bytes.Buffer
	if n.tmpl.Transform(n.receiver.TmplName) == constants.DefaultWebhookTemplate {
		if err := utils.JsonEncode(&buf, data); err != nil {
			_ = level.Error(n.logger).Log("msg", "WebhookNotifier: encode message error", "error", err.Error())
			return err
		}
	} else if n.tmpl.Transform(n.receiver.TmplName) == constants.DefaultHistoryTemplate {
		// just for notification history
		if err := generateNotificationHistory(&buf, data); err != nil {
			_ = level.Error(n.logger).Log("msg", "WebhookNotifier: generate notification history error", "error", err.Error())
			return err
		}
	} else {
		msg, err := n.tmpl.Text(n.receiver.TmplName, data)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "WebhookNotifier: generate message error", "error", err.Error())
			return err
		}

		buf.WriteString(msg)
	}

	request, err := http.NewRequest(http.MethodPost, n.receiver.URL, &buf)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")

	if n.receiver.HttpConfig != nil {
		if n.receiver.HttpConfig.BearerToken != nil {
			bearer, err := n.notifierCtl.GetCredential(n.receiver.HttpConfig.BearerToken)
			if err != nil {
				_ = level.Error(n.logger).Log("msg", "WebhookNotifier: get bearer token error", "error", err.Error())
				return err
			}

			request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", bearer))
		} else if n.receiver.HttpConfig.BasicAuth != nil {
			pass := ""
			if n.receiver.HttpConfig.BasicAuth.Password != nil {
				p, err := n.notifierCtl.GetCredential(n.receiver.HttpConfig.BasicAuth.Password)
				if err != nil {
					_ = level.Error(n.logger).Log("msg", "WebhookNotifier: get password error", "error", err.Error())
					return err
				}

				pass = p
			}
			request.SetBasicAuth(n.receiver.HttpConfig.BasicAuth.Username, pass)
		}
	}

	transport, err := n.getTransport(n.receiver)
	if err != nil {
		_ = level.Error(n.logger).Log("msg", "WebhookNotifier: get transport error", "error", err.Error())
		return err
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   n.timeout,
	}

	_, err = utils.DoHttpRequest(ctx, client, request)
	if err != nil {
		_ = level.Error(n.logger).Log("msg", "WebhookNotifier: do http request error", "error", err.Error())
		return err
	}

	if n.sentSuccessfulHandler != nil {
		(*n.sentSuccessfulHandler)(data.Alerts)
	}

	_ = level.Debug(n.logger).Log("msg", "WebhookNotifier: send message", "to", n.receiver.URL)
	return nil
}

func generateNotificationHistory(buf *bytes.Buffer, data *template.Data) error {
	for _, alert := range data.Alerts {
		m := make(map[string]interface{})
		m[Status] = alert.Status
		m[StartsAt] = alert.StartsAt
		m[EndsAt] = alert.EndsAt
		m[NotificationTime] = time.Now()
		m[constants.AlertMessage] = alert.Message()

		if alert.Labels != nil {
			for k, v := range alert.Labels {
				m[k] = v
			}
		}

		if alert.Annotations != nil {
			for _, p := range alert.Annotations.SortedPairs().DefaultFilter() {
				m[p.Name] = p.Value
			}
		}

		if err := utils.JsonEncode(buf, m); err != nil {
			return err
		}
	}

	return nil
}

func (n *Notifier) getTransport(r *webhook.Receiver) (http.RoundTripper, error) {

	transport := &http.Transport{
		DisableKeepAlives:  false,
		DisableCompression: true,
		DialContext: conntrack.NewDialContextFunc(
			conntrack.DialWithTracing(),
			conntrack.DialWithName(r.URL),
		),
	}

	if c := r.HttpConfig; c != nil {

		if c.TLSConfig != nil {
			tlsConfig := &tls.Config{InsecureSkipVerify: c.TLSConfig.InsecureSkipVerify}

			// If a CA cert is provided then let's read it in, so we can validate the
			// scrape target's certificate properly.
			if c.TLSConfig.RootCA != nil {
				if ca, err := n.notifierCtl.GetCredential(c.TLSConfig.RootCA); err != nil {
					return nil, err
				} else {
					caCertPool := x509.NewCertPool()
					if !caCertPool.AppendCertsFromPEM([]byte(ca)) {
						return nil, err
					}
					tlsConfig.RootCAs = caCertPool
				}
			}

			if !utils.StringIsNil(c.TLSConfig.ServerName) {
				tlsConfig.ServerName = c.TLSConfig.ServerName
			}

			// If a client cert & key is provided then configure TLS config accordingly.
			if c.TLSConfig.ClientCertificate != nil {
				if c.TLSConfig.Cert != nil && c.TLSConfig.Key == nil {
					return nil, utils.Error("Client cert file specified without client key file")
				} else if c.TLSConfig.Cert == nil && c.TLSConfig.Key != nil {
					return nil, utils.Error("Client key file specified without client cert file")
				} else if c.TLSConfig.Cert != nil && c.TLSConfig.Key != nil {
					key, err := n.notifierCtl.GetCredential(c.TLSConfig.Key)
					if err != nil {
						return nil, err
					}

					cert, err := n.notifierCtl.GetCredential(c.TLSConfig.Cert)
					if err != nil {
						return nil, err
					}

					tlsCert, err := tls.X509KeyPair([]byte(cert), []byte(key))
					if err != nil {
						return nil, err
					}
					tlsConfig.Certificates = []tls.Certificate{tlsCert}
				}
			}

			transport.TLSClientConfig = tlsConfig
		}

		if !utils.StringIsNil(c.ProxyURL) {
			var proxy func(*http.Request) (*url.URL, error)
			if u, err := url.Parse(c.ProxyURL); err != nil {
				return nil, err
			} else {
				proxy = http.ProxyURL(u)
			}

			transport.Proxy = proxy
		}
	}

	return transport, nil
}
