package wechat

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/pkg/async"
	"github.com/kubesphere/notification-manager/pkg/constants"
	"github.com/kubesphere/notification-manager/pkg/controller"
	"github.com/kubesphere/notification-manager/pkg/internal"
	"github.com/kubesphere/notification-manager/pkg/internal/wechat"
	"github.com/kubesphere/notification-manager/pkg/notify/notifier"
	"github.com/kubesphere/notification-manager/pkg/template"
	"github.com/kubesphere/notification-manager/pkg/utils"
)

const (
	DefaultApiURL                 = "https://qyapi.weixin.qq.com/cgi-bin/"
	DefaultSendTimeout            = time.Second * 3
	ToUserBatchSize               = 1000
	ToPartyBatchSize              = 100
	ToTagBatchSize                = 100
	AccessTokenInvalid            = 42001
	DefaultTextTemplate           = `{{ template "nm.default.text" . }}`
	DefaultMarkdownTemplate       = `{{ template "nm.default.markdown" . }}`
	MessageMaxSize                = 2048
	DefaultExpires                = time.Hour * 2
	ChatbotMessageMaxTextSize     = 2048
	ChatbotMessageMaxMarkdownSize = 4096
)

type Notifier struct {
	notifierCtl    *controller.Controller
	receiver       *wechat.Receiver
	accessToken    string
	timeout        time.Duration
	logger         log.Logger
	tmpl           *template.Template
	ats            *notifier.AccessTokenService
	messageMaxSize int
	tokenExpires   time.Duration
}

type weChatMessageContent struct {
	Content             string   `json:"content"`
	MentionedList       []string `json:"mentioned_list,omitempty"`
	MentionedMobileList []string `json:"mentioned_mobile_list,omitempty"`
}

type weChatMessage struct {
	Text     weChatMessageContent `yaml:"text,omitempty" json:"text,omitempty"`
	Markdown weChatMessageContent `yaml:"markdown,omitempty" json:"markdown,omitempty"`
	ToUser   string               `yaml:"touser,omitempty" json:"touser,omitempty"`
	ToParty  string               `yaml:"toparty,omitempty" json:"toparty,omitempty"`
	ToTag    string               `yaml:"totag,omitempty" json:"totag,omitempty"`
	AgentID  string               `yaml:"agentid,omitempty" json:"agentid,omitempty"`
	Safe     string               `yaml:"safe,omitempty" json:"safe,omitempty"`
	Type     string               `yaml:"msgtype,omitempty" json:"msgtype,omitempty"`
}

type weChatResponse struct {
	ErrorCode    int    `json:"errcode"`
	ErrorMsg     string `json:"errmsg"`
	AccessToken  string `json:"access_token,omitempty"`
	InvalidUser  string `json:"invaliduser,omitempty"`
	InvalidParty string `json:"invalidparty,omitempty"`
	InvalidTag   string `json:"invalidTag,omitempty"`
}

func NewWechatNotifier(logger log.Logger, receiver internal.Receiver, notifierCtl *controller.Controller) (notifier.Notifier, error) {

	n := &Notifier{
		notifierCtl:    notifierCtl,
		logger:         logger,
		timeout:        DefaultSendTimeout,
		ats:            notifier.GetAccessTokenService(),
		messageMaxSize: MessageMaxSize,
		tokenExpires:   DefaultExpires,
	}

	opts := notifierCtl.ReceiverOpts
	tmplType := constants.Text
	tmplName := ""
	if opts != nil && opts.Global != nil && !utils.StringIsNil(opts.Global.Template) {
		tmplName = opts.Global.Template
	}

	if opts != nil && opts.Wechat != nil {

		if opts.Wechat.NotificationTimeout != nil {
			n.timeout = time.Second * time.Duration(*opts.Wechat.NotificationTimeout)
		}

		if !utils.StringIsNil(opts.Wechat.Template) {
			tmplName = opts.Wechat.Template
		}

		if !utils.StringIsNil(opts.Wechat.TmplType) {
			tmplType = opts.Wechat.TmplType
		}

		if opts.Wechat.MessageMaxSize > 0 {
			n.messageMaxSize = opts.Wechat.MessageMaxSize
		}

		if opts.Wechat.TokenExpires != 0 {
			n.tokenExpires = opts.Wechat.TokenExpires
		}
	}

	n.receiver = receiver.(*wechat.Receiver)
	if n.receiver.Config == nil && n.receiver.ChatBot == nil {
		_ = level.Warn(logger).Log("msg", "WechatNotifier: ignore receiver because of empty config")
		return nil, utils.Error("ignore receiver because of empty config")
	}
	if n.receiver.Config != nil {
		if utils.StringIsNil(n.receiver.APIURL) {
			n.receiver.APIURL = DefaultApiURL
		}
	}

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

	var err error
	n.tmpl, err = notifierCtl.GetReceiverTmpl(n.receiver.TmplText)
	if err != nil {
		_ = level.Error(n.logger).Log("msg", "WechatNotifier: create receiver template error", "error", err.Error())
		return nil, err
	}

	return n, nil
}

