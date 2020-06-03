package main

import (
	"context"
	"fmt"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/pkg/notify/config"
	wh "github.com/kubesphere/notification-manager/pkg/webhook"
	"gopkg.in/alecthomas/kingpin.v2"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

var (
	cfg *config.Config

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
	).Default("5s").String()

	wkrTimeout = kingpin.Flag(
		"worker.timeout",
		"Processing timeout for each incoming alerts or notifications",
	).Default("30s").String()

	wkrQueue = kingpin.Flag(
		"worker.queue",
		"Notification worker queue capacity",
	).Default("1000").Int()

	monitorNamespaces = kingpin.Flag(
		"monitorNamespaces",
		"Monitor namespaces",
	).Default("kubesphere-monitoring-system").String()

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

	logger = log.With(logger, "ts", log.DefaultTimestampUTC)
	logger = log.With(logger, "caller", log.DefaultCaller)
	_ = level.Info(logger).Log("msg", "Starting notification manager...", "addr", *listenAddress, "timeout", *webhookTimeout)

	ctxHttp, cancelHttp := context.WithCancel(context.Background())
	defer cancelHttp()

	// Setup notification manager config
	var err error
	if cfg, err = config.New(ctxHttp, logger, *monitorNamespaces); err != nil {
		_ = level.Error(logger).Log("msg", "Failed to create notification manager config")
	}
	// Sync notification manager config
	if err := cfg.Run(); err != nil {
		_ = level.Error(logger).Log("msg", "Failed to create sync notification manager config")
	}

	// Setup webhook to receive alert/notification msg
	webhook := wh.New(
		logger,
		cfg,
		&wh.Options{
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
			_ = level.Error(logger).Log("msg", "Run HTTP server", "err", err)
			srvCh <- err
		}
	}()

	for {
		select {
		case <-termCh:
			_ = level.Info(logger).Log("msg", "Received SIGTERM, exiting gracefully...")
			cancelHttp()
		case err := <-srvCh:
			if err != nil {
				_ = level.Error(logger).Log("msg", "Abnormal exit", "error", err.Error())
				return 1
			}
			return 0
		}
	}
}

func main() {
	os.Exit(Main())
}
