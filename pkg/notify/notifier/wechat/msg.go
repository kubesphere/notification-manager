package wechat

import (
	"bytes"
	"context"
	"fmt"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	json "github.com/json-iterator/go"
	"github.com/kubesphere/notification-manager/pkg/notify/config"
	"github.com/kubesphere/notification-manager/pkg/notify/notifier"
	"github.com/prometheus/alertmanager/template"
	"net/http"
	"strings"
	"time"
)

const (
	DefaultApiURL      = "https://qyapi.weixin.qq.com/cgi-bin/"
	DefaultSendTimeout = time.Second * 3
	ToUserBatchSize    = 1000
	ToPartyBatchSize   = 100
	ToTagBatchSize     = 100
	AccessTokenInvalid = 42001
	DefaultTemplate    = `{{ template "nm.default.text" . }}`
)

type Notifier struct {
	notifierCfg  *config.Config
	wechat       map[string]*config.Wechat
	accessToken  string
	timeout      time.Duration
	logger       log.Logger
	template     *notifier.Template
	templateName string
}

type weChatMessageContent struct {
	Content string `json:"content"`
}

type weChatMessage struct {
	Text    weChatMessageContent `yaml:"text,omitempty" json:"text,omitempty"`
	ToUser  string               `yaml:"touser,omitempty" json:"touser,omitempty"`
	ToParty string               `yaml:"toparty,omitempty" json:"toparty,omitempty"`
	Totag   string               `yaml:"totag,omitempty" json:"totag,omitempty"`
	AgentID string               `yaml:"agentid,omitempty" json:"agentid,omitempty"`
	Safe    string               `yaml:"safe,omitempty" json:"safe,omitempty"`
	Type    string               `yaml:"msgtype,omitempty" json:"msgtype,omitempty"`
}

type weChatResponse struct {
	Code  int    `json:"code"`
	Error string `json:"error"`
}

func NewWechatNotifier(logger log.Logger, receivers []config.Receiver, notifierCfg *config.Config) notifier.Notifier {

	var path []string
	opts := notifierCfg.ReceiverOpts
	if opts != nil && opts.Global != nil {
		path = opts.Global.TemplateFiles
	}
	tmpl, err := notifier.NewTemplate(path)
	if err != nil {
		_ = level.Error(logger).Log("msg", "WechatNotifier: get template error", "error", err.Error())
		return nil
	}

	n := &Notifier{
		notifierCfg:  notifierCfg,
		wechat:       make(map[string]*config.Wechat),
		logger:       logger,
		timeout:      DefaultSendTimeout,
		template:     tmpl,
		templateName: DefaultTemplate,
	}

	if opts != nil && opts.Wechat != nil {

		if opts.Wechat.NotificationTimeout != nil {
			n.timeout = time.Second * time.Duration(*opts.Wechat.NotificationTimeout)
		}

		if len(opts.Wechat.Template) > 0 {
			n.templateName = opts.Wechat.Template
		}
	}

	for _, r := range receivers {

		receiver, ok := r.(*config.Wechat)
		if !ok || receiver == nil {
			continue
		}

		if receiver.WechatConfig == nil {
			_ = level.Warn(logger).Log("msg", "WechatNotifier: ignore receiver because of empty config")
			continue
		}

		if len(receiver.WechatConfig.APIURL) == 0 {
			receiver.WechatConfig.APIURL = DefaultApiURL
		}

		c := receiver.Clone()
		key, err := notifier.Md5key(c)
		if err != nil {
			_ = level.Error(logger).Log("msg", "WechatNotifier: get notifier error", "error", err.Error())
			continue
		}

		w, ok := n.wechat[key]
		if !ok {
			w = c
		}

		if len(receiver.ToUser) > 0 {
			w.ToUser += "|" + receiver.ToUser
		}
		w.ToUser = strings.TrimPrefix(w.ToUser, "|")

		if len(receiver.ToTag) > 0 {
			w.ToTag += "|" + receiver.ToTag
		}
		w.ToTag = strings.TrimPrefix(w.ToTag, "|")

		if len(receiver.ToParty) > 0 {
			w.ToParty += "|" + receiver.ToParty
		}
		w.ToParty = strings.TrimPrefix(w.ToParty, "|")

		n.wechat[key] = w
	}

	return n
}

