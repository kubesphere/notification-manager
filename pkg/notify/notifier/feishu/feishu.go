package feishu

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	json "github.com/json-iterator/go"

	"github.com/kubesphere/notification-manager/pkg/async"
	"github.com/kubesphere/notification-manager/pkg/constants"
	"github.com/kubesphere/notification-manager/pkg/controller"
	"github.com/kubesphere/notification-manager/pkg/internal"
	"github.com/kubesphere/notification-manager/pkg/internal/feishu"
	"github.com/kubesphere/notification-manager/pkg/notify/notifier"
	"github.com/kubesphere/notification-manager/pkg/template"
	"github.com/kubesphere/notification-manager/pkg/utils"
)

const (
	TokenAPI            = "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal"
	BatchAPI            = "https://open.feishu.cn/open-apis/message/v4/batch_send/"
	DefaultSendTimeout  = time.Second * 3
	DefaultPostTemplate = `{{ template "nm.feishu.post" . }}`
	DefaultTextTemplate = `{{ template "nm.feishu.text" . }}`
	DefaultExpires      = time.Hour * 2
	ExceedLimitCode     = 9499
)

type Notifier struct {
	notifierCtl  *controller.Controller
	receiver     *feishu.Receiver
	timeout      time.Duration
	logger       log.Logger
	tmpl         *template.Template
	ats          *notifier.AccessTokenService
	tokenExpires time.Duration

	sentSuccessfulHandler *func([]*template.Alert)
}

type Message struct {
	MsgType    string         `json:"msg_type"`
	Content    messageContent `json:"content"`
	Department []string       `json:"department_ids,omitempty"`
	User       []string       `json:"user_ids,omitempty"`
	Timestamp  int64          `json:"timestamp,omitempty"`
	Sign       string         `json:"sign,omitempty"`
}

type messageContent struct {
	Post interface{} `json:"post,omitempty"`
	Text string      `json:"text,omitempty"`
}

type Response struct {
	Code        int          `json:"code"`
	Msg         string       `json:"msg"`
	AccessToken string       `json:"tenant_access_token"`
	Data        responseData `json:"data,omitempty"`
	Expire      int          `json:"expire"`
}

type responseData struct {
	InvalidDepartment []string `json:"invalid_department_ids"`
	InvalidUser       []string `json:"invalid_user_ids"`
}

func NewFeishuNotifier(logger log.Logger, receiver internal.Receiver, notifierCtl *controller.Controller) (notifier.Notifier, error) {

	n := &Notifier{
		notifierCtl:  notifierCtl,
		logger:       logger,
		timeout:      DefaultSendTimeout,
		ats:          notifier.GetAccessTokenService(),
		tokenExpires: DefaultExpires,
	}

	opts := notifierCtl.ReceiverOpts
	tmplType := constants.Post
	tmplName := ""
	if opts != nil && opts.Global != nil && !utils.StringIsNil(opts.Global.Template) {
		tmplName = opts.Global.Template
	}

	if opts != nil && opts.Feishu != nil {

		if opts.Feishu.NotificationTimeout != nil {
			n.timeout = time.Second * time.Duration(*opts.Feishu.NotificationTimeout)
		}

		if !utils.StringIsNil(opts.Feishu.Template) {
			tmplName = opts.Feishu.Template
		}

		if !utils.StringIsNil(opts.Feishu.TmplType) {
			tmplType = opts.Feishu.TmplType
		}

		if opts.Feishu.TokenExpires != 0 {
			n.tokenExpires = opts.Feishu.TokenExpires
		}
	}

	n.receiver = receiver.(*feishu.Receiver)
	if utils.StringIsNil(n.receiver.TmplType) {
		n.receiver.TmplType = tmplType
	}

	if utils.StringIsNil(n.receiver.TmplName) {
		if tmplName != "" {
			n.receiver.TmplName = tmplName
		} else {
			if n.receiver.TmplType == constants.Post {
				n.receiver.TmplName = DefaultPostTemplate
			} else if n.receiver.TmplType == constants.Text {
				n.receiver.TmplName = DefaultTextTemplate
			}
		}
	}

	var err error
	n.tmpl, err = notifierCtl.GetReceiverTmpl(n.receiver.TmplText)
	if err != nil {
		_ = level.Error(n.logger).Log("msg", "FeishuNotifier: create receiver template error", "error", err.Error())
		return nil, err
	}

	return n, nil
}

func (n *Notifier) SetSentSuccessfulHandler(h *func([]*template.Alert)) {
	n.sentSuccessfulHandler = h
}

