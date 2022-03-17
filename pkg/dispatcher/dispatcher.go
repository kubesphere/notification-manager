package dispatcher

import (
	"context"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/pkg/aggregation"
	"github.com/kubesphere/notification-manager/pkg/controller"
	"github.com/kubesphere/notification-manager/pkg/filter"
	"github.com/kubesphere/notification-manager/pkg/history"
	"github.com/kubesphere/notification-manager/pkg/notify"
	"github.com/kubesphere/notification-manager/pkg/route"
	"github.com/kubesphere/notification-manager/pkg/silence"
	"github.com/kubesphere/notification-manager/pkg/stage"
	"github.com/kubesphere/notification-manager/pkg/store"
	"github.com/kubesphere/notification-manager/pkg/template"
	"github.com/kubesphere/notification-manager/pkg/utils"
)

type Dispatcher struct {
	l           log.Logger
	notifierCtl *controller.Controller
	alerts      *store.AlertStore

	scheduleTimeout time.Duration
	wkrTimeout      time.Duration

	semCh chan struct{}
	seq   int64
}

func New(l log.Logger, notifierCtl *controller.Controller, alerts *store.AlertStore, scheduleTimeout time.Duration, wkrTimeout time.Duration, workerQueue int) *Dispatcher {

	return &Dispatcher{
		l:               l,
		notifierCtl:     notifierCtl,
		alerts:          alerts,
		scheduleTimeout: scheduleTimeout,
		wkrTimeout:      wkrTimeout,
		semCh:           make(chan struct{}, workerQueue),
	}
}

func (d *Dispatcher) Run() error {

	for {
		// err is not nil means the store had closed, dispatcher should process remaining alerts, then exit.
		if alerts, err := d.alerts.Pull(d.notifierCtl.GetBatchMaxSize(), d.notifierCtl.GetBatchMaxWait()); err == nil {
			go d.processAlerts(alerts)
		} else {
			d.processAlerts(alerts)
			return nil
		}
	}
}

func (d *Dispatcher) processAlerts(alerts []*template.Alert) {

	if len(alerts) == 0 {
		return
	}

	_ = level.Debug(d.l).Log("msg", "Dispatcher: Begins to process alerts...", "alerts", len(alerts))

	if err := d.getWorker(); err != nil {
		return
	}
	defer d.releaseWorker()

	d.seq = d.seq + 1
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), d.wkrTimeout)
	ctx = context.WithValue(ctx, "seq", d.seq)
	defer cancel()

	stopCh := make(chan struct{})
	go d.worker(ctx, alerts, stopCh)

	select {
	case <-stopCh:
		elapsed := time.Since(start).String()
		_ = level.Debug(d.l).Log("msg", "Dispatcher: Processor exit after "+elapsed)
		return
	case <-ctx.Done():
		if err := ctx.Err(); err != nil {
			_ = level.Warn(d.l).Log("msg", "Dispatcher: process alerts timeout in "+d.wkrTimeout.String(), "error", err.Error())
		}
		return
	}
}

func (d *Dispatcher) getWorker() error {
	ctx, cancel := context.WithTimeout(context.Background(), d.scheduleTimeout)
	defer cancel()
	select {
	case d.semCh <- struct{}{}:
		_ = level.Debug(d.l).Log("msg", "Dispatcher: Acquired worker queue lock...")
	case <-ctx.Done():
		_ = level.Warn(d.l).Log("msg", "Dispatcher: Running out of queue capacity in "+d.scheduleTimeout.String(), "error", ctx.Err())
		return utils.Error("Time out")
	}

	return nil
}

func (d *Dispatcher) releaseWorker() {
	d.semCh <- struct{}{}
}

func (d *Dispatcher) worker(ctx context.Context, data interface{}, stopCh chan struct{}) {

	pipeline := stage.MultiStage{}
	// Global silence stage
	pipeline = append(pipeline, silence.NewStage(d.notifierCtl))
	// Route stage
	pipeline = append(pipeline, route.NewStage(d.notifierCtl))
	// Tenant silence stage
	pipeline = append(pipeline, filter.NewStage(d.notifierCtl))
	// Aggregation stage
	pipeline = append(pipeline, aggregation.NewStage(d.notifierCtl))
	// Notify stage
	pipeline = append(pipeline, notify.NewStage(d.notifierCtl))
	// History stage
	pipeline = append(pipeline, history.NewStage(d.notifierCtl))

	if _, _, err := pipeline.Exec(ctx, d.l, data); err != nil {
		_ = level.Error(d.l).Log("msg", "Dispatcher: process alerts failed", "seq", ctx.Value("seq"))
	}

	stopCh <- struct{}{}
}
