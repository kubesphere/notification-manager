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
	"github.com/kubesphere/notification-manager/pkg/async"
	"github.com/kubesphere/notification-manager/pkg/controller"
	"github.com/kubesphere/notification-manager/pkg/internal"
	"github.com/kubesphere/notification-manager/pkg/internal/webhook"
	"github.com/kubesphere/notification-manager/pkg/notify/notifier"
	"github.com/kubesphere/notification-manager/pkg/utils"
	"github.com/mwitkow/go-conntrack"
)

const (
	DefaultSendTimeout = time.Second * 5
	DefaultTemplate    = `{{ template "webhook.default.message" . }}`
)

type Notifier struct {
	notifierCtl  *controller.Controller
	receivers    []*webhook.Receiver
	timeout      time.Duration
	logger       log.Logger
	template     *notifier.Template
	templateName string
}

func NewWebhookNotifier(logger log.Logger, receivers []internal.Receiver, notifierCtl *controller.Controller) notifier.Notifier {

	var path []string
	opts := notifierCtl.ReceiverOpts
	if opts != nil && opts.Global != nil {
		path = opts.Global.TemplateFiles
	}
	tmpl, err := notifier.NewTemplate(path)
	if err != nil {
		_ = level.Error(logger).Log("msg", "WebhookNotifier: get template error", "error", err.Error())
		return nil
	}

	n := &Notifier{
		notifierCtl:  notifierCtl,
		timeout:      DefaultSendTimeout,
		logger:       logger,
		template:     tmpl,
		templateName: DefaultTemplate,
	}

	if opts != nil && opts.Global != nil && !utils.StringIsNil(opts.Global.Template) {
		n.templateName = opts.Global.Template
	}

	if opts != nil && opts.Webhook != nil {

		if opts.Webhook.NotificationTimeout != nil {
			n.timeout = time.Second * time.Duration(*opts.Webhook.NotificationTimeout)
		}

		if !utils.StringIsNil(opts.Webhook.Template) {
			n.templateName = opts.Webhook.Template
		}
	}

	for _, r := range receivers {
		receiver, ok := r.(*webhook.Receiver)
		if !ok || receiver == nil {
			continue
		}

		//if receiver.WebhookConfig == nil {
		//	_ = level.Warn(logger).Log("msg", "WebhookNotifier: ignore receiver because of empty config")
		//	continue
		//}

		if utils.StringIsNil(receiver.Template) {
			receiver.Template = n.templateName
		}

		n.receivers = append(n.receivers, receiver)
	}

	return n
}

func (n *Notifier) Notify(ctx context.Context, alerts *notifier.Alerts) error {

	send := func(r *webhook.Receiver) error {

		start := time.Now()
		defer func() {
			_ = level.Debug(n.logger).Log("msg", "WebhookNotifier: send message", "used", time.Since(start).String())
		}()

		var buf bytes.Buffer
		if r.Template != DefaultTemplate {
			msg, err := n.template.TempleText(r.Template, alerts, n.logger)
			if err != nil {
				_ = level.Error(n.logger).Log("msg", "WebhookNotifier: generate message error", "error", err.Error())
				return err
			}

			buf.WriteString(msg)
		} else {
			if err := utils.JsonEncode(&buf, n.template.NewTemplateData(alerts, n.logger)); err != nil {
				_ = level.Error(n.logger).Log("msg", "WebhookNotifier: encode message error", "error", err.Error())
				return err
			}
		}

		request, err := http.NewRequest(http.MethodPost, r.URL, &buf)
		if err != nil {
			return err
		}
		request.Header.Set("Content-Type", "application/json")

		if r.HttpConfig != nil {
			if r.HttpConfig.BearerToken != nil {
				bearer, err := n.notifierCtl.GetCredential(r.HttpConfig.BearerToken)
				if err != nil {
					_ = level.Error(n.logger).Log("msg", "WebhookNotifier: get bearer token error", "error", err.Error())
					return err
				}

				request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", bearer))
			} else if r.HttpConfig.BasicAuth != nil {
				pass := ""
				if r.HttpConfig.BasicAuth.Password != nil {
					p, err := n.notifierCtl.GetCredential(r.HttpConfig.BasicAuth.Password)
					if err != nil {
						_ = level.Error(n.logger).Log("msg", "WebhookNotifier: get password error", "error", err.Error())
						return err
					}

					pass = p
				}
				request.SetBasicAuth(r.HttpConfig.BasicAuth.Username, pass)
			}
		}

		transport, err := n.getTransport(r)
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

		_ = level.Debug(n.logger).Log("msg", "WebhookNotifier: send message", "to", r.URL)

		return nil
	}

	group := async.NewGroup(ctx)
	for _, receiver := range n.receivers {
		r := receiver
		group.Add(func(stopCh chan interface{}) {
			stopCh <- send(r)
		})
	}

	return group.Wait()
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