func (n *Notifier) Notify(ctx context.Context, data *template.Data) error {
	content, err := n.tmpl.Text(n.receiver.TmplName, data)
	if err != nil {
		_ = level.Error(n.logger).Log("msg", "FeishuNotifier: generate message error", "error", err.Error())
		return err
	}

	group := async.NewGroup(ctx)
	if n.receiver.ChatBot != nil {
		group.Add(func(stopCh chan interface{}) {
			err := n.sendToChatBot(ctx, content)
			if err == nil {
				if n.sentSuccessfulHandler != nil {
					(*n.sentSuccessfulHandler)(data.Alerts)
				}
			}
			stopCh <- err
		})
	}

	if len(n.receiver.User) > 0 || len(n.receiver.Department) > 0 {
		group.Add(func(stopCh chan interface{}) {
			err := n.batchSend(ctx, content)
			if err == nil {
				if n.sentSuccessfulHandler != nil {
					(*n.sentSuccessfulHandler)(data.Alerts)
				}
			}
			stopCh <- err
		})
	}

	return group.Wait()
}

func (n *Notifier) sendToChatBot(ctx context.Context, content string) error {
	keywords := ""
	if len(n.receiver.ChatBot.Keywords) != 0 {
		keywords = fmt.Sprintf("[Keywords] %s", utils.ArrayToString(n.receiver.ChatBot.Keywords, ","))
	}

	message := &Message{MsgType: n.receiver.TmplType}
	if n.receiver.TmplType == constants.Post {
		post := make(map[string]interface{})
		if err := json.Unmarshal([]byte(content), &post); err != nil {
			_ = level.Error(n.logger).Log("msg", "FeishuNotifier: unmarshal failed", "error", err)
			return err
		}

		if len(keywords) > 0 {
			for k, v := range post {
				p := v.(map[string]interface{})
				items := p["content"].([]interface{})
				items = append(items, []interface{}{
					map[string]string{
						"tag":  "text",
						"text": keywords,
					},
				})
				p["content"] = items
				post[k] = p
			}
		}

		message.Content.Post = post
	} else if n.receiver.TmplType == constants.Text {
		message.Content.Text = content
		if len(keywords) > 0 {
			message.Content.Text = fmt.Sprintf("%s\n\n%s", content, keywords)
		}
	} else {
		_ = level.Error(n.logger).Log("msg", "FeishuNotifier: unknown message type", "type", n.receiver.TmplType)
		return utils.Errorf("Unknown message type, %s", n.receiver.TmplType)
	}

	if n.receiver.ChatBot.Secret != nil {
		secret, err := n.notifierCtl.GetCredential(n.receiver.ChatBot.Secret)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "FeishuNotifier: get secret error", "error", err.Error())
			return err
		}

		message.Timestamp = time.Now().Unix()
		message.Sign, err = genSign(secret, message.Timestamp)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "FeishuNotifier: calculate signature error", "error", err.Error())
			return err
		}
	}

	send := func() (bool, error) {
		webhook, err := n.notifierCtl.GetCredential(n.receiver.ChatBot.Webhook)
		if err != nil {
			return false, err
		}

		var buf bytes.Buffer
		if err := utils.JsonEncode(&buf, message); err != nil {
			return false, err
		}

		request, err := http.NewRequest(http.MethodPost, webhook, &buf)
		if err != nil {
			return false, err
		}
		request.Header.Set("Content-Type", "application/json; charset=utf-8")

		respBody, err := utils.DoHttpRequest(ctx, nil, request)
		if err != nil && len(respBody) == 0 {
			return false, err
		}

		var resp Response
		if err := utils.JsonUnmarshal(respBody, &resp); err != nil {
			return false, err
		}

		if resp.Code == 0 {
			return false, nil
		}

		// 9499 means the API call exceeds the limit, need to retry.
		if resp.Code == ExceedLimitCode {
			return true, utils.Errorf("%d, %s", resp.Code, resp.Msg)
		}

		return false, utils.Errorf("%d, %s", resp.Code, resp.Msg)
	}

	start := time.Now()
	defer func() {
		_ = level.Debug(n.logger).Log("msg", "FeishuNotifier: send message to chatbot", "used", time.Since(start).String())
	}()

	retry := 0
	// The retries will continue until the send times out and the context is cancelled.
	// There is only one case that triggers the retry mechanism, that is, the API call exceeds the limit.
	// The maximum frequency for sending notifications to chatbot is 5 times/second and 100 times/minute.
	for {
		needRetry, err := send()
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "FeishuNotifier: send notification to chatbot error", "error", err.Error())
		}
		if needRetry {
			retry = retry + 1
			time.Sleep(time.Second)
			_ = level.Info(n.logger).Log("msg", "FeishuNotifier: retry to send notification to chatbot", "retry", retry)
			continue
		}

		return err
	}
}

