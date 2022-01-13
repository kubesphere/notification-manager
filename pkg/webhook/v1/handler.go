package v1

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/pkg/apis/v2beta2"
	"github.com/kubesphere/notification-manager/pkg/async"
	"github.com/kubesphere/notification-manager/pkg/notify"
	"github.com/kubesphere/notification-manager/pkg/notify/config"
	"github.com/kubesphere/notification-manager/pkg/utils"
	"github.com/prometheus/alertmanager/template"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	historyRetryMax   = 10
	historyRetryDelay = time.Second * 5
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

func (h *HttpHandler) CreateNotificationFromAlerts(w http.ResponseWriter, r *http.Request) {
	defer func() {
		_ = r.Body.Close()
	}()

	// Parse alerts sent through Alertmanager webhook, more detail please refer to
	// https://github.com/prometheus/alertmanager/blob/master/template/template.go#L231
	data := template.Data{}
	if err := utils.JsonDecode(r.Body, &data); err != nil {
		h.handle(w, &response{http.StatusBadRequest, err.Error()})
		return
	}

	//	if alerts, err := json.MarshalIndent(data, "", "  "); err != nil {
	//		_ = level.Error(h.logger).Log("msg", "Failed to encode alerts:", "err", err)
	//	} else {
	//		os.Stdout.Write(alerts)
	//		os.Stdout.Write([]byte("\n"))
	//	}

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

			cluster := "default"
			if h.notifierCfg != nil && h.notifierCfg.ReceiverOpts != nil && h.notifierCfg.ReceiverOpts.Global != nil {
				if h.notifierCfg.ReceiverOpts.Global.Cluster != "" {
					cluster = h.notifierCfg.ReceiverOpts.Global.Cluster
				}
			}

			for _, alert := range data.Alerts {
				alert.Labels["cluster"] = cluster
			}

			dm := make(map[string]template.Data)
			ns, ok := wkload.CommonLabels["namespace"]
			if ok {
				dm[ns] = wkload
			} else {
				for _, alert := range wkload.Alerts {
					ns, ok = alert.Labels["namespace"]
					if !ok {
						ns = ""
					}

					d, ok := dm[ns]
					if !ok {
						d = template.Data{
							Alerts:       template.Alerts{},
							CommonLabels: map[string]string{},
							GroupLabels:  map[string]string{},
							Receiver:     wkload.Receiver,
							ExternalURL:  wkload.ExternalURL,
						}
						for k, v := range wkload.CommonLabels {
							d.CommonLabels[k] = v
						}
						if len(ns) > 0 {
							d.CommonLabels["namespace"] = ns
						}
						for k, v := range wkload.GroupLabels {
							d.GroupLabels[k] = v
						}
					}

					d.Alerts = append(d.Alerts, alert)
					dm[ns] = d
				}
			}

			group := async.NewGroup(ctx)
			for k, d := range dm {
				var ns *string = nil
				if len(k) > 0 {
					ns = &k
				}
				receivers := h.notifierCfg.RcvsFromNs(ns)
				n := notify.NewNotification(h.logger, receivers, h.notifierCfg, d)
				group.Add(func(stopCh chan interface{}) {
					stopCh <- n.Notify(ctx)
				})

				// Export notification history.
				var selectors []*metav1.LabelSelector
				for _, receiver := range receivers {
					selectors = append(selectors, receiver.GetAlertSelector())
				}
				h.SendNotificationHistory(d, selectors)
			}

			errs := group.Wait()
			if errs != nil && len(errs) > 0 {
				_ = level.Error(h.logger).Log("msg", "Worker: notification sent error")
			}

			_ = level.Debug(h.logger).Log("msg", "Worker: notification sent")
		}()

		select {
		case <-ctx.Done():
			err = ctx.Err()
			if err != nil {
				_ = level.Warn(h.logger).Log("msg", "Worker: sending notification timeout in "+h.wkrTimeout.String(), "error", err.Error())
			}
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

func (h *HttpHandler) ServeMetrics(w http.ResponseWriter, _ *http.Request) {
	h.handle(w, &response{http.StatusOK, "metrics"})
}

func (h *HttpHandler) ServeReload(w http.ResponseWriter, _ *http.Request) {
	h.handle(w, &response{http.StatusOK, "reload"})
}

func (h *HttpHandler) ServeHealthCheck(w http.ResponseWriter, _ *http.Request) {
	h.handle(w, &response{http.StatusOK, "health check"})
}

func (h *HttpHandler) ServeReadinessCheck(w http.ResponseWriter, _ *http.Request) {
	h.handle(w, &response{http.StatusOK, "ready"})
}

func (h *HttpHandler) ServeStatus(w http.ResponseWriter, _ *http.Request) {
	h.handle(w, &response{http.StatusOK, "status"})
}

func (h *HttpHandler) handle(w http.ResponseWriter, resp *response) {
	bytes, _ := utils.JsonMarshal(resp)
	msg := string(bytes[:])
	w.WriteHeader(resp.Status)
	_, _ = io.WriteString(w, msg)

	if resp.Status != http.StatusOK {
		_ = level.Error(h.logger).Log("msg", resp.Message)
	} else {
		_ = level.Debug(h.logger).Log("msg", resp.Message)
	}
}

func (h *HttpHandler) GetReceivers(w http.ResponseWriter, r *http.Request) {

	_ = r.ParseForm()
	bs, _ := utils.JsonMarshalIndent(h.notifierCfg.OutputReceiver(r.FormValue("tenant"), r.FormValue("type")), "", "  ")
	_, _ = w.Write(bs)
	return
}

func (h *HttpHandler) Verify(w http.ResponseWriter, r *http.Request) {

	m := make(map[string]interface{})
	if err := utils.JsonDecode(r.Body, &m); err != nil {
		h.handle(w, &response{http.StatusBadRequest, err.Error()})
		return
	}

	receivers, err := h.getReceiversFromRequest(m)
	if err != nil {
		h.handle(w, &response{http.StatusBadRequest, err.Error()})
		return
	}

	labels := template.KV{
		"alertname": "Test",
	}
	annotations := template.KV{
		"message": "Congratulations, your configuration is correct!",
	}

	d := template.Data{
		Receiver: "Default",
		Status:   "firing",
		Alerts: template.Alerts{
			{
				Status:      "firing",
				Labels:      labels,
				Annotations: annotations,
				StartsAt:    time.Now(),
				EndsAt:      time.Now(),
			},
		},
		GroupLabels:       labels,
		CommonLabels:      labels,
		CommonAnnotations: annotations,
		ExternalURL:       "kubesphere.io",
	}

	if msg := h.send(receivers, d, h.logger); msg != "" {
		h.handle(w, &response{http.StatusBadRequest, msg})
		return
	}

	h.handle(w, &response{http.StatusOK, "Verify successfully"})
}

func (h *HttpHandler) Notification(w http.ResponseWriter, r *http.Request) {

	m := make(map[string]interface{})
	if err := utils.JsonDecode(r.Body, &m); err != nil {
		h.handle(w, &response{http.StatusBadRequest, err.Error()})
		return
	}

	receivers, err := h.getReceiversFromRequest(m)
	if err != nil {
		h.handle(w, &response{http.StatusBadRequest, err.Error()})
		return
	}

	d := template.Data{}
	alert, ok := m["alert"]
	if !ok {
		h.handle(w, &response{http.StatusBadRequest, "alert is nil"})
		return
	}

	bs, err := utils.JsonMarshal(alert)
	if err != nil {
		h.handle(w, &response{http.StatusBadRequest, err.Error()})
		return
	}

	if err := utils.JsonUnmarshal(bs, &d); err != nil {
		h.handle(w, &response{http.StatusBadRequest, err.Error()})
		return
	}

	if msg := h.send(receivers, d, h.logger); msg != "" {
		h.handle(w, &response{http.StatusBadRequest, msg})
		return
	}

	h.handle(w, &response{http.StatusOK, "Send alerts successfully"})
}

func (h *HttpHandler) getReceiversFromRequest(m map[string]interface{}) ([]config.Receiver, error) {

	nr := v2beta2.Receiver{}
	if _, ok := m["receiver"]; !ok {
		return nil, fmt.Errorf("receiver is nil")
	}

	if err := utils.MapToStruct(m["receiver"].(map[string]interface{}), &nr); err != nil {
		return nil, err
	}

	var nc *v2beta2.Config = nil
	if _, ok := m["config"]; ok {
		tmp := v2beta2.Config{}
		if err := utils.MapToStruct(m["config"].(map[string]interface{}), &tmp); err != nil {
			return nil, err
		}
		nc = &tmp
	}

	receivers, err := h.notifierCfg.GenerateReceivers(&nr, nc)
	if err != nil {
		return nil, err
	}

	return receivers, nil
}

func (h *HttpHandler) send(receivers []config.Receiver, d template.Data, logger log.Logger) string {
	n := notify.NewNotification(logger, receivers, h.notifierCfg, d)
	ctx, cancel := context.WithTimeout(context.Background(), h.wkrTimeout)
	defer cancel()
	errs := n.Notify(ctx)
	if errs != nil && len(errs) > 0 {
		msg := ""
		for _, err := range errs {
			msg += err.Error() + ", "
		}
		msg = strings.TrimSuffix(msg, ", ")
		return msg
	}

	return ""
}

func (h *HttpHandler) SendNotificationHistory(data template.Data, selectors []*metav1.LabelSelector) {

	go func() {

		receivers := h.notifierCfg.GetHistoryReceivers()
		if receivers == nil || len(receivers) == 0 {
			return
		}

		newData := template.Data{
			Receiver:          data.Receiver,
			Status:            data.Status,
			GroupLabels:       data.GroupLabels,
			CommonLabels:      data.CommonLabels,
			CommonAnnotations: data.CommonAnnotations,
			ExternalURL:       data.ExternalURL,
		}

		for _, alert := range data.Alerts {
			flag := false
			for _, selector := range selectors {
				if utils.SelectorMatchesAlert(alert, selector) {
					flag = true
					break
				}
			}

			if flag {
				newData.Alerts = append(newData.Alerts, alert)
			}
		}

		if len(newData.Alerts) == 0 {
			return
		}

		for retry := 0; retry <= historyRetryMax; retry++ {
			logger := log.With(h.logger, "type", "history")
			if retry != 0 {
				logger = log.With(logger, "retry", retry)
			}

			if errMsg := h.send(receivers, newData, logger); errMsg == "" {
				return
			}

			time.Sleep(historyRetryDelay)
		}
	}()
}
