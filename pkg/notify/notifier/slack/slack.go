package slack

import (
	"bytes"
	"context"
	"fmt"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	jsoniter "github.com/json-iterator/go"
	nmv1alpha1 "github.com/kubesphere/notification-manager/pkg/apis/v1alpha1"
	nmconfig "github.com/kubesphere/notification-manager/pkg/notify/config"
	"github.com/kubesphere/notification-manager/pkg/notify/notifier"
	"github.com/prometheus/alertmanager/config"
	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/template"
	commoncfg "github.com/prometheus/common/config"
	"io"
	"io/ioutil"
	"net/http"
	"time"
)

const (
	DefaultSendTimeout = time.Second * 3
	URL                = "https://slack.com/api/chat.postMessage"
	DefaultTemplate    = `{{ template "wechat.default.message" . }}`
)

type Notifier struct {
	slack        []*nmconfig.Slack
	timeout      time.Duration
	logger       log.Logger
	client       *http.Client
	template     *notifier.Template
	templateName string
}

type slackResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

func NewSlackNotifier(logger log.Logger, val interface{}, opts *nmv1alpha1.Options) notifier.Notifier {

	sv, ok := val.([]interface{})
	if !ok {
		_ = level.Error(logger).Log("msg", "SlackNotifier: value type error")
		return nil
	}

	c, err := commoncfg.NewClientFromConfig(commoncfg.HTTPClientConfig{}, "", false)
	if err != nil {
		_ = level.Error(log.NewNopLogger()).Log("msg", "SlackNotifier: create http client error", "error", err.Error())
		return nil
	}

	var path []string
	if opts != nil && opts.Global != nil {
		path = opts.Global.TemplateFiles
	}
	tmpl, err := notifier.NewTemplate(path)
	if err != nil {
		_ = level.Error(logger).Log("msg", "EmailNotifier: get template error", "error", err.Error())
		return nil
	}

	n := &Notifier{
		timeout:      DefaultSendTimeout,
		logger:       logger,
		client:       c,
		template:     tmpl,
		templateName: DefaultTemplate,
	}

	if opts != nil && opts.Slack != nil {

		if opts.Slack.NotificationTimeout != nil {
			n.timeout = time.Second * time.Duration(*opts.Slack.NotificationTimeout)
		}

		if len(opts.Slack.Template) > 0 {
			n.templateName = opts.Slack.Template
		}
	}

	for _, v := range sv {
		s, ok := v.(*nmconfig.Slack)
		if !ok || s == nil {
			_ = level.Error(logger).Log("msg", "SlackNotifier: value type error")
			continue
		}

		n.slack = append(n.slack, s)
	}

	return n
}

func (n *Notifier) Notify(data template.Data) []error {

	var errs []error
	send := func(c *nmconfig.Slack) error {
		ctx, cancel := context.WithTimeout(context.Background(), n.timeout)
		ctx = notify.WithGroupLabels(ctx, notifier.KvToLabelSet(data.GroupLabels))
		ctx = notify.WithReceiverName(ctx, data.Receiver)
		defer cancel()

		msg, err := n.template.TemlText(ctx, n.templateName, n.logger, data)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "SlackNotifier: generate slack message error", "error", err.Error())
			return err
		}

		var buf bytes.Buffer
		if err := jsoniter.NewEncoder(&buf).Encode(msg); err != nil {
			_ = level.Error(n.logger).Log("msg", "SlackNotifier: encode error", "error", err.Error())
			return err
		}

		url := &config.URL{}
		url.URL, err = url.Parse(URL)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "SlackNotifier: Unable to parse slack url", "url", URL, "err", err)
			return err
		}

		postMessageURL := url
		q := postMessageURL.Query()
		q.Set("token", c.Token)
		q.Set("channel", c.Channel)
		q.Set("text", msg)
		postMessageURL.RawQuery = q.Encode()

		req, err := http.NewRequest(http.MethodPost, postMessageURL.String(), &buf)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "SlackNotifier: create http request error", "error", err.Error())
			return err
		}
		req.Header.Add("Content-Type", "application/json")

		resp, err := n.client.Do(req.WithContext(ctx))
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "SlackNotifier: do http request error", "error", err.Error())
			return err
		}
		defer func() {
			_, _ = io.Copy(ioutil.Discard, resp.Body)
			_ = resp.Body.Close()
		}()

		if resp.StatusCode != http.StatusOK {
			_ = level.Error(n.logger).Log("msg", "SlackNotifier: http error", "status", resp.StatusCode)
			return err
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "SlackNotifier: read response body error", "error", err)
			return err
		}

		var slResp slackResponse
		if err := jsoniter.Unmarshal(body, &slResp); err != nil {
			_ = level.Error(n.logger).Log("msg", "SlackNotifier: decode error", "error", err)
			return err
		}

		if !slResp.OK {
			_ = level.Error(n.logger).Log("msg", "SlackNotifier: slack error", "error", slResp.Error)
			return fmt.Errorf("%s", slResp.Error)
		}

		_ = level.Debug(n.logger).Log("msg", "send slack message", "to", c.Channel)

		return nil
	}

	for _, s := range n.slack {
		err := send(s)
		if err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}

func (n *Notifier) clone(c *nmconfig.Slack) *nmconfig.Slack {

	if c == nil {
		return nil
	}

	return &nmconfig.Slack{
		Channel: c.Channel,
		Token:   c.Token,
	}
}
