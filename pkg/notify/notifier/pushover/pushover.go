package pushover

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/pkg/async"
	"github.com/kubesphere/notification-manager/pkg/notify/config"
	"github.com/kubesphere/notification-manager/pkg/notify/notifier"
	"github.com/kubesphere/notification-manager/pkg/utils"
	"github.com/prometheus/alertmanager/template"
	"golang.org/x/sync/errgroup"
)

const (
	DefaultSendTimeout = time.Second * 3
	URL                = "https://api.pushover.net/1/messages.json"
	DefaultTemplate    = `{{ template "nm.default.text" . }}`
	PoMsgLimitAlert    = 99
)

type Notifier struct {
	notifierCfg  *config.Config
	pushover     []*config.Pushover
	timeout      time.Duration
	logger       log.Logger
	template     *notifier.Template
	templateName string
	client       http.Client
}

type pushoverRequest struct {
	pushoverMessage
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

	send := func(userKey string, c *config.Pushover) error {

		start := time.Now()
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

		// send messages in parallel with errgroup
		g := new(errgroup.Group)
		for _, message := range messages {
			msg := message
			g.Go(func() error {
				// construct pushover message struct as request parameters, and validate it
				pm := newPushoverMessage(token, userKey, msg)
				err, warnings := pm.validate()
				if err != nil {
					_ = level.Error(n.logger).Log("msg", "PushoverNotifier: invalid pushover message", "userKey", userKey, "error", err.Error())
					return err
				}
				if len(warnings) > 0 {
					_ = level.Warn(n.logger).Log("msg", "PushoverNotifier: warnings about the message", "userKey", userKey, "warnings", strings.Join(warnings, "; "))
				}
				pReq := &pushoverRequest{pm}

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
				}

				// check if the remaining number of messages that can be sent is low
				PoRemainingMsg, err := strconv.Atoi(response.Header.Get("X-Limit-App-Remaining"))
				if err != nil {
					_ = level.Error(n.logger).Log("msg", "PushoverNotifier: get response headers error", "userKey", userKey, "error", err.Error())
					return err
				}
				if PoRemainingMsg < PoMsgLimitAlert {
					_ = level.Warn(n.logger).Log("msg", "PushoverNotifier: you are approaching Pushover app's message limits", "userKey", userKey, "warnings", fmt.Sprintf("remaining %d message for this period", PoRemainingMsg))
				}

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
		if err := g.Wait(); err != nil {
			_ = level.Error(n.logger).Log("msg", "PushoverNotifier: an error occurred in a goroutine when sending a message", "userKey", userKey, "error", err.Error())
			return err
		}
		return nil
	}

	group := async.NewGroup(ctx)
	for _, pushover := range n.pushover {
		p := pushover
		for _, userKeys := range p.UserKeys {
			uk := userKeys
			group.Add(func(stopCh chan interface{}) {
				stopCh <- send(uk, p)
			})
		}
	}

	return group.Wait()
}
