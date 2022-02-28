package webhook

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/pkg/controller"
	"github.com/kubesphere/notification-manager/pkg/store"
	v1 "github.com/kubesphere/notification-manager/pkg/webhook/v1"
)

type Options struct {
	ListenAddress  string
	WebhookTimeout time.Duration
	WorkerTimeout  time.Duration
}

type Webhook struct {
	router chi.Router
	*Options
	logger  log.Logger
	handler *v1.HttpHandler
}

func New(logger log.Logger, notifierCtl *controller.Controller, alerts *store.AlertStore, o *Options) *Webhook {

	h := &Webhook{
		Options: o,
		logger:  logger,
	}

	h.handler = v1.New(logger, h.WorkerTimeout, notifierCtl, alerts)
	h.router = chi.NewRouter()

	h.router.Use(middleware.RequestID)
	// h.router.Use(middleware.Logger)
	h.router.Use(middleware.Recoverer)
	h.router.Use(middleware.Timeout(2 * h.WebhookTimeout))
	h.router.Get("/receivers", h.handler.ListReceivers)
	h.router.Get("/configs", h.handler.ListConfigs)
	h.router.Get("/receiverWithConfig", h.handler.ListReceiverWithConfig)
	h.router.Post("/api/v2/alerts", h.handler.Alert)
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
		Addr:    h.ListenAddress,
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
