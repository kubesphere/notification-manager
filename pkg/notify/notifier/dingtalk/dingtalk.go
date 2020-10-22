package dingtalk

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	json "github.com/json-iterator/go"
	"github.com/kubesphere/notification-manager/pkg/notify/config"
	"github.com/kubesphere/notification-manager/pkg/notify/notifier"
	"github.com/prometheus/alertmanager/template"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	URL                = "https://oapi.dingtalk.com/"
	DefaultSendTimeout = time.Second * 3
	DefaultTemplate    = `{{ template "nm.default.text" . }}`
)

type Notifier struct {
	notifierCfg  *config.Config
	DingTalk     []*config.DingTalk
	timeout      time.Duration
	logger       log.Logger
	template     *notifier.Template
	templateName string
	throttle     *Throttle
	maxWaitTime  time.Duration
}

type dingtalkMessageContent struct {
	Content string `json:"content"`
}

type dingtalkMessage struct {
	Text dingtalkMessageContent `yaml:"text,omitempty" json:"text,omitempty"`
	ID   string                 `yaml:"chatid,omitempty" json:"chatid,omitempty"`
	Type string                 `yaml:"msgtype,omitempty" json:"msgtype,omitempty"`
}

type response struct {
	Code    int    `json:"errcode"`
	Message string `json:"errmsg"`
	Token   string `json:"access_token"`
	Status  int    `json:"status"`
	Punish  string `json:"punish"`
}

func NewDingTalkNotifier(logger log.Logger, receivers []config.Receiver, notifierCfg *config.Config) notifier.Notifier {

	var path []string
	opts := notifierCfg.ReceiverOpts
	if opts != nil && opts.Global != nil {
		path = opts.Global.TemplateFiles
	}
	tmpl, err := notifier.NewTemplate(path)
	if err != nil {
		_ = level.Error(logger).Log("msg", "DingTalkNotifier: get template error", "error", err.Error())
		return nil
	}

	n := &Notifier{
		notifierCfg:  notifierCfg,
		timeout:      DefaultSendTimeout,
		logger:       logger,
		template:     tmpl,
		templateName: DefaultTemplate,
		throttle:     GetThrottle(),
	}

	if opts != nil && opts.DingTalk != nil {

		if opts.DingTalk.NotificationTimeout != nil {
			n.timeout = time.Second * time.Duration(*opts.DingTalk.NotificationTimeout)
		}

		if len(opts.DingTalk.Template) > 0 {
			n.templateName = opts.DingTalk.Template
		} else if opts.Global != nil && len(opts.Global.Template) > 0 {
			n.templateName = opts.Global.Template
		}

		if opts.DingTalk.MaxWaitTime != nil {
			n.maxWaitTime = time.Second * time.Duration(*opts.DingTalk.MaxWaitTime)
		}
	}

	n.throttle.SetMaxWaitTime(n.maxWaitTime)

	for _, r := range receivers {
		receiver, ok := r.(*config.DingTalk)
		if !ok || receiver == nil {
			continue
		}

		if receiver.DingTalkConfig == nil {
			_ = level.Warn(logger).Log("msg", "DingTalkNotifier: ignore receiver because of empty config")
			continue
		}

		n.DingTalk = append(n.DingTalk, receiver)
	}

	return n
}

func (n *Notifier) Notify(data template.Data) []error {
	var errs []error
	send := func(c *config.DingTalk) error {
		ctx, cancel := context.WithTimeout(context.Background(), n.timeout)
		defer cancel()

		if c.DingTalkConfig.Conversation != nil {
			if es := n.sendToConversation(ctx, c, data); es != nil {
				errs = append(errs, es...)
			}
		}

		if c.DingTalkConfig.ChatBot != nil {
			if err := n.sendToWebhook(ctx, c, data); err != nil {
				errs = append(errs, err)
			}
		}

		return nil
	}

	for _, s := range n.DingTalk {
		_ = send(s)
	}

	return errs
}