func (n *Notifier) Notify(ctx context.Context, data *template.Data) error {

	send := func(r *wechat.Receiver, msg string) error {

		start := time.Now()
		defer func() {
			_ = level.Debug(n.logger).Log("msg", "WechatNotifier: send message", "used", time.Since(start).String())
		}()

		wechatMsg := &weChatMessage{
			ToUser:  utils.ArrayToString(r.ToUser, "|"),
			ToParty: utils.ArrayToString(r.ToParty, "|"),
			ToTag:   utils.ArrayToString(r.ToTag, "|"),
			AgentID: r.AgentID,
			Safe:    "0",
		}
		if r.TmplType == constants.Markdown {
			wechatMsg.Type = constants.Markdown
			wechatMsg.Markdown.Content = msg
		} else if r.TmplType == constants.Text {
			wechatMsg.Type = constants.Text
			wechatMsg.Text.Content = msg
		} else {
			_ = level.Error(n.logger).Log("msg", "WechatNotifier: unkown message type", "type", r.TmplType)
			return utils.Errorf("Unknown message type, %s", r.TmplType)
		}

		sendMessage := func() (bool, error) {

			accessToken, err := n.getToken(ctx, r)
			if err != nil {
				_ = level.Error(n.logger).Log("msg", "WechatNotifier: get access token error", "error", err.Error())
				return false, err
			}

			var buf bytes.Buffer
			if err := utils.JsonEncode(&buf, wechatMsg); err != nil {
				_ = level.Error(n.logger).Log("msg", "WechatNotifier: encode message error", "error", err.Error())
				return false, err
			}

			u, err := utils.UrlWithPath(r.APIURL, "message/send")
			if err != nil {
				_ = level.Error(n.logger).Log("msg", "WechatNotifier: set path error", "error", err)
				return false, err
			}

			parameters := make(map[string]string)
			parameters["access_token"] = accessToken
			u, err = utils.UrlWithParameters(u, parameters)
			if err != nil {
				_ = level.Error(n.logger).Log("msg", "WechatNotifier: set parameters error", "error", err)
				return false, err
			}

			request, err := http.NewRequest(http.MethodPost, u, &buf)
			if err != nil {
				return false, err
			}
			request.Header.Set("Content-Type", "application/json")

			body, err := utils.DoHttpRequest(ctx, nil, request)
			if err != nil {
				_ = level.Error(n.logger).Log("msg", "WechatNotifier: do http error", "error", err)
				return false, err
			}

			var weResp weChatResponse
			if err := utils.JsonUnmarshal(body, &weResp); err != nil {
				_ = level.Error(n.logger).Log("msg", "WechatNotifier: decode response body error", "error", err)
				return false, err
			}

			if weResp.ErrorCode == 0 {
				if weResp.InvalidUser != "" || weResp.InvalidParty != "" || weResp.InvalidTag != "" {
					_ = level.Error(n.logger).Log("msg", "WechatNotifier: send message",
						"from", r.AgentID,
						"InvalidUser", weResp.InvalidUser,
						"InvalidParty", weResp.InvalidParty,
						"InvalidTag", weResp.InvalidTag)
					e := ""
					if weResp.InvalidUser != "" {
						e = fmt.Sprintf("invalid user %s, ", weResp.InvalidUser)
					}
					if weResp.InvalidParty != "" {
						e = fmt.Sprintf("%sinvalid party %s, ", e, weResp.InvalidParty)
					}
					if weResp.InvalidTag != "" {
						e = fmt.Sprintf("%sinvalid tag %s, ", e, weResp.InvalidTag)
					}
					return false, utils.Error(strings.TrimSuffix(e, ", "))
				}
				_ = level.Debug(n.logger).Log("msg", "WechatNotifier: send message",
					"from", r.AgentID,
					"toUser", utils.ArrayToString(r.ToUser, "|"),
					"toParty", utils.ArrayToString(r.ToParty, "|"),
					"toTag", utils.ArrayToString(r.ToTag, "|"))
				return false, nil
			}

			// AccessToken is expired
			if weResp.ErrorCode == AccessTokenInvalid {
				_ = level.Error(n.logger).Log("msg", "WechatNotifier: token expired", "error", err)
				go n.invalidToken(ctx, r)
				return true, utils.Errorf("%d, %s", weResp.ErrorCode, weResp.ErrorMsg)
			}

			_ = level.Error(n.logger).Log("msg", "WechatNotifier: wechat response error",
				"code", weResp.ErrorCode,
				"message", weResp.ErrorMsg,
				"from", r.AgentID,
				"toUser", utils.ArrayToString(r.ToUser, "|"),
				"toParty", utils.ArrayToString(r.ToParty, "|"),
				"toTag", utils.ArrayToString(r.ToTag, "|"))
			return false, utils.Errorf("%d, %s", weResp.ErrorCode, weResp.ErrorMsg)
		}

		retry, err := sendMessage()
		if retry {
			_, err = sendMessage()
		}

		return err
	}

	messages, _, err := n.tmpl.Split(data, MessageMaxSize, n.receiver.TmplName, "", n.logger)
	if err != nil {
		_ = level.Error(n.logger).Log("msg", "WechatNotifier: split message error", "error", err.Error())
		return nil
	}

	us, ps, ts := 0, 0, 0
	toUser := n.receiver.ToUser
	toParty := n.receiver.ToParty
	toTag := n.receiver.ToTag

	group := async.NewGroup(ctx)
	if n.receiver.ChatBot != nil {

		group.Add(func(stopCh chan interface{}) {
			stopCh <- n.sendToChatBot(ctx, data)
		})
	}
	for {
		if us >= len(toUser) && ps >= len(toParty) && ts >= len(toTag) {
			break
		}

		r := n.receiver.Clone().(*wechat.Receiver)
		r.ToUser = batch(toUser, &us, ToUserBatchSize)
		r.ToParty = batch(toParty, &ps, ToPartyBatchSize)
		r.ToTag = batch(toTag, &ts, ToTagBatchSize)

		for index := range messages {
			msg := messages[index]
			group.Add(func(stopCh chan interface{}) {
				stopCh <- send(r, msg)
			})
		}
	}

	return group.Wait()
}

