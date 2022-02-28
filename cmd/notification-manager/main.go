package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/pkg/controller"
	"github.com/kubesphere/notification-manager/pkg/dispatcher"
	"github.com/kubesphere/notification-manager/pkg/store"
	wh "github.com/kubesphere/notification-manager/pkg/webhook"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	ctl *controller.Controller

	logLevel = kingpin.Flag(
		"log.level",
		fmt.Sprintf("Log level to use. Possible values: %s", strings.Join(logLevels, ", ")),
	).Default("info").String()

	logfmt = kingpin.Flag(
		"log.format",
		fmt.Sprintf("Log format to use. Possible values: %s", strings.Join(logFormats, ", ")),
	).Default("logfmt").String()

	listenAddress = kingpin.Flag(
		"webhook.address",
		"The address to listen on for incoming alerts or notifications.",
	).Default(":19093").String()

	webhookTimeout = kingpin.Flag(
		"webhook.timeout",
		"Webhook timeout for each incoming request",
	).Default("5s").Duration()

	wkrTimeout = kingpin.Flag(
		"worker.timeout",
		"Processing timeout for each incoming alerts or notifications",
	).Default("30s").Duration()

	wkrQueue = kingpin.Flag(
		"worker.queue",
		"Notification worker queue capacity",
	).Default("1000").Int()

	storeType = kingpin.Flag(
		"store.type",
		"Type of store which used to cache the alerts",
	).Default("memory").String()

	logLevels = []string{
		logLevelDebug,
		logLevelInfo,
		logLevelWarn,
		logLevelError,
	}

	logFormats = []string{
		logFormatLogfmt,
		logFormatJson,
	}
)

const (
	logFormatLogfmt = "logfmt"
	logFormatJson   = "json"
	logLevelDebug   = "debug"
	logLevelInfo    = "info"
	logLevelWarn    = "warn"
	logLevelError   = "error"
)

func Main() int {
	kingpin.Parse()
	logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stdout))
	if *logfmt == logFormatJson {
		logger = log.NewJSONLogger(log.NewSyncWriter(os.Stdout))
	}

	switch *logLevel {
	case logLevelDebug:
		logger = level.NewFilter(logger, level.AllowDebug())
	case logLevelInfo:
		logger = level.NewFilter(logger, level.AllowInfo())
	case logLevelWarn:
		logger = level.NewFilter(logger, level.AllowWarn())
	case logLevelError:
		logger = level.NewFilter(logger, level.AllowError())
	default:
		_, _ = fmt.Fprintf(os.Stderr, "log level %v unknown, %v are possible values", *logLevel, logLevels)
		return 1
	}

	logger = log.With(logger, "ts", log.DefaultTimestamp)
	logger = log.With(logger, "caller", log.DefaultCaller)
	_ = level.Info(logger).Log("msg", "Starting notification manager...", "addr", *listenAddress, "timeout", *webhookTimeout)

	// Setup notification manager controller
	var err error
	ctlCtx, cancelCtl := context.WithCancel(context.Background())
	defer cancelCtl()
	if ctl, err = controller.New(ctlCtx, logger); err != nil {
		_ = level.Error(logger).Log("msg", "Failed to create notification manager controller")
		return -1
	}
	// Sync notification manager config
	if err := ctl.Run(); err != nil {
		_ = level.Error(logger).Log("msg", "Failed to create sync notification manager controller")
		return -1
	}

	alerts := store.NewAlertStore(*storeType)

	// Setup webhook to receive alert/notification msg
	webhook := wh.New(
		logger,
		ctl,
		alerts,
		&wh.Options{
			ListenAddress:  *listenAddress,
			WebhookTimeout: *webhookTimeout,
			WorkerTimeout:  *wkrTimeout,
		})

	ctxHttp, cancelHttp := context.WithCancel(context.Background())
	defer cancelHttp()

	srvCh := make(chan error, 1)
	go func() {
		srvCh <- webhook.Run(ctxHttp)
	}()

	dispCh := make(chan error, 1)
	disp := dispatcher.New(logger, ctl, alerts, *webhookTimeout, *wkrTimeout, *wkrQueue)
	go func() {
		dispCh <- disp.Run()
	}()

	termCh := make(chan os.Signal, 1)
	signal.Notify(termCh, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case <-termCh:
			_ = level.Info(logger).Log("msg", "Received SIGTERM, exiting gracefully...")
			cancelHttp()
		case err := <-srvCh:
			if err != nil {
				_ = level.Error(logger).Log("msg", "Abnormal exit", "error", err.Error())
			}
			_ = alerts.Close()
			_ = level.Info(logger).Log("msg", "Store closed")
		case <-dispCh:
			_ = level.Info(logger).Log("msg", "Dispatcher closed")
			return 0
		}
	}
}

func main() {
	os.Exit(Main())
}
