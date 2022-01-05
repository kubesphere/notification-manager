package history

import (
	"context"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/kubesphere/notification-manager/pkg/notify"
	"github.com/kubesphere/notification-manager/pkg/notify/config"
	"github.com/prometheus/alertmanager/template"
)

const (
	defaultTimeout  = 30 * time.Second
	historyRecorded = "history/recorded"
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
			return
		}

		if data.CommonAnnotations != nil {
			// History had sent, return
			if data.CommonAnnotations[historyRecorded] == "true" {
				return
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
			data.CommonAnnotations[historyRecorded] = "true"

			n := notify.NewNotification(b.logger, receivers, b.notifierCfg, data)
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
	time.Sleep(time.Second)
	b.ch <- data
}
