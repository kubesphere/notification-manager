/*
Copyright 2020 The KubeSphere Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"github.com/emicklei/go-restful"
	"github.com/golang/glog"
	"github.com/prometheus/common/model"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	TivoliAlertUnknown  = 1
	TivoliAlertError    = 3
	TivoliAlertCritical = 4

	GoroutinesNumMax      = 100
	WaitGoroutinesTimeout = 5 * time.Second
	SendMessageTimeout    = 5 * time.Second
)

var (
	ip                    string
	port                  int
	goroutinesNum         int
	chanLen               int
	waitGoroutinesTimeout time.Duration
	sendTimeout           time.Duration
	ch                    chan *Alert
	si                    *Statistics
	waitHandlerGroup      sync.WaitGroup
)

func main() {

	cmd := NewServerCommand()

	if err := cmd.Execute(); err != nil {
		log.Fatalln(err)
	}
}

func AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&ip, "ip", "", "Socket ip")
	fs.IntVar(&port, "port", 8080, "Socket port")
	fs.IntVar(&goroutinesNum, "goroutines-num", GoroutinesNumMax, "the max num of goroutines to send alert,default 1000")
	fs.IntVar(&chanLen, "channel-len", 1000, "the capability of channel, default 1000")
	fs.DurationVar(&waitGoroutinesTimeout, "wait-timeout", WaitGoroutinesTimeout, "the time to wait for a new goroutines, default 5s")
	fs.DurationVar(&sendTimeout, "send-timeout", SendMessageTimeout, "the time to send message, default 5s")
}

func NewServerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "notification-adapter",
		Long: `The webhook to receive alert from notification manager, and send to socket`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return Run()
		},
	}
	AddFlags(cmd.Flags())
	cmd.Flags().AddGoFlagSet(flag.CommandLine)

	return cmd
}

func Run() error {

	pflag.VisitAll(func(flag *pflag.Flag) {
		glog.Errorf("FLAG: --%s=%q", flag.Name, flag.Value)
	})

	ch = make(chan *Alert, chanLen)
	si = NewStatisticsInfo(ch)

	go work()

	return httpserver()
}

func httpserver() error {
	container := restful.NewContainer()
	ws := new(restful.WebService)
	ws.Path("").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)
	ws.Route(ws.GET("/statistics/status").To(statisticsStatusGet))
	ws.Route(ws.PUT("/statistics/status").To(statisticsStatusUpdate))
	ws.Route(ws.GET("/statistics/freshTime").To(statisticsFreshTimeGet))
	ws.Route(ws.PUT("/statistics/freshTime").To(statisticsFreshTimeUpdate))
	ws.Route(ws.GET("/statistics/info").To(statisticsInfo))
	ws.Route(ws.POST("/alerts").To(handler))
	ws.Route(ws.GET("/readiness").To(readiness))
	ws.Route(ws.GET("/liveness").To(readiness))
	ws.Route(ws.GET("/preStop").To(preStop))

	container.Add(ws)

	server := &http.Server{
		Addr:    ":8080",
		Handler: container,
	}

	if err := server.ListenAndServe(); err != nil {
		glog.Fatal(err)
	}

	return nil
}

func work() {
	routinesChan := make(chan interface{}, goroutinesNum)

	for {
		alert := <-ch
		if alert == nil {
			break
		}
		alert.PullTime = time.Now()

		if err := tryAdd(routinesChan, waitGoroutinesTimeout); err != nil {
			alert.WaitRoutinesTimeout = true
			si.StatisticsStep(alert)
			glog.Error("get goroutines timeout")
			continue
		}

		go func() {
			defer func() {
				<-routinesChan
				si.StatisticsStep(alert)
			}()

			stopCh := make(chan interface{}, 1)
			go func() {
				sendMessage(alert)
				close(stopCh)
			}()

			if err := wait(stopCh, sendTimeout); err != nil {
				alert.SendTimeout = true
				glog.Errorf("send alert timeout")
				return
			}

			alert.SendTime = time.Now()
		}()
	}
}

func tryAdd(ch chan interface{}, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	select {
	case ch <- struct{}{}:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("timeout")
	}
}

func wait(ch chan interface{}, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("timeout")
	}
}

func handler(req *restful.Request, resp *restful.Response) {

	waitHandlerGroup.Add(1)
	defer waitHandlerGroup.Done()

	body, err := ioutil.ReadAll(req.Request.Body)
	if err != nil {
		glog.Errorf("read request body error, %s", err)
		err := resp.WriteHeaderAndEntity(http.StatusBadRequest, "")
		if err != nil {
			glog.Errorf("response error %s", err)
		}
		return
	}

	alerts, err := NewAlerts(body)
	if err != nil {
		err := resp.WriteHeaderAndEntity(http.StatusBadRequest, "")
		if err != nil {
			glog.Errorf("response error %s", err)
		}
		return
	}

	for _, alert := range alerts {
		ch <- alert
		alert.PushTime = time.Now()
	}

	err = resp.WriteHeaderAndEntity(http.StatusOK, "")
	if err != nil {
		glog.Errorf("response error %s", err)
	}
}

func sendMessage(alert *Alert) {

	msg := getMessage(alert)
	if len(msg) == 0 {
		return
	}

	addr := fmt.Sprintf("%s:%d", ip, port)
	conn, err := net.DialTimeout("tcp", addr, sendTimeout)
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

func getMessage(alert *Alert) string {

	level := TivoliAlertUnknown
	if t, ok := alert.Labels["severity"]; ok {
		switch t {
		case "critical":
			level = TivoliAlertCritical
		case "error":
		case "warning":
			level = TivoliAlertError
		}
	}

	if level < TivoliAlertError {
		return ""
	}

	return fmt.Sprintf("%s#%d#%s#%s#%s#%s#%d\n",
		alert.Labels["cluster"],
		model.LabelsToSignature(alert.Labels),
		alert.Labels["alertname"],
		alert.Labels["namespace"],
		alert.Annotations["message"],
		alert.Annotations["summaryCn"],
		level)
}

//readiness
func readiness(_ *restful.Request, resp *restful.Response) {

	responseWithHeaderAndEntity(resp, http.StatusOK, "")
}

//preStop
func preStop(_ *restful.Request, resp *restful.Response) {

	waitHandlerGroup.Wait()
	glog.Errorf("msg handler close, wait pool close")
	close(ch)
	responseWithHeaderAndEntity(resp, http.StatusOK, "")
	glog.Flush()
}

//get statistics fresh time
func statisticsFreshTimeGet(_ *restful.Request, resp *restful.Response) {
	responseWithJson(resp, si.FreshTime)
}

//set statistics fresh time
func statisticsFreshTimeUpdate(req *restful.Request, resp *restful.Response) {

	s := req.QueryParameter("freshTime")
	n, err := strconv.Atoi(s)
	if err != nil {
		responseWithHeaderAndEntity(resp, http.StatusBadRequest, "parameter error")
		return
	}

	si.SetFreshTime(time.Second * time.Duration(n))
	responseWithJson(resp, "Success")
}

//get statistics status
func statisticsStatusGet(_ *restful.Request, resp *restful.Response) {
	responseWithJson(resp, si.Enable)
}

//set statistics status
func statisticsStatusUpdate(req *restful.Request, resp *restful.Response) {

	enable := req.QueryParameter("enable")
	b, err := strconv.ParseBool(enable)
	if err != nil {
		responseWithHeaderAndEntity(resp, http.StatusBadRequest, "parameter error")
		return
	}

	si.Enable = b
	responseWithJson(resp, "Success")
}

//get statistics info
func statisticsInfo(_ *restful.Request, resp *restful.Response) {
	responseWithJson(resp, si.Print())
}

func responseWithJson(resp *restful.Response, value interface{}) {
	e := resp.WriteAsJson(value)
	if e != nil {
		glog.Errorf("response error %s", e)
	}
}

func responseWithHeaderAndEntity(resp *restful.Response, status int, value interface{}) {
	e := resp.WriteHeaderAndEntity(status, value)
	if e != nil {
		glog.Errorf("response error %s", e)
	}
}
