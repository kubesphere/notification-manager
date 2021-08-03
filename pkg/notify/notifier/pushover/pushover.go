package pushover

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/kubesphere/notification-manager/pkg/apis/v2beta2"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/pkg/async"
	"github.com/kubesphere/notification-manager/pkg/notify/config"
	"github.com/kubesphere/notification-manager/pkg/notify/notifier"
	"github.com/kubesphere/notification-manager/pkg/utils"
	"github.com/prometheus/alertmanager/template"
)

const (
	DefaultSendTimeout = time.Second * 3
	URL                = "https://api.pushover.net/1/messages.json"
	DefaultTemplate    = `{{ template "nm.default.text" . }}`
	MessageMaxLength   = 1024
)

type Notifier struct {
	notifierCfg  *config.Config
	pushover     []*config.Pushover
	timeout      time.Duration
	logger       log.Logger
	template     *notifier.Template
	templateName string
	client       http.Client
	limiter      *rate.Limiter
}

// Pushover message struct
type pushoverRequest struct {
	// required fields
	// Token is a Pushover application API token, required.
	Token string `json:"token"`
	// UserKey is recipient's Pushover User Key, required.
	UserKey string `json:"user"`
	// Message is your text message, required.
	Message string `json:"message"`

	// common optional fields
	// Device specifies a set of user's devices to send the message; all would be sent if empty
	Device string `json:"device,omitempty"`
	// Title is the message's title, otherwise application's name is used.
	Title string `json:"title,omitempty"`
	// Sound is the name of one of the sounds supported by device clients.
	Sound string `json:"sound,omitempty"`
}

type pushoverResponse struct {
	Status  int      `json:"status"`
	Request string   `json:"request"`
	Errors  []string `json:"errors,omitempty"`
	Receipt string   `json:"receipt,omitempty"`
}

func NewPushoverNotifier(logger log.Logger, receivers []config.Receiver, notifierCfg *config.Config) notifier.Notifier {

	var path []string
	opts := notifierCfg.ReceiverOpts
	if opts != nil && opts.Global != nil {
		path = opts.Global.TemplateFiles
	}
	tmpl, err := notifier.NewTemplate(path)
	if err != nil {
		_ = level.Error(logger).Log("msg", "PushoverNotifier: get template error", "error", err.Error())
		return nil
	}

	n := &Notifier{
		notifierCfg:  notifierCfg,
		timeout:      DefaultSendTimeout,
		logger:       logger,
		template:     tmpl,
		templateName: DefaultTemplate,
		client:       http.Client{Timeout: DefaultSendTimeout},
		limiter:      rate.NewLimiter(1, 4), // a bucket with at most 4 tokens, and 1 token is generated per sec
	}

	if opts != nil && opts.Global != nil && !utils.StringIsNil(opts.Global.Template) {
		n.templateName = opts.Global.Template
	}

	if opts != nil && opts.Pushover != nil {

		if opts.Pushover.NotificationTimeout != nil {
			n.timeout = time.Second * time.Duration(*opts.Pushover.NotificationTimeout)
			n.client.Timeout = n.timeout
		}

		if !utils.StringIsNil(opts.Pushover.Template) {
			n.templateName = opts.Pushover.Template
		}
	}

	for _, r := range receivers {
		receiver, ok := r.(*config.Pushover)
		if !ok || receiver == nil {
			continue
		}

		if receiver.PushoverConfig == nil {
			_ = level.Warn(logger).Log("msg", "PushoverNotifier: ignore receiver because of empty config")
			continue
		}

		if utils.StringIsNil(receiver.Template) {
			receiver.Template = n.templateName
		}

		n.pushover = append(n.pushover, receiver)
	}

	return n
}

