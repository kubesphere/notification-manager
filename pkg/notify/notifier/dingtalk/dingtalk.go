package dingtalk

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/pkg/async"
	"github.com/kubesphere/notification-manager/pkg/constants"
	"github.com/kubesphere/notification-manager/pkg/controller"
	"github.com/kubesphere/notification-manager/pkg/internal"
	"github.com/kubesphere/notification-manager/pkg/internal/dingtalk"
	"github.com/kubesphere/notification-manager/pkg/notify/notifier"
	"github.com/kubesphere/notification-manager/pkg/template"
	"github.com/kubesphere/notification-manager/pkg/utils"
)

const (
	URL                          = "https://oapi.dingtalk.com/"
	DefaultSendTimeout           = time.Second * 3
	DefaultTextTemplate          = `{{ template "nm.default.text" . }}`
	DefaultTitleTemplate         = `{{ template "nm.default.subject" . }}`
	DefaultMarkdownTemplate      = `{{ template "nm.default.markdown" . }}`
	ConversationMessageMaxSize   = 5000
	ChatbotMessageMaxSize        = 19960
	DefaultExpires               = time.Hour * 2
	DefaultChatbotThreshold      = 20
	DefaultChatbotUnit           = time.Minute
	DefaultChatbotWaitTime       = time.Second * 10
	DefaultConversationThreshold = 25
	DefaultConversationUnit      = time.Second
)

type Notifier struct {
	notifierCtl                *controller.Controller
	receiver                   *dingtalk.Receiver
	timeout                    time.Duration
	logger                     log.Logger
	tmpl                       *template.Template
	throttle                   *Throttle
	ats                        *notifier.AccessTokenService
	tokenExpires               time.Duration
	conversationMessageMaxSize int
	chatbotMessageMaxSize      int
	chatbotThreshold           int
	chatbotUnit                time.Duration
	chatbotMaxWaitTime         time.Duration
	conversationThreshold      int
	conversationUnit           time.Duration
	conversationMaxWaitTime    time.Duration
}

type dingtalkText struct {
	Content string `json:"content"`
}

type dingtalkMarkdown struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

type at struct {
	AtMobiles []string `yaml:"atMobiles,omitempty" json:"atMobiles,omitempty"`
	AtUserIds []string `yaml:"atUserIds,omitempty" json:"atUserIds,omitempty"`
	IsAtAll   bool     `yaml:"isAtAll,omitempty" json:"isAtAll,omitempty"`
}

type dingtalkChatBotMessage struct {
	Markdown dingtalkMarkdown `yaml:"markdown,omitempty" json:"markdown,omitempty"`
	Text     dingtalkText     `yaml:"text,omitempty" json:"text,omitempty"`
	Type     string           `yaml:"msgtype,omitempty" json:"msgtype,omitempty"`
	At       at               `yaml:"at,omitempty" json:"at,omitempty"`
}

type dingtalkConversationMessage struct {
	ID      string                 `yaml:"chatid,omitempty" json:"chatid,omitempty"`
	Message dingtalkChatBotMessage `yaml:"msg,omitempty" json:"msg,omitempty"`
}

type response struct {
	Code    int    `json:"errcode"`
	Message string `json:"errmsg"`
	Token   string `json:"access_token"`
	Status  int    `json:"status"`
	Punish  string `json:"punish"`
}

