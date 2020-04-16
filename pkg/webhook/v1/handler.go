package v1

import (
	"context"
	"encoding/json"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/pkg/notify/config"
	"github.com/kubesphere/notification-manager/pkg/webhook/v1/notify"
	"github.com/prometheus/alertmanager/template"
	"io"
	"net/http"
	"time"
)

type HttpHandler struct {
	logger         log.Logger
	semCh          chan struct{}
	webhookTimeout time.Duration
	wkrTimeout     time.Duration
	notifierCfg    *config.Config
}

type response struct {
	Status  int
	Message string
}

func New(logger log.Logger, semCh chan struct{}, webhookTimeout time.Duration, wkrTimeout time.Duration, cfg *config.Config) *HttpHandler {
	h := &HttpHandler{
		logger:         logger,
		semCh:          semCh,
		webhookTimeout: webhookTimeout,
		wkrTimeout:     wkrTimeout,
		notifierCfg:    cfg,
	}
	return h
}

func (h *HttpHandler) CreateNotificationfromAlerts(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	// Parse alerts sent through Alertmanager webhook, more detail please refer to
	// https://github.com/prometheus/alertmanager/blob/master/template/template.go#L231
	data := template.Data{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.handle(w, &response{http.StatusBadRequest, err.Error()})
		return
	}

	//bytes, _ := json.Marshal(data)
	//msg := string(bytes[:])
	//level.Debug(h.logger).Log("msg", "Received alerts", "alert", msg)

	ctx, cancel := context.WithTimeout(context.Background(), h.webhookTimeout)
	defer cancel()
	select {
	case h.semCh <- struct{}{}:
		_ = level.Debug(h.logger).Log("msg", "Acquired worker queue lock...")
	case <-ctx.Done():
		_ = level.Warn(h.logger).Log("msg", "Running out of queue capacity in "+h.webhookTimeout.String(), "error", ctx.Err())
		h.handle(w, &response{http.StatusInternalServerError, "Running out of queue capacity with error: " + ctx.Err().Error()})
	}

	worker := func(ctx context.Context, wkload template.Data, stopCh chan struct{}) error {
		var err error
		wkrCh := make(chan struct{})
		defer close(stopCh)

		go func() {
			defer close(wkrCh)

			for _, alert := range wkload.Alerts {
				var ns *string = nil
				value, ok := alert.Labels["namespace"]
				if ok {
					ns = &value
				}
				receivers := h.notifierCfg.RcvsFromNs(ns)
				integration := notify.NewIntegration(h.logger, receivers, alert)
				errs := integration.Notify()
				if errs != nil && len(errs) > 0 {
					_ = level.Error(h.logger).Log("msg", "Worker: notification sent error")
				}
			}

			_ = level.Info(h.logger).Log("msg", "Worker: notification sent")
		}()

		select {
		case <-ctx.Done():
			err = ctx.Err()
			_ = level.Warn(h.logger).Log("msg", "Worker: sending notification timeout in "+h.wkrTimeout.String(), "error", err.Error())
		case <-wkrCh:
			_ = level.Debug(h.logger).Log("msg", "Worker: exiting")
		}

		_ = level.Debug(h.logger).Log("msg", "Worker: exit")
		return err
	}

	// launch one worker goroutine for each received alert to create notification for it
	go func(semCh chan struct{}, timeout time.Duration) {
		_ = level.Debug(h.logger).Log("msg", "Begins to send notification...")

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		stopCh := make(chan struct{})
		wkloadCh := make(chan template.Data, 1)
		wkloadCh <- data

		t := time.Now()
		for i := 0; i < 2; i += 1 {
			select {
			case wkload := <-wkloadCh:
				_ = worker(ctx, wkload, stopCh)
			case <-stopCh:
				<-h.semCh
				elapsed := time.Since(t).String()
				_ = level.Debug(h.logger).Log("msg", "Worker exit after "+elapsed)
				return
			}
		}
	}(h.semCh, h.wkrTimeout)

	h.handle(w, &response{http.StatusOK, "Notification request accepted"})
}

func (h *HttpHandler) ServeMetrics(w http.ResponseWriter, r *http.Request) {
	h.handle(w, &response{http.StatusOK, "metrics"})
}

func (h *HttpHandler) ServeReload(w http.ResponseWriter, r *http.Request) {
	h.handle(w, &response{http.StatusOK, "reload"})
}

func (h *HttpHandler) ServeHealthCheck(w http.ResponseWriter, r *http.Request) {
	h.handle(w, &response{http.StatusOK, "health check"})
}

func (h *HttpHandler) ServeReadinessCheck(w http.ResponseWriter, r *http.Request) {
	h.handle(w, &response{http.StatusOK, "ready"})
}

func (h *HttpHandler) ServeStatus(w http.ResponseWriter, r *http.Request) {
	h.handle(w, &response{http.StatusOK, "status"})
}

func (h *HttpHandler) handle(w http.ResponseWriter, resp *response) {
	bytes, _ := json.Marshal(resp)
	msg := string(bytes[:])
	w.WriteHeader(resp.Status)
	_, _ = io.WriteString(w, msg)

	if resp.Status != http.StatusOK {
		_ = level.Error(h.logger).Log("msg", resp.Message)
	} else {
		_ = level.Debug(h.logger).Log("msg", resp.Message)
	}
}