func (n *Notifier) sendToChatBot(ctx context.Context, data *template.Data) error {

	bot := n.receiver.ChatBot

	webhook := bot.Webhook

	send := func(msg string) error {

		start := time.Now()
		defer func() {
			_ = level.Debug(n.logger).Log("msg", "wechatBotNotifier: send message to chatbot", "used", time.Since(start).String())
		}()

		var buf bytes.Buffer

		if n.receiver.TmplType == constants.Markdown {
			chatBotMsgMarkdown := weChatMessage{
				Type: constants.Markdown,
				Markdown: weChatMessageContent{
					Content: msg,
				},
			}
			if err := utils.JsonEncode(&buf, chatBotMsgMarkdown); err != nil {
				_ = level.Error(n.logger).Log("msg", "wechatBotNotifier: encode markdown message error", "error", err.Error())
				return err
			}
		} else if n.receiver.TmplType == constants.Text {
			chatBotMsgText := weChatMessage{
				Type: constants.Text,
				Text: weChatMessageContent{
					Content:             msg,
					MentionedList:       bot.AtUsers,
					MentionedMobileList: bot.AtMobiles,
				},
			}
			if err := utils.JsonEncode(&buf, chatBotMsgText); err != nil {
				_ = level.Error(n.logger).Log("msg", "wechatBotNotifier: encode text message error", "error", err.Error())
				return err
			}
		} else {
			_ = level.Error(n.logger).Log("msg", "wechatBotkNotifier: unknown message type", "type", n.receiver.TmplType)
			return utils.Errorf("Unknown message type, %s", n.receiver.TmplType)
		}
		webhookStr, err := n.notifierCtl.GetCredential(webhook)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "WechatNotifier: get webhook error", "error", err)
			return err
		}
		request, err := http.NewRequest(http.MethodPost, webhookStr, &buf)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "wechatBotNotifier: create http request error", "error", err)
			return err
		}
		request.Header.Set("Content-Type", "application/json")

		body, err := utils.DoHttpRequest(context.Background(), nil, request)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "wechatBotNotifier: do http error", "error", err)
			return err
		}

		res := &weChatResponse{}
		if err := utils.JsonUnmarshal(body, res); err != nil {
			_ = level.Error(n.logger).Log("msg", "wechatBotNotifier: decode response body error", "error", err)
			return err
		}

		if res.ErrorCode != 0 {
			_ = level.Error(n.logger).Log("msg", "wechatBotNotifier: send message to chatbot error", "errcode", res.ErrorCode, "errmsg", res.ErrorMsg)
			return utils.Errorf("%d, %s", res.ErrorCode, res.ErrorMsg)
		}

		_ = level.Debug(n.logger).Log("msg", "wechatBotNotifier: send message to chatbot")

		return nil
	}

	atUsers := ""
	// Note that in markdown format, you can only use <@userid> to @someone, @all is not supported, and @phone number is not allowed.
	if len(bot.AtUsers) > 0 {
		var temp []string
		for _, user := range bot.AtUsers {
			temp = append(temp, fmt.Sprintf("<@%s>", user))
		}
		atUsers = strings.Join(temp, "")
	}

	// Need to limit the length of the message, the longest is not more than 4096 bytes.
	var msgSize int
	if n.receiver.TmplType == constants.Markdown {
		msgSize = ChatbotMessageMaxMarkdownSize - len(atUsers)
	} else {
		msgSize = ChatbotMessageMaxTextSize
	}
	messages, _, err := n.tmpl.Split(data, msgSize, n.receiver.TmplName, n.receiver.TitleTmplName, n.logger)
	if err != nil {
		_ = level.Error(n.logger).Log("msg", "wechatBotNotifier: split message error", "error", err.Error())
		return err
	}
	group := async.NewGroup(ctx)
	for index := range messages {
		msg := fmt.Sprintf("%s", messages[index])
		if n.receiver.TmplType == constants.Markdown {
			msg = fmt.Sprintf("%s\n%s", msg, atUsers)
		}
		group.Add(func(stopCh chan interface{}) {
			stopCh <- send(msg)
		})
	}

	return group.Wait()
}