func NewDingTalkNotifier(logger log.Logger, receiver internal.Receiver, notifierCtl *controller.Controller) (notifier.Notifier, error) {

	n := &Notifier{
		notifierCtl:                notifierCtl,
		timeout:                    DefaultSendTimeout,
		logger:                     logger,
		throttle:                   GetThrottle(),
		ats:                        notifier.GetAccessTokenService(),
		tokenExpires:               DefaultExpires,
		conversationMessageMaxSize: ConversationMessageMaxSize,
		chatbotMessageMaxSize:      ChatbotMessageMaxSize,
		chatbotThreshold:           DefaultChatbotThreshold,
		chatbotUnit:                DefaultChatbotUnit,
		chatbotMaxWaitTime:         DefaultChatbotWaitTime,
		conversationThreshold:      DefaultConversationThreshold,
		conversationUnit:           DefaultConversationUnit,
		conversationMaxWaitTime:    DefaultConversationUnit,
	}

	opts := notifierCtl.ReceiverOpts
	tmplType := constants.Text
	tmplName := ""
	titleTmplName := DefaultTitleTemplate
	if opts != nil && opts.Global != nil && !utils.StringIsNil(opts.Global.Template) {
		tmplName = opts.Global.Template
	}

	if opts != nil && opts.DingTalk != nil {

		d := opts.DingTalk
		if d.NotificationTimeout != nil {
			n.timeout = time.Second * time.Duration(*d.NotificationTimeout)
		}

		if !utils.StringIsNil(d.Template) {
			tmplName = d.Template
		}

		if !utils.StringIsNil(d.TitleTemplate) {
			titleTmplName = d.TitleTemplate
		}

		if !utils.StringIsNil(d.TmplType) {
			tmplType = d.TmplType
		}

		if d.TokenExpires != 0 {
			n.tokenExpires = d.TokenExpires
		}

		if d.ConversationMessageMaxSize > 0 {
			n.conversationMessageMaxSize = d.ConversationMessageMaxSize
		}

		if d.ChatbotMessageMaxSize > 0 {
			n.chatbotMessageMaxSize = d.ChatbotMessageMaxSize
		}

		if d.ChatBotThrottle != nil {
			t := d.ChatBotThrottle
			if t.Threshold > 0 {
				n.chatbotThreshold = t.Threshold
			}

			if t.Unit != 0 {
				n.chatbotUnit = t.Unit
			}

			if t.MaxWaitTime != 0 {
				n.chatbotMaxWaitTime = t.MaxWaitTime
			}
		}

		if d.ConversationThrottle != nil {
			t := d.ConversationThrottle
			if t.Threshold > 0 {
				n.conversationThreshold = t.Threshold
			}

			if t.Unit != 0 {
				n.conversationUnit = t.Unit
			}

			if t.MaxWaitTime != 0 {
				n.conversationMaxWaitTime = t.MaxWaitTime
			}
		}
	}

	n.receiver = receiver.(*dingtalk.Receiver)

	// If the template type of receiver is not set, use the global template type.
	if utils.StringIsNil(n.receiver.TmplType) {
		n.receiver.TmplType = tmplType
	}

	if utils.StringIsNil(n.receiver.TmplName) {
		if tmplName != "" {
			n.receiver.TmplName = tmplName
		} else {
			if n.receiver.TmplType == constants.Markdown {
				n.receiver.TmplName = DefaultMarkdownTemplate
			} else if n.receiver.TmplType == constants.Text {
				n.receiver.TmplName = DefaultTextTemplate
			}
		}
	}

	if utils.StringIsNil(n.receiver.TitleTmplName) && n.receiver.TmplType == constants.Markdown {
		n.receiver.TitleTmplName = titleTmplName
	}

	var err error
	n.tmpl, err = notifierCtl.GetReceiverTmpl(n.receiver.TmplText)
	if err != nil {
		_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: create receiver template error", "error", err.Error())
		return nil, err
	}

	return n, nil
}

func (n *Notifier) Notify(ctx context.Context, data *template.Data) error {

	group := async.NewGroup(ctx)
	if n.receiver.ChatBot != nil {
		group.Add(func(stopCh chan interface{}) {
			stopCh <- n.sendToChatBot(ctx, data)
		})
	}

	if len(n.receiver.ChatIDs) > 0 {
		group.Add(func(stopCh chan interface{}) {
			stopCh <- n.sendToConversation(ctx, data)
		})
	}

	return group.Wait()
}