func (n *Notifier) Notify(data template.Data) []error {

	var errs []error
	send := func(w *config.Wechat, data template.Data) (bool, error) {
		ctx, cancel := context.WithTimeout(context.Background(), n.timeout)
		defer cancel()

		msg, err := n.template.TemlText(n.templateName, n.logger, data)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "WechatNotifier: generate message error", "error", err.Error())
			return false, err
		}

		wechatMsg := &weChatMessage{
			Text: weChatMessageContent{
				Content: msg,
			},
			ToUser:  w.ToUser,
			ToParty: w.ToParty,
			Totag:   w.ToTag,
			AgentID: w.WechatConfig.AgentID,
			Type:    "text",
			Safe:    "0",
		}

		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(wechatMsg); err != nil {
			_ = level.Error(n.logger).Log("msg", "WechatNotifier: encode message error", "error", err.Error())
			return false, err
		}

		apiSecret, err := n.notifierCfg.GetSecretData(w.GetNamespace(), w.WechatConfig.APISecret)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "WechatNotifier: get api secret error", "error", err.Error())
			return false, err
		}

		res := make(chan interface{})
		ats.get(w.WechatConfig.APIURL, w.WechatConfig.CorpID, apiSecret, w.WechatConfig.AgentID, ctx, res)
		accessToken := ""
		select {
		case <-ctx.Done():
			_ = level.Error(n.logger).Log("msg", "WechatNotifier: get access token timeout")
			return true, fmt.Errorf("get accesstoken timeout")
		case val := <-res:
			switch val.(type) {
			case error:
				_ = level.Error(n.logger).Log("msg", "WechatNotifier: get access token error", "error", val.(error).Error())
				return true, val.(error)
			case string:
				accessToken = val.(string)
			}
		}

		u, err := notifier.UrlWithPath(w.WechatConfig.APIURL, "message/send")
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "WechatNotifier: set path error", "error", err)
			return false, err
		}

		parameters := make(map[string]string)
		parameters["access_token"] = accessToken
		u, err = notifier.UrlWithParameters(u, parameters)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "WechatNotifier: set parameters error", "error", err)
			return false, err
		}

		request, err := http.NewRequest(http.MethodPost, u, &buf)
		if err != nil {
			return false, err
		}
		request.Header.Set("Content-Type", "application/json")

		body, err := notifier.DoHttpRequest(ctx, nil, request)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "WechatNotifier: do http error", "error", err)
			return false, err
		}

		var weResp weChatResponse
		if err := json.Unmarshal(body, &weResp); err != nil {
			_ = level.Error(n.logger).Log("msg", "WechatNotifier: decode response body error", "error", err)
			return false, err
		}

		// https://work.weixin.qq.com/api/doc#10649
		if weResp.Code == 0 {
			_ = level.Debug(n.logger).Log("msg", "WechatNotifier: send message", "from", w.WechatConfig.AgentID, "toUser", w.ToUser, "toParty", w.ToParty, "toTag", w.ToTag)
			return false, nil
		}

		// AccessToken is expired
		if weResp.Code == AccessTokenInvalid {
			ats.invalid(w.WechatConfig.APIURL, w.WechatConfig.CorpID, apiSecret, w.WechatConfig.AgentID)
			return true, fmt.Errorf("%s", weResp.Error)
		}

		return false, fmt.Errorf("%s", weResp.Error)
	}

	for _, w := range n.wechat {

		us, ps, ts := 0, 0, 0
		toUser := strings.Split(w.ToUser, "|")
		toParty := strings.Split(w.ToParty, "|")
		toTag := strings.Split(w.ToTag, "|")

		batch := func(src []string, index *int, size int) string {
			if *index > len(src) {
				return ""
			}

			var sub []string
			if *index+size > len(src) {
				sub = src[*index:]
			} else {
				sub = src[*index : *index+size]
			}

			*index += size

			to := ""
			for _, t := range sub {
				to += fmt.Sprintf("%s|", t)
			}

			return to
		}

		nw := w.Clone()
		for {
			if us >= len(toUser) && ps >= len(toParty) && ts >= len(toTag) {
				break
			}

			nw.ToUser = batch(toUser, &us, ToUserBatchSize)
			nw.ToParty = batch(toParty, &ps, ToPartyBatchSize)
			nw.ToTag = batch(toTag, &ts, ToTagBatchSize)

			for _, alert := range data.Alerts {
				d := template.Data{
					Alerts: template.Alerts{
						alert,
					},
					Receiver:    data.Receiver,
					GroupLabels: data.GroupLabels,
				}
				retry, err := send(nw, d)
				if err != nil {
					if retry {
						_, err = send(nw, d)
					}
					if err != nil {
						errs = append(errs, err)
					}
				}
			}
		}
	}

	return errs
}