func (n *Notifier) getToken(ctx context.Context, r *wechat.Receiver) (string, error) {

	apiSecret, err := n.notifierCtl.GetCredential(r.APISecret)
	if err != nil {
		return "", err
	}

	get := func(ctx context.Context) (string, time.Duration, error) {
		u := r.APIURL
		u, err := utils.UrlWithPath(u, "gettoken")
		if err != nil {
			return "", 0, err
		}

		parameters := make(map[string]string)
		parameters["corpsecret"] = apiSecret
		parameters["corpid"] = r.CorpID
		u, err = utils.UrlWithParameters(u, parameters)
		if err != nil {
			return "", 0, err
		}

		var request *http.Request
		request, err = http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			return "", 0, err
		}
		request.Header.Set("Content-Type", "application/json")

		body, err := utils.DoHttpRequest(ctx, nil, request)
		if err != nil {
			return "", 0, err
		}

		resp := &weChatResponse{}
		err = utils.JsonUnmarshal(body, resp)
		if err != nil {
			return "", 0, err
		}

		if resp.ErrorCode != 0 {
			return "", 0, utils.Error(resp.ErrorMsg)
		}

		_ = level.Debug(n.logger).Log("msg", "WechatNotifier: get token", "key", r.CorpID+" | "+r.AgentID)
		return resp.AccessToken, DefaultExpires, nil
	}

	return n.ats.GetToken(ctx, r.CorpID+" | "+apiSecret, get)
}

func (n *Notifier) invalidToken(ctx context.Context, r *wechat.Receiver) {
	key := r.CorpID + " | " + r.AgentID
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