func (n *Notifier) sendToChatBot(ctx context.Context, data *template.Data) error {

	bot := n.receiver.ChatBot

	webhook, err := n.notifierCtl.GetCredential(bot.Webhook)
	if err != nil {
		_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: get webhook secret error", "error", err.Error())
		return err
	}

	send := func(title, msg string) error {
		// end
		start := time.Now()
		defer func() {
			_ = level.Debug(n.logger).Log("msg", "DingTalkNotifier: send message to chatbot", "used", time.Since(start).String())
		}()

		chatBotMsg := dingtalkChatBotMessage{
			Type: n.receiver.TmplType,
			At: at{
				AtMobiles: bot.AtMobiles,
				AtUserIds: bot.AtUsers,
				IsAtAll:   bot.AtAll,
			},
		}

		if n.receiver.TmplType == constants.Markdown {
			chatBotMsg.Markdown.Title = title
			chatBotMsg.Markdown.Text = msg
		} else if n.receiver.TmplType == constants.Text {
			chatBotMsg.Text.Content = msg
		} else {
			_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: unknown message type", "type", n.receiver.TmplType)
			return utils.Errorf("Unknown message type, %s", n.receiver.TmplType)
		}

		var buf bytes.Buffer
		if err := utils.JsonEncode(&buf, chatBotMsg); err != nil {
			_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: encode text message error", "error", err.Error())
			return err
		}

		secret := ""
		if bot.Secret != nil {
			secret, err = n.notifierCtl.GetCredential(bot.Secret)
			if err != nil {
				_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: get chatbot secret error", "error", err.Error())
				return err
			}
		}

		u := webhook
		if !utils.StringIsNil(secret) {
			timestamp, sign := calcSign(secret)
			p := make(map[string]string)
			p["timestamp"] = timestamp
			p["sign"] = sign
			u, err = utils.UrlWithParameters(webhook, p)
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

		body, err := utils.DoHttpRequest(context.Background(), nil, request)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: do http error", "error", err)
			return err
		}

		res := &response{}
		if err := utils.JsonUnmarshal(body, res); err != nil {
			_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: decode response body error", "error", err)
			return err
		}

		if res.Code != 0 {
			_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: send message to chatbot error", "errcode", res.Code, "errmsg", res.Message)
			return utils.Errorf("%d, %s", res.Code, res.Message)
		}

		if res.Status != 0 {
			_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: send message to chatbot error", "status", res.Status, "punish", res.Punish)
			return utils.Errorf("%d, %s", res.Status, res.Punish)
		}

		_ = level.Debug(n.logger).Log("msg", "DingTalkNotifier: send message to chatbot")

		return nil
	}

	keywords := ""
	if len(bot.Keywords) > 0 {
		keywords = "\n\n[Keywords] "
		for _, k := range bot.Keywords {
			keywords = fmt.Sprintf("%s%s, ", keywords, k)
		}

		keywords = strings.TrimSuffix(keywords, ", ")
	}

	atMobiles := ""
	if len(bot.AtMobiles) > 0 {
		for _, mobile := range bot.AtMobiles {
			atMobiles = fmt.Sprintf("%s@%s, ", atMobiles, mobile)
		}

		atMobiles = strings.TrimSuffix(atMobiles, ", ")
	}

	maxSize := n.chatbotMessageMaxSize - len(keywords)
	// The mobiles must be in the message when the message format is markdown.
	if n.receiver.TmplType == constants.Markdown {
		maxSize = maxSize - len(atMobiles)
	}

	messages, titles, err := n.tmpl.Split(data, n.chatbotMessageMaxSize-len(keywords)-len(atMobiles), n.receiver.TmplName, n.receiver.TitleTmplName, n.logger)
	if err != nil {
		_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: split message error", "error", err.Error())
		return err
	}

	group := async.NewGroup(ctx)
	for index := range messages {
		title := titles[index]
		msg := fmt.Sprintf("%s%s", messages[index], keywords)
		if n.receiver.TmplType == constants.Markdown {
			msg = fmt.Sprintf("%s %s", msg, atMobiles)
		}
		group.Add(func(stopCh chan interface{}) {
			n.throttle.TryAdd(webhook, n.chatbotThreshold, n.chatbotUnit, n.chatbotMaxWaitTime)
			if n.throttle.Allow(webhook, n.logger) {
				stopCh <- send(title, msg)
			} else {
				_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: message to chatbot dropped because of flow control")
				stopCh <- utils.Error("")
			}
		})
	}

	return group.Wait()
}

