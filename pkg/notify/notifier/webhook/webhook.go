package webhook

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	json "github.com/json-iterator/go"
	"github.com/kubesphere/notification-manager/pkg/notify/config"
	"github.com/kubesphere/notification-manager/pkg/notify/notifier"
	"github.com/mwitkow/go-conntrack"
	"github.com/prometheus/alertmanager/template"
	"net/http"
	"net/url"
	"time"
)

const (
	DefaultSendTimeout = time.Second * 5
	DefaultTemplate    = `{{ template "webhook.default.message" . }}`
)

type Notifier struct {
	notifierCfg  *config.Config
	webhooks     []*config.Webhook
	timeout      time.Duration
	logger       log.Logger
	template     *notifier.Template
	templateName string
}

func NewWebhookNotifier(logger log.Logger, receivers []config.Receiver, notifierCfg *config.Config) notifier.Notifier {

	var path []string
	opts := notifierCfg.ReceiverOpts
	if opts != nil && opts.Global != nil {
		path = opts.Global.TemplateFiles
	}
	tmpl, err := notifier.NewTemplate(path)
	if err != nil {
		_ = level.Error(logger).Log("msg", "WebhookNotifier: get template error", "error", err.Error())
		return nil
	}

	n := &Notifier{
		notifierCfg:  notifierCfg,
		timeout:      DefaultSendTimeout,
		logger:       logger,
		template:     tmpl,
		templateName: DefaultTemplate,
	}

	if opts != nil && opts.Webhook != nil {

		if opts.Webhook.NotificationTimeout != nil {
			n.timeout = time.Second * time.Duration(*opts.Webhook.NotificationTimeout)
		}

		if len(opts.Webhook.Template) > 0 {
			n.templateName = opts.Webhook.Template
		} else if opts.Global != nil && len(opts.Global.Template) > 0 {
			n.templateName = opts.Global.Template
		}
	}

	for _, r := range receivers {
		receiver, ok := r.(*config.Webhook)
		if !ok || receiver == nil {
			continue
		}

		if receiver.WebhookConfig == nil {
			_ = level.Warn(logger).Log("msg", "WebhookNotifier: ignore receiver because of empty config")
			continue
		}

		n.webhooks = append(n.webhooks, receiver)
	}

	return n
}

func (n *Notifier) Notify(data template.Data) []error {

	var errs []error
	var value interface{} = data
	if n.templateName != DefaultTemplate {
		msg, err := n.template.TemlText(n.templateName, n.logger, data)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "WebhookNotifier: generate message error", "error", err.Error())
			return append(errs, err)
		}

		value = msg
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(value); err != nil {
		_ = level.Error(n.logger).Log("msg", "WebhookNotifier: encode message error", "error", err.Error())
		return append(errs, err)
	}

	send := func(w *config.Webhook) error {
		ctx, cancel := context.WithTimeout(context.Background(), n.timeout)
		defer cancel()

		request, err := http.NewRequest(http.MethodPost, w.WebhookConfig.URL, &buf)
		if err != nil {
			return err
		}
		request.Header.Set("Content-Type", "application/json")

		if w.WebhookConfig.HttpConfig.BearerToken != nil {
			bearer, err := n.notifierCfg.GetSecretData(w.GetNamespace(), w.WebhookConfig.HttpConfig.BearerToken)
			if err != nil {
				_ = level.Error(n.logger).Log("msg", "WebhookNotifier: get bearer token error", "error", err.Error())
				return err
			}

			request.Header.Set("Authorization", bearer)
		} else if w.WebhookConfig.HttpConfig.BasicAuth != nil {
			pass := ""
			if w.WebhookConfig.HttpConfig.BasicAuth.Password != nil {
				p, err := n.notifierCfg.GetSecretData(w.GetNamespace(), w.WebhookConfig.HttpConfig.BasicAuth.Password)
				if err != nil {
					_ = level.Error(n.logger).Log("msg", "WebhookNotifier: get password error", "error", err.Error())
					return err
				}

				pass = p
			}
			request.SetBasicAuth(w.WebhookConfig.HttpConfig.BasicAuth.Username, pass)
		}

		transport, err := n.getTransport(w)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "WebhookNotifier: get transport error", "error", err.Error())
			return err
		}

		client := &http.Client{
			Transport: transport,
			Timeout:   n.timeout,
		}

		_, err = notifier.DoHttpRequest(ctx, client, request)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "WebhookNotifier: do http request error", "error", err.Error())
			return err
		}

		_ = level.Debug(n.logger).Log("msg", "WebhookNotifier: send message", "to", w.WebhookConfig.URL)

		return nil
	}

	for _, w := range n.webhooks {
		err := send(w)
		if err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}

func (n *Notifier) getTransport(w *config.Webhook) (http.RoundTripper, error) {

	transport := &http.Transport{
		DisableKeepAlives:  false,
		DisableCompression: true,
		DialContext: conntrack.NewDialContextFunc(
			conntrack.DialWithTracing(),
			conntrack.DialWithName(w.WebhookConfig.URL),
		),
	}

	if c := w.WebhookConfig.HttpConfig; c != nil {

		if c.TLSConfig != nil {
			tlsConfig := &tls.Config{InsecureSkipVerify: c.TLSConfig.InsecureSkipVerify}

			// If a CA cert is provided then let's read it in so we can validate the
			// scrape target's certificate properly.
			if c.TLSConfig.CA != nil {
				if ca, err := n.notifierCfg.GetSecretData(w.GetNamespace(), c.TLSConfig.CA); err != nil {
					return nil, err
				} else {
					caCertPool := x509.NewCertPool()
					if !caCertPool.AppendCertsFromPEM([]byte(ca)) {
						return nil, err
					}
					tlsConfig.RootCAs = caCertPool
				}
			}

			if len(c.TLSConfig.ServerName) > 0 {
				tlsConfig.ServerName = c.TLSConfig.ServerName
			}

			// If a client cert & key is provided then configure TLS config accordingly.
			if c.TLSConfig.Cert != nil && c.TLSConfig.Key == nil {
				return nil, fmt.Errorf("client cert file specified without client key file")
			} else if c.TLSConfig.Cert == nil && c.TLSConfig.Key != nil {
				return nil, fmt.Errorf("client key file specified without client cert file")
			} else if c.TLSConfig.Cert != nil && c.TLSConfig.Key != nil {
				key, err := n.notifierCfg.GetSecretData(w.GetNamespace(), c.TLSConfig.Key)
				if err != nil {
					return nil, err
				}

				cert, err := n.notifierCfg.GetSecretData(w.GetNamespace(), c.TLSConfig.Cert)
				if err != nil {
					return nil, err
				}

				tlsCert, err := tls.X509KeyPair([]byte(cert), []byte(key))
				if err != nil {
					return nil, err
				}
				tlsConfig.Certificates = []tls.Certificate{tlsCert}
			}

			transport.TLSClientConfig = tlsConfig
		}

		if len(c.ProxyURL) > 0 {
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
