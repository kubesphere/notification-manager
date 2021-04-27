package wechat

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	json "github.com/json-iterator/go"
	"github.com/kubesphere/notification-manager/pkg/async"
	"github.com/kubesphere/notification-manager/pkg/notify/config"
	"github.com/kubesphere/notification-manager/pkg/notify/notifier"
	"github.com/prometheus/alertmanager/template"
)

const (
	DefaultApiURL      = "https://qyapi.weixin.qq.com/cgi-bin/"
	DefaultSendTimeout = time.Second * 3
	ToUserBatchSize    = 1000
	ToPartyBatchSize   = 100
	ToTagBatchSize     = 100
	AccessTokenInvalid = 42001
	DefaultTemplate    = `{{ template "nm.default.text" . }}`
	MessageMaxSize     = 2048
	DefaultExpires     = time.Hour * 2
)

type Notifier struct {
	notifierCfg    *config.Config
	wechat         map[string]*config.Wechat
	accessToken    string
	timeout        time.Duration
	logger         log.Logger
	template       *notifier.Template
	templateName   string
	ats            *notifier.AccessTokenService
	messageMaxSize int
	tokenExpires   time.Duration
}

type weChatMessageContent struct {
	Content string `json:"content"`
}

type weChatMessage struct {
	Text    weChatMessageContent `yaml:"text,omitempty" json:"text,omitempty"`
	ToUser  string               `yaml:"touser,omitempty" json:"touser,omitempty"`
	ToParty string               `yaml:"toparty,omitempty" json:"toparty,omitempty"`
	ToTag   string               `yaml:"totag,omitempty" json:"totag,omitempty"`
	AgentID string               `yaml:"agentid,omitempty" json:"agentid,omitempty"`
	Safe    string               `yaml:"safe,omitempty" json:"safe,omitempty"`
	Type    string               `yaml:"msgtype,omitempty" json:"msgtype,omitempty"`
}

type weChatResponse struct {
	ErrorCode    int    `json:"errcode"`
	ErrorMsg     string `json:"errmsg"`
	AccessToken  string `json:"access_token,omitempty"`
	InvalidUser  string `json:"invaliduser,omitempty"`
	InvalidParty string `json:"invalidparty,omitempty"`
	InvalidTag   string `json:"invalidTag,omitempty"`
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
		notifierCfg:    notifierCfg,
		wechat:         make(map[string]*config.Wechat),
		logger:         logger,
		timeout:        DefaultSendTimeout,
		template:       tmpl,
		templateName:   DefaultTemplate,
		ats:            notifier.GetAccessTokenService(),
		messageMaxSize: MessageMaxSize,
		tokenExpires:   DefaultExpires,
	}

	if opts != nil && opts.Wechat != nil {

		if opts.Wechat.NotificationTimeout != nil {
			n.timeout = time.Second * time.Duration(*opts.Wechat.NotificationTimeout)
		}

		if len(opts.Wechat.Template) > 0 {
			n.templateName = opts.Wechat.Template
		} else if opts.Global != nil && len(opts.Global.Template) > 0 {
			n.templateName = opts.Global.Template
		}

		if opts.Wechat.MessageMaxSize > 0 {
			n.messageMaxSize = opts.Wechat.MessageMaxSize
		}

		if opts.Wechat.TokenExpires != 0 {
			n.tokenExpires = opts.Wechat.TokenExpires
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

		newReceiver := receiver.Clone()
		key, err := notifier.Md5key(newReceiver.WechatConfig)
		if err != nil {
			_ = level.Error(logger).Log("msg", "WechatNotifier: get notifier error", "error", err.Error())
			continue
		}

		w, ok := n.wechat[key]
		if !ok {
			w = newReceiver
		} else {
			if newReceiver.ToUser != nil && len(newReceiver.ToUser) > 0 {
				w.ToUser = append(w.ToUser, newReceiver.ToUser...)
			}

			if newReceiver.ToParty != nil && len(newReceiver.ToParty) > 0 {
				w.ToParty = append(w.ToUser, newReceiver.ToParty...)
			}

			if newReceiver.ToTag != nil && len(newReceiver.ToTag) > 0 {
				w.ToTag = append(w.ToUser, newReceiver.ToTag...)
			}
		}

		n.wechat[key] = w
	}

	return n
}