func (n *Notifier) sendToConversation(ctx context.Context, data *template.Data) error {

	if n.receiver.Config == nil {
		_ = level.Debug(n.logger).Log("msg", "DingTalkNotifier: config is nil")
		return utils.Error("DingTalkNotifier: config is nil")
	}

	appkey, err := n.notifierCtl.GetCredential(n.receiver.AppKey)
	if err != nil {
		_ = level.Debug(n.logger).Log("msg", "DingTalkNotifier: get appkey error", "error", err)
		return err
	}

	appsecret, err := n.notifierCtl.GetCredential(n.receiver.AppSecret)
	if err != nil {
		_ = level.Debug(n.logger).Log("msg", "DingTalkNotifier: get appsecret error", "error", err)
		return err
	}

	send := func(chatID, title, msg string) error {
		start := time.Now()
		defer func() {
			_ = level.Debug(n.logger).Log("msg", "DingTalkNotifier: send message to conversation", "conversation", chatID, "used", time.Since(start).String())
		}()

		token, err := n.getToken(ctx, appkey, appsecret)
		if err != nil {
			_ = level.Debug(n.logger).Log("msg", "DingTalkNotifier: get token error", "conversation", chatID, "error", err)
			return err
		}

		conversationMsg := dingtalkConversationMessage{
			ID: chatID,
			Message: dingtalkChatBotMessage{
				Type: n.receiver.TmplType,
			},
		}

		if n.receiver.TmplType == constants.Markdown {
			conversationMsg.Message.Markdown.Title = title
			conversationMsg.Message.Markdown.Text = msg
		} else if n.receiver.TmplType == constants.Text {
			conversationMsg.Message.Text.Content = msg
		} else {
			_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: unknown message type", "conversation", chatID, "type", n.receiver.TmplType)
			return utils.Errorf("Unknown message type, %s", n.receiver.TmplType)
		}

		var buf bytes.Buffer
		if err := utils.JsonEncode(&buf, conversationMsg); err != nil {
			_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: encode markdown message error", "conversation", chatID, "error", err.Error())
			return err
		}

		u, err := utils.UrlWithPath(URL, "chat/send")
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: set path error", "conversation", chatID, "error", err)
			return err
		}

		p := make(map[string]string)
		p["access_token"] = token
		u, err = utils.UrlWithParameters(u, p)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: set parameters error", "conversation", chatID, "error", err)
			return err
		}

		request, err := http.NewRequest(http.MethodPost, u, &buf)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: create http request error", "conversation", chatID, "error", err)
			return err
		}
		request.Header.Set("Content-Type", "application/json")

		body, err := utils.DoHttpRequest(context.Background(), nil, request)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: do http error", "conversation", chatID, "error", err)
			return err
		}

		res := &response{}
		if err := utils.JsonUnmarshal(body, res); err != nil {
			_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: decode response body error", "conversation", chatID, "error", err)
			return err
		}

		if res.Code != 0 {
			_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: send message to conversation error", "conversation", chatID, "errcode", res.Code, "errmsg", res.Message)
			return utils.Errorf("%d, %s", res.Code, res.Message)
		}

		if res.Status != 0 {
			_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: send message to conversation error", "conversation", chatID, "status", res.Status, "punish", res.Punish)
			return utils.Errorf("%d, %s", res.Status, res.Punish)
		}

		_ = level.Debug(n.logger).Log("msg", "DingTalkNotifier: send message to conversation", "conversation", chatID)

		return nil
	}

	messages, titles, err := n.tmpl.Split(data, n.conversationMessageMaxSize, n.receiver.TmplName, n.receiver.TitleTmplName, n.logger)
	if err != nil {
		_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: split message error", "error", err.Error())
		return nil
	}

	group := async.NewGroup(ctx)
	for index := range messages {
		title := titles[index]
		msg := messages[index]
		for _, chatID := range n.receiver.ChatIDs {
			id := chatID
			group.Add(func(stopCh chan interface{}) {
				n.throttle.TryAdd(appkey, n.conversationThreshold, n.conversationUnit, n.conversationMaxWaitTime)
				if n.throttle.Allow(appkey, n.logger) {
					stopCh <- send(id, title, msg)
				} else {
					_ = level.Error(n.logger).Log("msg", "DingTalkNotifier: message to conversation dropped because of flow control", "conversation", chatID)
					stopCh <- utils.Error("")
				}
			})
		}
	}

	return group.Wait()
}

func (n *Notifier) getToken(ctx context.Context, appkey, appsecret string) (string, error) {

	get := func(ctx context.Context) (string, time.Duration, error) {
		u, err := utils.UrlWithPath(URL, "gettoken")
		if err != nil {
			return "", 0, err
		}

		p := make(map[string]string)
		p["appkey"] = appkey
		p["appsecret"] = appsecret

		u, err = utils.UrlWithParameters(u, p)
		if err != nil {
			return "", 0, err
		}

		request, err := http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			return "", 0, err
		}
		request.Header.Set("Content-Type", "application/json")

		body, err := utils.DoHttpRequest(context.Background(), nil, request)
		if err != nil {
			return "", 0, err
		}

		res := &response{}
		if err := utils.JsonUnmarshal(body, res); err != nil {
			return "", 0, err
		}

		if res.Code != 0 {
			return "", 0, utils.Errorf("%d, %s", res.Code, res.Message)
		}

		_ = level.Debug(n.logger).Log("msg", "DingTalkNotifier: get token", "key", appkey+" | "+appsecret)
		return res.Token, n.tokenExpires, nil
	}

	return n.ats.GetToken(ctx, appkey+" | "+appsecret, get)
}

func calcSign(secret string) (string, string) {

	timestamp := fmt.Sprintf("%d", time.Now().Unix()*1000)
	msg := fmt.Sprintf("%s\n%s", timestamp, secret)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(msg))
	sign := base64.StdEncoding.EncodeToString(h.Sum(nil))

	return timestamp, url.QueryEscape(sign)
}
