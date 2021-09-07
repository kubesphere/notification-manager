package tivoli

import (
	"adapter/pkg/common"
	"adapter/pkg/export"
	"bytes"
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/prometheus/common/model"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

const (
	AlertUnknown  = 1
	AlertOrdinary = 2
	AlertError    = 3
	AlertCritical = 4

	GoroutinesNumMax      = 100
	WaitGoroutinesTimeout = 5 * time.Second
	SendMessageTimeout    = 5 * time.Second
)

type exporter struct {
	ip                    string
	port                  int
	goroutinesNum         int
	ch                    chan *common.Alert
	chanLen               int
	waitGoroutinesTimeout time.Duration
	sendTimeout           time.Duration

	wg sync.WaitGroup
}

var (
	ip                    string
	port                  int
	goroutinesNum         int
	chanLen               int
	waitGoroutinesTimeout time.Duration
	sendTimeout           time.Duration
)

func init() {
	flag.StringVar(&ip, "ip", "", "Socket ip")
	flag.IntVar(&port, "port", 8080, "Socket port")
	flag.IntVar(&goroutinesNum, "goroutines-num", GoroutinesNumMax, "the max num of goroutines to send alert,default 1000")
	flag.IntVar(&chanLen, "channel-len", 1000, "the capability of channel, default 1000")
	flag.DurationVar(&waitGoroutinesTimeout, "wait-timeout", WaitGoroutinesTimeout, "the time to wait for a new goroutines, default 5s")
	flag.DurationVar(&sendTimeout, "send-timeout", SendMessageTimeout, "the time to send message, default 5s")
}

func NewExporter() export.Exporter {

	e := &exporter{
		ip:                    ip,
		port:                  port,
		goroutinesNum:         goroutinesNum,
		chanLen:               chanLen,
		waitGoroutinesTimeout: waitGoroutinesTimeout,
		sendTimeout:           sendTimeout,
	}

	e.ch = make(chan *common.Alert, e.chanLen)

	go e.work()

	return e
}

func (e *exporter) Export(alerts []*common.Alert) error {

	for _, alert := range alerts {
		e.ch <- alert
	}

	return nil
}

func (e *exporter) Close() error {
	close(e.ch)
	e.wg.Wait()
	return nil
}

func (e *exporter) work() {

	e.wg.Add(1)
	defer e.wg.Done()

	routinesChan := make(chan interface{}, e.goroutinesNum)

	for {
		alert := <-e.ch
		if alert == nil {
			break
		}

		if err := e.tryAdd(routinesChan); err != nil {
			glog.Error("get goroutines timeout")
			continue
		}

		go func() {
			defer func() {
				<-routinesChan
			}()

			stopCh := make(chan interface{}, 1)
			go func() {
				e.sendMessage(alert)
				close(stopCh)
			}()

			if err := e.wait(stopCh); err != nil {
				glog.Errorf("send alert timeout")
				return
			}
		}()
	}
}

func (e *exporter) tryAdd(ch chan interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), e.waitGoroutinesTimeout)
	defer cancel()

	select {
	case ch <- struct{}{}:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("timeout")
	}
}

func (e *exporter) wait(ch chan interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), e.sendTimeout)
	defer cancel()

	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("timeout")
	}
}

func (e *exporter) sendMessage(alert *common.Alert) {

	msg := getMessage(alert)
	if len(msg) == 0 {
		return
	}

	addr := fmt.Sprintf("%s:%d", e.ip, e.port)
	conn, err := net.DialTimeout("tcp", addr, e.sendTimeout)
	if err != nil {
		glog.Errorf("connect to %s error, %s", addr, err.Error())
		return
	}
	defer func() {
		if err := conn.Close(); err != nil {
			glog.Errorf("close connect error, %s", err.Error())
		}
	}()

	reader := transform.NewReader(bytes.NewReader([]byte(msg)), simplifiedchinese.GBK.NewEncoder())
	bs, err := ioutil.ReadAll(reader)
	if err != nil {
		glog.Errorf("transform msg error, %s", err.Error())
		return
	}

	body := bs
	size := 0
	for {
		n, err := conn.Write(body)
		if err != nil {
			glog.Errorf("write error, %s", err.Error())
		}

		size = size + n
		if size == len(bs) {
			break
		}

		body = bs[size:]
	}

	return
}

func getMessage(alert *common.Alert) string {

	level := AlertUnknown
	if t, ok := alert.Labels["severity"]; ok {
		switch t {
		case "critical":
			level = AlertCritical
		case "error":
			level = AlertError
		case "warning":
			level = AlertOrdinary
		}
	}

	if level < AlertOrdinary {
		return ""
	}

	cluster := alert.Labels["cluster"]
	array := strings.Split(cluster, "_")
	department := ""
	if array != nil {
		if len(array) > 0 {
			cluster = array[0]
		}

		if len(array) > 1 {
			department = array[1]
		}
	}

	return fmt.Sprintf("%s#%s#%d#%s#%s#%s#%s#%d\n",
		cluster,
		department,
		model.LabelsToSignature(alert.Labels),
		alert.Labels["alertname"],
		alert.Labels["namespace"],
		alert.Annotations["message"],
		alert.Annotations["summaryCn"],
		level)
}
