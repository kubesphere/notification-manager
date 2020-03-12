package main

import (
	"context"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/webhook"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/promlog/flag"
	"gopkg.in/alecthomas/kingpin.v2"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	var (
		listenAddress = kingpin.Flag(
			"webhook.address",
			"The address to listen on for incoming alerts or notifications.",
		).Default(":9193").String()
		webhookTimeout = kingpin.Flag(
			"webhook.timeout",
			"Webhook timeout for each incoming request",
		).Default("5s").String()
		wkrTimeout = kingpin.Flag(
			"worker.timeout",
			"Processing timeout for each incoming alerts or notifications",
		).Default("30s").String()
		wkrQueue = kingpin.Flag(
			"worker.queue",
			"Notification worker queue capacity",
		).Default("1000").Int()
	)

	logConfig := &promlog.Config{}
	flag.AddFlags(kingpin.CommandLine, logConfig)
	kingpin.Parse()
	logger := promlog.New(logConfig)

	level.Info(logger).Log("msg", "Starting notification manager...", "addr", *listenAddress, "timeout", *webhookTimeout)

	ctxHttp, cancelHttp := context.WithCancel(context.Background())
	defer cancelHttp()

	webhook := webhook.New(
		log.With(logger, "component", "webhook"),
		&webhook.Options{
			ListenAddress:  *listenAddress,
			WebhookTimeout: *webhookTimeout,
			WorkerTimeout:  *wkrTimeout,
			WorkerQueue:    *wkrQueue,
		})

	srvCh := make(chan error, 1)
	termCh := make(chan os.Signal, 1)
	signal.Notify(termCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		defer close(srvCh)
		if err := webhook.Run(ctxHttp); err != nil {
			level.Error(logger).Log("msg", "Run HTTP server", "err", err)
			srvCh <- err
		}
	}()

	for {
		select {
		case <-termCh:
			level.Info(logger).Log("msg", "Received SIGTERM, exiting gracefully...")
			cancelHttp()
		case err := <-srvCh:
			if err != nil {
				level.Error(logger).Log("msg", "Abnormal exit", "error", err.Error())
				os.Exit(1)
			}
			os.Exit(0)
		}
	}
}
