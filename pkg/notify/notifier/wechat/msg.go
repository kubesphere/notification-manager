package wechat

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
	"github.com/prometheus/alertmanager/template"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

const (
	DefaultApiURL      = "https://qyapi.weixin.qq.com/cgi-bin/"
	DefaultSendTimeout = time.Second * 3
	ToUserMax          = 1000
	ToPartyMax         = 100
	ToTagMax           = 100
)

type Notifier struct {
	wechat      map[string]*config.WechatConfig
	accessToken string
	client      *http.Client
	timeout     time.Duration
	logger      log.Logger
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

func NewWechatNotifier(logger log.Logger, val interface{}, opts *nmv1alpha1.Options) notifier.Notifier {

	sv, ok := val.([]interface{})
	if !ok {
		_ = level.Error(logger).Log("msg", "WechatNotifier: value type error")
		return nil
	}

	n := &Notifier{
		wechat:  make(map[string]*config.WechatConfig),
		logger:  logger,
		timeout: DefaultSendTimeout,
		client:  ats.client,
	}

	if opts != nil && opts.Wechat != nil && opts.Wechat.NotificationTimeout != nil {
		n.timeout = time.Second * time.Duration(*opts.Wechat.NotificationTimeout)
	}

	for _, v := range sv {

		wv, ok := v.(*nmconfig.Wechat)
		if !ok || wv == nil {
			_ = level.Error(logger).Log("msg", "WechatNotifier: value type error")
			continue
		}

		c := n.clone(wv.WechatConfig)
		key, err := notifier.Md5key(c)
		if err != nil {
			_ = level.Error(logger).Log("msg", "WechatNotifier: get notifier error", "error", err.Error())
			continue
		}

		w, ok := n.wechat[key]
		if !ok {
			w = c
		}

		if len(wv.ToUser) > 0 {
			w.ToUser += "|" + wv.ToUser
		}
		w.ToUser = strings.TrimPrefix(w.ToUser, "|")

		if len(wv.ToTag) > 0 {
			w.ToTag += "|" + wv.ToTag
		}
		w.ToTag = strings.TrimPrefix(w.ToTag, "|")

		if len(wv.ToParty) > 0 {
			w.ToParty += "|" + wv.ToParty
		}
		w.ToParty = strings.TrimPrefix(w.ToParty, "|")

		n.wechat[key] = w
	}

	return n
}

func (n *Notifier) Notify(data template.Data) []error {

	var errs []error
	send := func(c *config.WechatConfig) (bool, error) {
		ctx, cancel := context.WithTimeout(context.Background(), n.timeout)
		defer cancel()

		msg := &weChatMessage{
			Text: weChatMessageContent{
				Content: n.getMsg(data),
			},
			ToUser:  c.ToUser,
			ToParty: c.ToParty,
			Totag:   c.ToTag,
			AgentID: c.AgentID,
			Type:    "text",
			Safe:    "0",
		}

		var buf bytes.Buffer
		if err := jsoniter.NewEncoder(&buf).Encode(msg); err != nil {
			_ = level.Error(n.logger).Log("msg", "WechatNotifier: encode error", "error", err.Error())
			return false, err
		}

		postMessageURL := c.APIURL.Copy()
		postMessageURL.Path += "message/send"
		q := postMessageURL.Query()
		res := make(chan interface{})
		ats.get(c, ctx, res)
		accessToken := ""
		select {
		case <-ctx.Done():
			_ = level.Error(n.logger).Log("msg", "WechatNotifier: get accesstoken timeout")
			return true, fmt.Errorf("get accesstoken timeout")
		case val := <-res:
			switch val.(type) {
			case error:
				_ = level.Error(n.logger).Log("msg", "WechatNotifier: get accesstoken error", "error", val.(error).Error())
				return true, val.(error)
			case string:
				accessToken = val.(string)
			}
		}

		q.Set("access_token", accessToken)
		postMessageURL.RawQuery = q.Encode()

		req, err := http.NewRequest(http.MethodPost, postMessageURL.String(), &buf)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "WechatNotifier: wechat notify error", "error", err.Error())
			return false, err
		}

		resp, err := n.client.Do(req.WithContext(ctx))
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "WechatNotifier: wechat notify error", "error", err.Error())
			return false, err
		}
		defer func() {
			_, _ = io.Copy(ioutil.Discard, resp.Body)
			_ = resp.Body.Close()
		}()

		if resp.StatusCode != 200 {
			_ = level.Error(n.logger).Log("msg", "WechatNotifier: wechat notify error", "status", resp.StatusCode)
			return false, err
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "WechatNotifier: wechat notify error", "error", err)
			return false, err
		}

		var weResp weChatResponse
		if err := jsoniter.Unmarshal(body, &weResp); err != nil {
			_ = level.Error(n.logger).Log("msg", "WechatNotifier: decode error", "error", err)
			return false, err
		}

		// https://work.weixin.qq.com/api/doc#10649
		if weResp.Code == 0 {
			_ = level.Debug(n.logger).Log("msg", "send wechat", "from", c.AgentID, "toUser", c.ToUser, "toParty", c.ToParty, "toTag", c.ToTag)
			return false, nil
		}

		// AccessToken is expired
		if weResp.Code == 42001 {
			ats.invalid(c)
			return true, fmt.Errorf("%s", weResp.Error)
		}

		return false, fmt.Errorf("%s", weResp.Error)
	}

	for _, w := range n.wechat {
		c := n.clone(w)
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

		for {
			if us >= len(toUser) && ps >= len(toParty) && ts >= len(toTag) {
				break
			}

			c.ToUser = batch(toUser, &us, ToUserMax)
			c.ToParty = batch(toParty, &ps, ToPartyMax)
			c.ToTag = batch(toTag, &ts, ToTagMax)

			retry, err := send(c)
			if err != nil {
				if retry {
					_, err = send(c)
				}
				if err != nil {
					errs = append(errs, err)
				}
			}
		}
	}

	return errs
}

func (n *Notifier) getMsg(data template.Data) string {

	msg := fmt.Sprintf("[%d] Firing", len(data.Alerts.Firing()))

	var head []string
	var body []string
	for k, v := range data.CommonLabels {
		if k == "alertname" {
			head = append([]string{fmt.Sprintf("%s = %s", k, v)}, head...)
		} else if k == "namespace" {
			head = append(head, fmt.Sprintf("%s = %s", k, v))
		} else {
			body = append(body, fmt.Sprintf("%s = %s", k, v))
		}
	}

	for _, s := range head {
		msg = fmt.Sprintf("%s\n%s", msg, s)
	}

	for _, s := range body {
		msg = fmt.Sprintf("%s\n%s", msg, s)
	}
	msg = fmt.Sprintf("%s\nDetails: %s", msg, data.ExternalURL)

	return msg
}

func (n *Notifier) clone(c *config.WechatConfig) *config.WechatConfig {

	if c == nil {
		return nil
	}

	wc := &config.WechatConfig{
		NotifierConfig: c.NotifierConfig,
		HTTPConfig:     c.HTTPConfig,
		APISecret:      c.APISecret,
		CorpID:         c.CorpID,
		Message:        c.Message,
		APIURL:         c.APIURL,
		ToUser:         "",
		ToParty:        "",
		ToTag:          "",
		AgentID:        c.AgentID,
	}

	if wc.APIURL == nil || len(wc.APIURL.URL.String()) == 0 {
		url := &config.URL{}
		url.URL, _ = url.Parse(DefaultApiURL)
		wc.APIURL = url
	}

	return wc
}