func (n *Notifier) sendToWebhook(ctx context.Context, d *config.DingTalk, data template.Data) error {

	bot := d.DingTalkConfig.ChatBot

	webhook, err := n.notifierCfg.GetSecretData(d.GetNamespace(), bot.Webhook)
	if err != nil {
		_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: get webhook secret error", "error", err.Error())
		return err
	}

	send := func() error {
		msg, err := n.template.TemlText(n.templateName, n.logger, data)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: generate message error", "error", err.Error())
			return err
		}

		if bot.Keywords != nil && len(bot.Keywords) > 0 {
			kw := "[Keywords] "
			for _, k := range bot.Keywords {
				kw = fmt.Sprintf("%s%s, ", kw, k)
			}

			msg = fmt.Sprintf("%s\n\n%s: ", msg, strings.TrimSuffix(kw, ", "))
		}

		dm := dingtalkMessage{
			Text: dingtalkMessageContent{
				Content: msg,
			},
			Type: "text",
		}

		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(dm); err != nil {
			_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: encode message error", "error", err.Error())
			return err
		}

		secret, err := n.notifierCfg.GetSecretData(d.GetNamespace(), bot.Secret)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: get chatbot secret error", "error", err.Error())
			return err
		}

		u := webhook
		if len(secret) > 0 {
			timestamp, sign := calcSign(secret)
			p := make(map[string]string)
			p["timestamp"] = timestamp
			p["sign"] = sign
			u, err = notifier.UrlWithParameters(webhook, p)
			if err != nil {
				_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: set parameters error", "error", err)
				return err
			}
		}

		request, err := http.NewRequest(http.MethodPost, u, &buf)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: create http request error", "error", err)
			return err
		}
		request.Header.Set("Content-Type", "application/json")

		body, err := notifier.DoHttpRequest(ctx, nil, request)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: do http error", "error", err)
			return err
		}

		res := &response{}
		if err := json.Unmarshal(body, res); err != nil {
			_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: decode response body error", "error", err)
			return err
		}

		if res.Code != 0 {
			_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: send message to webhook error", "name", bot.Webhook.Name, "key", bot.Webhook.Key, "errcode", res.Code, "errmsg", res.Message)
			return err
		}

		if res.Status != 0 {
			_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: send message to webhook error", "name", bot.Webhook.Name, "key", bot.Webhook.Key, "status", res.Status, "punish", res.Punish)
			return err
		}

		_ = level.Debug(n.logger).Log("msg", "DingTalkNotifier: send message to chatbot", "name", bot.Webhook.Name, "key", bot.Webhook.Key)

		return nil
	}

	allow, err := n.throttle.Allow(webhook, send)
	if !allow {
		_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: send message to chatbot error", "name", bot.Webhook.Name, "key", bot.Webhook.Key, "error", err.Error())
		return err
	}

	return nil
}

func (n *Notifier) sendToConversation(ctx context.Context, d *config.DingTalk, data template.Data) []error {

	send := func(alert template.Data) error {
		token, err := n.getToken(ctx, d)
		if err != nil {
			return err
		}

		msg, err := n.template.TemlText(n.templateName, n.logger, alert)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: generate message error", "error", err.Error())
			return err
		}

		dm := dingtalkMessage{
			Text: dingtalkMessageContent{
				Content: msg,
			},
			Type: "text",
			ID:   d.DingTalkConfig.Conversation.ChatID,
		}

		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(dm); err != nil {
			_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: encode message error", "error", err.Error())
			return err
		}

		u, err := notifier.UrlWithPath(URL, "chat/send")
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: set path error", "error", err)
			return err
		}

		p := make(map[string]string)
		p["access_token"] = token
		u, err = notifier.UrlWithParameters(u, p)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: set parameters error", "error", err)
			return err
		}

		request, err := http.NewRequest(http.MethodPost, u, &buf)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: create http request error", "error", err)
			return err
		}
		request.Header.Set("Content-Type", "application/json")

		body, err := notifier.DoHttpRequest(ctx, nil, request)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: do http error", "error", err)
			return err
		}

		res := &response{}
		if err := json.Unmarshal(body, res); err != nil {
			_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: decode response body error", "error", err)
			return err
		}

		if res.Code != 0 {
			_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: send message to conversation error", "conversation", d.DingTalkConfig.Conversation.ChatID, "errcode", res.Code, "errmsg", res.Message)
			return err
		}

		_ = level.Debug(n.logger).Log("msg", "DingTalkNotifier: send message", "to", d.DingTalkConfig.Conversation.ChatID)

		return nil
	}

	var errs []error
	for _, alert := range data.Alerts {
		d := template.Data{
			Alerts: template.Alerts{
				alert,
			},
			Receiver:    data.Receiver,
			GroupLabels: data.GroupLabels,
		}
		if e := send(d); e != nil {
			errs = append(errs, e)
		}
	}
	return errs
}

func (n *Notifier) getToken(ctx context.Context, d *config.DingTalk) (string, error) {

	u, err := notifier.UrlWithPath(URL, "gettoken")
	if err != nil {
		return "", err
	}

	appkey, err := n.notifierCfg.GetSecretData(d.GetNamespace(), d.DingTalkConfig.Conversation.AppKey)
	if err != nil {
		_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: get appkey error", "error", err)
		return "", err
	}

	appsecret, err := n.notifierCfg.GetSecretData(d.GetNamespace(), d.DingTalkConfig.Conversation.AppSecret)
	if err != nil {
		_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: get appsecret error", "error", err)
		return "", err
	}

	p := make(map[string]string)
	p["appkey"] = appkey
	p["appsecret"] = appsecret

	u, err = notifier.UrlWithParameters(u, p)
	if err != nil {
		_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: set parameter error", "error", err)
		return "", err
	}

	request, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: create http request error", "error", err)
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")

	body, err := notifier.DoHttpRequest(ctx, nil, request)
	if err != nil {
		_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: do http error", "error", err)
		return "", err
	}

	res := &response{}
	if err := json.Unmarshal(body, res); err != nil {
		_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: decode response body error", "error", err)
		return "", err
	}

	if res.Code != 0 {
		_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: get token error", "errcode", res.Code, "errmsg", res.Message)
		return "", fmt.Errorf("errcode %d, errmsg %s", res.Code, res.Message)
	}

	return res.Token, nil
}

func calcSign(secret string) (string, string) {

	timestamp := fmt.Sprintf("%d", time.Now().Unix()*1000)
	msg := fmt.Sprintf("%s\n%s", timestamp, secret)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(msg))
	sign := base64.StdEncoding.EncodeToString(h.Sum(nil))

	return timestamp, url.QueryEscape(sign)
}
