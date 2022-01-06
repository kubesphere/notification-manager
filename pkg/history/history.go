package history

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/kubesphere/notification-manager/pkg/notify"
	"github.com/kubesphere/notification-manager/pkg/notify/config"
	"github.com/prometheus/alertmanager/template"
)

const (
	defaultTimeout      = 30 * time.Second
	notificationHistory = "notification/history"
	historyRetry        = "history/retry"
	retryMax            = 10
	requeueDelayTime    = 5 * time.Second
)

type backend struct {
	ch          chan interface{}
	notifierCfg *config.Config
	logger      log.Logger
	timeout     time.Duration
}

func StartBackend(ch chan interface{}, notifierCfg *config.Config, logger log.Logger, timeoutStr *string) {
	logger = log.With(logger, "type", "history")
	b := backend{
		ch:          ch,
		notifierCfg: notifierCfg,
		logger:      logger,
		timeout:     defaultTimeout,
	}

	if timeoutStr != nil {
		if timeout, err := time.ParseDuration(*timeoutStr); err != nil {
			b.timeout = timeout
		}
	}

	go b.run()
}

func (b *backend) run() {

	for {
		item := <-b.ch
		data, ok := item.(template.Data)
		if !ok {
			continue
		}

		if len(data.Alerts) == 0 {
			continue
		}

		if data.CommonAnnotations != nil {
			// The purpose of this annotation is to avoid the notification history being sent in a loop.
			if _, ok := data.CommonAnnotations[notificationHistory]; ok {
				continue
			}
		}

		go func() {
			receivers := b.notifierCfg.GetHistoryReceivers()
			if receivers == nil || len(receivers) == 0 {
				return
			}

			if data.CommonAnnotations == nil {
				data.CommonAnnotations = make(map[string]string)
			}
			data.CommonAnnotations[notificationHistory] = "true"

			retry := getRetryTimes(data)
			logger := b.logger
			if retry != 0 {
				logger = log.With(b.logger, "retry", retry)
			}

			n := notify.NewNotification(logger, receivers, b.notifierCfg, data)
			ctx, cancel := context.WithTimeout(context.Background(), b.timeout)
			defer cancel()
			err := n.Notify(ctx)
			if err != nil && len(err) > 0 {
				b.requeue(data)
			}
		}()
	}
}

func (b *backend) requeue(data template.Data) {
	time.Sleep(requeueDelayTime)

	delete(data.CommonAnnotations, notificationHistory)
	retry := getRetryTimes(data)
	if retry >= retryMax {
		return
	}
	data.CommonAnnotations[historyRetry] = fmt.Sprintf("%d", retry+1)

	b.ch <- data
}

func getRetryTimes(data template.Data) int {
	str, ok := data.CommonAnnotations[historyRetry]
	if !ok {
		return 0
	} else {
		retry, _ := strconv.Atoi(str)
		return retry
	}
}
