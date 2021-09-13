package webhook

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/pkg/config"
	whv1 "github.com/kubesphere/notification-manager/pkg/webhook/v1"
)

type Options struct {
	ListenAddress  string
	WebhookTimeout string
	WorkerTimeout  string
	WorkerQueue    int
}

type Webhook struct {
	router  chi.Router
	options *Options
	logger  log.Logger
	handler *whv1.HttpHandler
}

func New(logger log.Logger, notifierCfg *config.Config, o *Options) *Webhook {
	webhookTimeout, _ := time.ParseDuration(o.WebhookTimeout)
	wkrTimeout, _ := time.ParseDuration(o.WorkerTimeout)

	h := &Webhook{
		options: o,
		logger:  logger,
	}

	semCh := make(chan struct{}, h.options.WorkerQueue)
	h.handler = whv1.New(logger, semCh, webhookTimeout, wkrTimeout, notifierCfg)
	h.router = chi.NewRouter()

	h.router.Use(middleware.RequestID)
	// h.router.Use(middleware.Logger)
	h.router.Use(middleware.Recoverer)
	h.router.Use(middleware.Timeout(2 * webhookTimeout))
	h.router.Get("/receivers", h.handler.ListReceivers)
	h.router.Get("/configs", h.handler.ListConfigs)
	h.router.Get("/receiverWithConfig", h.handler.ListReceiverWithConfig)
	h.router.Post("/api/v2/alerts", h.handler.CreateNotificationFromAlerts)
	h.router.Post("/api/v2/verify", h.handler.Verify)
	h.router.Post("/api/v2/notifications", h.handler.Notification)
	h.router.Get("/metrics", h.handler.ServeMetrics)
	h.router.Get("/-/reload", h.handler.ServeReload)
	h.router.Get("/-/ready", h.handler.ServeHealthCheck)
	h.router.Get("/-/live", h.handler.ServeReadinessCheck)
	h.router.Get("/status", h.handler.ServeStatus)

	return h
}

func (h *Webhook) Run(ctx context.Context) error {
	var err error
	httpSrv := &http.Server{
		Addr:    h.options.ListenAddress,
		Handler: h.router,
	}

	srvClosed := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			// We received an interrupt signal, shut down.
			if err := httpSrv.Shutdown(ctx); err != nil {
				// Error from closing listeners, or context timeout:
				_ = level.Error(h.logger).Log("msg", "Shutdown HTTP server", "err", err)
			}
			_ = level.Info(h.logger).Log("msg", "Shutdown HTTP server")
			close(srvClosed)
		}
	}()

	if err = httpSrv.ListenAndServe(); err != http.ErrServerClosed {
		// Error starting or closing listener:
		_ = level.Error(h.logger).Log("msg", "HTTP server ListenAndServe", "err", err)
	}

	_ = level.Error(h.logger).Log("msg", "HTTP server exit", "err", err)
	<-srvClosed

	return err
}