func (n *Notifier) Notify(ctx context.Context, data template.Data) []error {

	send := func(profile *v2beta2.PushoverUserProfile, c *config.Pushover) error {

		start, userKey := time.Now(), *profile.UserKey
		defer func() {
			_ = level.Debug(n.logger).Log("msg", "PushoverNotifier: send message", "userKey", userKey, "used", time.Since(start).String())
		}()

		// retrieve app's token
		token, err := n.notifierCfg.GetCredential(c.PushoverConfig.Token)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "PushoverNotifier: get token secret", "userKey", userKey, "error", err.Error())
			return err
		}

		// filter data by selector
		filteredData := utils.FilterAlerts(data, c.Selector, n.logger)
		if len(filteredData.Alerts) == 0 {
			return nil
		}

		// split new data along with its Alerts to ensure each message is small enough to fit the Pushover's message length limit
		messages, _, err := n.template.Split(filteredData, MessageMaxLength, c.Template, "", n.logger)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "PushoverNotifier: split alerts error", "userKey", userKey, "error", err.Error())
			return err
		}

		// send messages in parallel
		g := new(errgroup.Group)
		for _, message := range messages {
			msg := message
			g.Go(func() (err error) {
				// consume a token from limiter bucket
				if err := n.limiter.Wait(context.Background()); err != nil {
					_ = level.Error(n.logger).Log("msg", "PushoverNotifier: limiter error", "userKey", userKey, "error", err.Error())
					return err
				}

				// construct pushover message struct as request parameters
				pReq := &pushoverRequest{
					Token:   token,
					UserKey: userKey,
					Message: msg,
				}
				if len(profile.Devices) > 0 {
					pReq.Device = strings.Join(profile.Devices, ",")
				}
				if profile.Title != nil {
					pReq.Title = *profile.Title
				}
				if profile.Sound != nil {
					pReq.Sound = *profile.Sound
				}

				// JSON encoding
				var buf bytes.Buffer
				if err := utils.JsonEncode(&buf, pReq); err != nil {
					_ = level.Error(n.logger).Log("msg", "PushoverNotifier: encode message error", "userKey", userKey, "error", err.Error())
					return err
				}

				// build a JSON request with context
				request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, URL, &buf)
				if err != nil {
					_ = level.Error(n.logger).Log("msg", "PushoverNotifier: encode http request error", "userKey", userKey, "error", err.Error())
					return err
				}
				request.Header.Set("Content-Type", "application/json")

				// send the request
				response, err := n.client.Do(request)
				if err != nil {
					_ = level.Error(n.logger).Log("msg", "PushoverNotifier: do http error", "userKey", userKey, "error", err.Error())
					return err
				}

				defer func() {
					_, _ = io.Copy(ioutil.Discard, response.Body)
					_ = response.Body.Close()
				}()

				// check status code, but not return error if it is not 2xx, since we will do this later
				if response.StatusCode != http.StatusOK {
					_ = level.Error(n.logger).Log("msg", "PushoverNotifier: got non-2xx response", "userKey", userKey, "StatusCode", response.StatusCode)
					body, _ := ioutil.ReadAll(response.Body)
					return fmt.Errorf("PushoverNotifier: got non-2xx response, StatusCode: %d, response: %s", response.StatusCode, string(body))
				}

				// report the remaining number of messages for this period
				_ = level.Info(n.logger).Log("msg", "PushoverNotifier: remaining available message(s) for this period", "userKey", userKey, "remaining", response.Header.Get("X-Limit-App-Remaining"))

				// decode the response
				body, err := ioutil.ReadAll(response.Body)
				if err != nil {
					_ = level.Error(n.logger).Log("msg", "PushoverNotifier: read response error", "userKey", userKey, "error", err.Error())
					return err
				}

				var pResp pushoverResponse
				if err := utils.JsonUnmarshal(body, &pResp); err != nil {
					_ = level.Error(n.logger).Log("msg", "PushoverNotifier: decode response body error", "userKey", userKey, "error", err.Error())
					return err
				}

				// handle errors if any
				if pResp.Status != 1 {
					errStr := strings.Join(pResp.Errors, "; ")
					_ = level.Error(n.logger).Log("msg", "PushoverNotifier: pushover error", "userKey", userKey, "error", errStr)
					return fmt.Errorf(errStr)
				}

				_ = level.Debug(n.logger).Log("msg", "PushoverNotifier: sent message", "userKey", userKey)
				return nil
			})
		}
		// Wait for all HTTP requests to complete.
		if err := g.Wait(); err != nil {
			_ = level.Error(n.logger).Log("msg", "PushoverNotifier: an occurred while sending messages", "userKey", userKey, "error", err.Error())
			return err
		}
		return nil
	}

	group := async.NewGroup(ctx)
	for _, pushover := range n.pushover {
		po := pushover
		for _, profiles := range po.Profiles {
			pf := profiles
			group.Add(func(stopCh chan interface{}) {
				stopCh <- send(pf, po)
			})
		}
	}

	return group.Wait()
}