func (n *Notifier) batchSend(ctx context.Context, content string) error {
	message := &Message{MsgType: n.receiver.TmplType}
	if n.receiver.TmplType == constants.Post {
		post := make(map[string]interface{})
		if err := json.Unmarshal([]byte(content), &post); err != nil {
			_ = level.Error(n.logger).Log("msg", "FeishuNotifier: unmarshal failed", "error", err)
			return err
		}
		message.Content.Post = post
	} else if n.receiver.TmplType == constants.Text {
		message.Content.Text = content
	} else {
		_ = level.Error(n.logger).Log("msg", "FeishuNotifier: unknown message type", "type", n.receiver.TmplType)
		return utils.Errorf("Unknown message type, %s", n.receiver.TmplType)
	}

	message.User = n.receiver.User
	message.Department = n.receiver.Department

	send := func(retry int) (bool, error) {
		if n.receiver.Config == nil {
			_ = level.Error(n.logger).Log("msg", "FeishuNotifier: config is nil")
			return false, utils.Error("FeishuNotifier: config is nil")
		}

		accessToken, err := n.getToken(ctx, n.receiver)
		if err != nil {
			return false, err
		}

		var buf bytes.Buffer
		if err := utils.JsonEncode(&buf, message); err != nil {
			return false, err
		}

		request, err := http.NewRequest(http.MethodPost, BatchAPI, &buf)
		if err != nil {
			return false, err
		}
		request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
		request.Header.Set("Content-Type", "application/json; charset=utf-8")

		respBody, err := utils.DoHttpRequest(ctx, nil, request)
		if err != nil {
			return false, err
		}

		var resp Response
		if err := utils.JsonUnmarshal(respBody, &resp); err != nil {
			return false, err
		}

		if resp.Code == 0 {
			if len(resp.Data.InvalidUser) > 0 || len(resp.Data.InvalidDepartment) > 0 {
				e := ""
				if len(resp.Data.InvalidUser) > 0 {
					e = fmt.Sprintf("invalid user %s, ", resp.Data.InvalidUser)
				}
				if len(resp.Data.InvalidDepartment) > 0 {
					e = fmt.Sprintf("%sinvalid department %s, ", e, resp.Data.InvalidDepartment)
				}

				return false, utils.Error(strings.TrimSuffix(e, ", "))
			}

			return false, nil
		}

		// 9499 means the API call exceeds the limit, need to retry.
		if resp.Code == ExceedLimitCode {
			return true, utils.Errorf("%d, %s", resp.Code, resp.Msg)
		}

		return false, utils.Errorf("%d, %s", resp.Code, resp.Msg)
	}

	start := time.Now()
	defer func() {
		_ = level.Debug(n.logger).Log("msg", "FeishuNotifier: send message", "used", time.Since(start).String(),
			"user", utils.ArrayToString(n.receiver.User, ","),
			"department", utils.ArrayToString(n.receiver.Department, ","))
	}()

	retry := 0
	// The retries will continue until the send times out and the context is cancelled.
	// There is only one case that triggers the retry mechanism, that is, the API call exceeds the limit.
	// The maximum frequency for sending notifications to the same user is 5 times/second.
	for {
		needRetry, err := send(retry)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "FeishuNotifier: send notification error", "error", err, "retry", retry)
		}
		if needRetry {
			retry = retry + 1
			time.Sleep(time.Second)
			_ = level.Info(n.logger).Log("msg", "FeishuNotifier: retry to send notification", "retry", retry)
			continue
		}

		return err
	}
}

func (n *Notifier) getToken(ctx context.Context, r *feishu.Receiver) (string, error) {

	appID, err := n.notifierCtl.GetCredential(r.AppID)
	if err != nil {
		return "", err
	}

	appSecret, err := n.notifierCtl.GetCredential(r.AppSecret)
	if err != nil {
		return "", err
	}

	get := func(ctx context.Context) (string, time.Duration, error) {

		body := make(map[string]string)
		body["app_id"] = appID
		body["app_secret"] = appSecret

		var buf bytes.Buffer
		if err := utils.JsonEncode(&buf, body); err != nil {
			_ = level.Error(n.logger).Log("msg", "FeishuNotifier: encode message error", "error", err.Error())
			return "", 0, err
		}

		var request *http.Request
		request, err = http.NewRequest(http.MethodPost, TokenAPI, &buf)
		if err != nil {
			return "", 0, err
		}
		request.Header.Set("Content-Type", "application/json; charset=utf-8")

		respBody, err := utils.DoHttpRequest(ctx, nil, request)
		if err != nil {
			return "", 0, err
		}

		resp := &Response{}
		err = utils.JsonUnmarshal(respBody, resp)
		if err != nil {
			return "", 0, err
		}

		if resp.Code != 0 {
			return "", 0, utils.Errorf("%d, %s", resp.Code, resp.Msg)
		}

		_ = level.Debug(n.logger).Log("msg", "FeishuNotifier: get token", "key", appID)
		return resp.AccessToken, time.Duration(resp.Expire) * time.Second, nil
	}

	return n.ats.GetToken(ctx, appID+" | "+appSecret, get)
}

func genSign(secret string, timestamp int64) (string, error) {

	stringToSign := fmt.Sprintf("%v", timestamp) + "\n" + secret
	var data []byte
	h := hmac.New(sha256.New, []byte(stringToSign))
	_, err := h.Write(data)
	if err != nil {
		return "", err
	}
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))
	return signature, nil
}
