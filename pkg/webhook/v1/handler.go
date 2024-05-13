package v1

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/apis/v2beta2"
	"github.com/kubesphere/notification-manager/pkg/aggregation"
	"github.com/kubesphere/notification-manager/pkg/constants"
	"github.com/kubesphere/notification-manager/pkg/controller"
	"github.com/kubesphere/notification-manager/pkg/internal"
	"github.com/kubesphere/notification-manager/pkg/notify"
	"github.com/kubesphere/notification-manager/pkg/stage"
	"github.com/kubesphere/notification-manager/pkg/store"
	"github.com/kubesphere/notification-manager/pkg/template"
	"github.com/kubesphere/notification-manager/pkg/utils"
)

type HttpHandler struct {
	logger      log.Logger
	wkrTimeout  time.Duration
	notifierCtl *controller.Controller
	alerts      *store.AlertStore
}

type response struct {
	Status  int
	Message string
}

func New(logger log.Logger, wkrTimeout time.Duration, ctl *controller.Controller, alerts *store.AlertStore) *HttpHandler {
	h := &HttpHandler{
		logger:      logger,
		wkrTimeout:  wkrTimeout,
		notifierCtl: ctl,
		alerts:      alerts,
	}
	return h
}

func (h *HttpHandler) Alert(w http.ResponseWriter, r *http.Request) {
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

	//if alerts, err := utils.JsonMarshalIndent(data, "", "  "); err != nil {
	//	_ = level.Error(h.logger).Log("msg", "Failed to encode alerts:", "err", err)
	//} else {
	//	fmt.Println(string(alerts))
	//}

	cluster := h.notifierCtl.GetCluster()
	for _, alert := range data.Alerts {
		if v := alert.Labels["cluster"]; v == "" {
			alert.Labels["cluster"] = cluster
		}

		if alert.Labels["alerttype"] == "metric" {
			alert.Annotations["alerttime"] = time.Now().Local().String()
		}

		alert.ID = utils.Hash(alert)
		if err := h.alerts.Push(alert); err != nil {
			_ = level.Error(h.logger).Log("msg", "push alert error", "error", err.Error())
		}
	}

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

func (h *HttpHandler) ListReceivers(w http.ResponseWriter, r *http.Request) {

	_ = r.ParseForm()
	bs, _ := utils.JsonMarshalIndent(h.notifierCtl.ListReceiver(r.FormValue("tenant"), r.FormValue("type")), "", "  ")
	_, _ = w.Write(bs)
	return
}

func (h *HttpHandler) ListConfigs(w http.ResponseWriter, r *http.Request) {

	_ = r.ParseForm()
	bs, _ := utils.JsonMarshalIndent(h.notifierCtl.ListConfig(r.FormValue("tenant"), r.FormValue("type")), "", "  ")
	_, _ = w.Write(bs)
	return
}

func (h *HttpHandler) ListReceiverWithConfig(w http.ResponseWriter, r *http.Request) {

	_ = r.ParseForm()
	bs, _ := utils.JsonMarshalIndent(h.notifierCtl.ListReceiverWithConfig(r.FormValue("tenant"), r.FormValue("name"), r.FormValue("type")), "", "  ")
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

	alerts := []*template.Alert{
		{
			Labels: template.KV{
				constants.AlertName: constants.Verify,
				constants.AlertType: constants.Verify,
				constants.AlertTime: time.Now().Local().String(),
			},
			Annotations: template.KV{
				constants.AlertMessage: "Congratulations, your notification configuration is correct!",
			},
			StartsAt: time.Now(),
			EndsAt:   time.Now(),
		},
	}

	if msg := h.send(receivers, alerts, constants.Verify); msg != "" {
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

	if msg := h.send(receivers, d.Alerts, constants.Notification); msg != "" {
		h.handle(w, &response{http.StatusBadRequest, msg})
		return
	}

	h.handle(w, &response{http.StatusOK, "Send alerts successfully"})
}

func (h *HttpHandler) getReceiversFromRequest(m map[string]interface{}) ([]internal.Receiver, error) {

	nr := v2beta2.Receiver{}
	if _, ok := m["receiver"]; !ok {
		return nil, utils.Error("Receiver is nil")
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

	receivers, err := h.notifierCtl.GenerateReceivers(&nr, nc)
	if err != nil {
		return nil, err
	}

	return receivers, nil
}

func (h *HttpHandler) send(receivers []internal.Receiver, alerts template.Alerts, seq string) string {
	ctx, cancel := context.WithTimeout(context.Background(), h.wkrTimeout)
	ctx = context.WithValue(ctx, "seq", seq)
	defer cancel()

	pipeline := stage.MultiStage{}
	// Aggregation stage
	pipeline = append(pipeline, aggregation.NewStage(h.notifierCtl))
	// Notify stage
	pipeline = append(pipeline, notify.NewStage(h.notifierCtl))

	val := make(map[internal.Receiver][]*template.Alert)
	for _, receiver := range receivers {
		val[receiver] = alerts
	}

	_, _, err := pipeline.Exec(ctx, h.logger, val)
	if err != nil {
		return err.Error()
	}

	return ""
}