func (n *Notifier) Notify(ctx context.Context, data template.Data) []error {

	send := func(w *config.Wechat, msg string) error {

		start := time.Now()
		defer func() {
			_ = level.Debug(n.logger).Log("msg", "WechatNotifier: send message", "used", time.Since(start).String())
		}()

		wechatMsg := &weChatMessage{
			Text: weChatMessageContent{
				Content: msg,
			},
			ToUser:  notifier.ArrayToString(w.ToUser, "|"),
			ToParty: notifier.ArrayToString(w.ToParty, "|"),
			ToTag:   notifier.ArrayToString(w.ToTag, "|"),
			AgentID: w.WechatConfig.AgentID,
			Type:    "text",
			Safe:    "0",
		}

		sendMessage := func() (bool, error) {

			accessToken, err := n.getToken(ctx, w)
			if err != nil {
				_ = level.Error(n.logger).Log("msg", "WechatNotifier: get access token error", "error", err.Error())
				return false, err
			}

			var buf bytes.Buffer
			if err := json.NewEncoder(&buf).Encode(wechatMsg); err != nil {
				_ = level.Error(n.logger).Log("msg", "WechatNotifier: encode message error", "error", err.Error())
				return false, err
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

			if weResp.ErrorCode == 0 {
				if weResp.InvalidUser != "" || weResp.InvalidParty != "" || weResp.InvalidTag != "" {
					_ = level.Error(n.logger).Log("msg", "WechatNotifier: send message",
						"from", w.WechatConfig.AgentID,
						"Invalid user", weResp.InvalidUser,
						"Invalid party", weResp.InvalidParty,
						"Invalid tag", weResp.InvalidTag)
				}
				_ = level.Debug(n.logger).Log("msg", "WechatNotifier: send message",
					"from", w.WechatConfig.AgentID,
					"toUser", notifier.ArrayToString(w.ToUser, "|"),
					"toParty", notifier.ArrayToString(w.ToParty, "|"),
					"toTag", notifier.ArrayToString(w.ToTag, "|"))
				return false, nil
			}

			// AccessToken is expired
			if weResp.ErrorCode == AccessTokenInvalid {
				_ = level.Error(n.logger).Log("msg", "WechatNotifier: token expired", "error", err)
				go n.invalidToken(ctx, w)
				return true, fmt.Errorf("%s", weResp.ErrorMsg)
			}

			_ = level.Error(n.logger).Log("msg", "WechatNotifier: wechat response error",
				"error code", weResp.ErrorCode, "error message", weResp.ErrorMsg,
				"from", w.WechatConfig.AgentID,
				"toUser", notifier.ArrayToString(w.ToUser, "|"),
				"toParty", notifier.ArrayToString(w.ToParty, "|"),
				"toTag", notifier.ArrayToString(w.ToTag, "|"))
			return false, nil
		}

		retry, err := sendMessage()
		if retry {
			_, err = sendMessage()
		}

		return err
	}

	group := async.NewGroup(ctx)
	for _, w := range n.wechat {

		newData := notifier.Filter(data, w.Selector, n.logger)
		if len(newData.Alerts) == 0 {
			continue
		}

		messages, err := n.template.Split(newData, MessageMaxSize, n.templateName, n.logger)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "WechatNotifier: split message error", "error", err.Error())
			return nil
		}

		us, ps, ts := 0, 0, 0
		toUser := w.ToUser
		toParty := w.ToParty
		toTag := w.ToTag

		nw := w.Clone()
		for {
			if us >= len(toUser) && ps >= len(toParty) && ts >= len(toTag) {
				break
			}

			nw.ToUser = batch(toUser, &us, ToUserBatchSize)
			nw.ToParty = batch(toParty, &ps, ToPartyBatchSize)
			nw.ToTag = batch(toTag, &ts, ToTagBatchSize)

			for _, m := range messages {
				msg := m
				group.Add(func(stopCh chan interface{}) {
					stopCh <- send(nw, msg)
				})
			}
		}
	}

	return group.Wait()
}

func (n *Notifier) getToken(ctx context.Context, w *config.Wechat) (string, error) {

	get := func(ctx context.Context) (string, time.Duration, error) {
		u := w.WechatConfig.APIURL
		u, err := notifier.UrlWithPath(u, "gettoken")
		if err != nil {
			return "", 0, err
		}

		apiSecret, err := n.notifierCfg.GetCredential(w.WechatConfig.APISecret)
		if err != nil {
			return "", 0, err
		}

		parameters := make(map[string]string)
		parameters["corpsecret"] = apiSecret
		parameters["corpid"] = w.WechatConfig.CorpID
		u, err = notifier.UrlWithParameters(u, parameters)
		if err != nil {
			return "", 0, err
		}

		var request *http.Request
		request, err = http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			return "", 0, err
		}
		request.Header.Set("Content-Type", "application/json")

		body, err := notifier.DoHttpRequest(ctx, nil, request)
		if err != nil {
			return "", 0, err
		}

		resp := &weChatResponse{}
		err = json.Unmarshal(body, resp)
		if err != nil {
			return "", 0, err
		}

		if resp.ErrorCode != 0 {
			return "", 0, fmt.Errorf("error code %d, error message %s", resp.ErrorCode, resp.ErrorMsg)
		}

		_ = level.Debug(n.logger).Log("msg", "WechatNotifier: get token", "key", w.WechatConfig.CorpID+" | "+w.WechatConfig.AgentID)
		return resp.AccessToken, DefaultExpires, nil
	}

	return n.ats.GetToken(ctx, w.WechatConfig.CorpID+" | "+w.WechatConfig.AgentID, get)
}

func (n *Notifier) invalidToken(ctx context.Context, w *config.Wechat) {
	key := w.WechatConfig.CorpID + " | " + w.WechatConfig.AgentID
	n.ats.InvalidToken(ctx, key, n.logger)
}

func batch(src []string, index *int, size int) []string {
	if *index > len(src) {
		return nil
	}

	var sub []string
	if *index+size > len(src) {
		sub = src[*index:]
	} else {
		sub = src[*index : *index+size]
	}

	*index += size
	return sub
}
