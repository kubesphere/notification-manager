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
	"adapter/pkg/common"
	"adapter/pkg/export"
	"adapter/pkg/export/stdout"
	"adapter/pkg/export/tivoli"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync"

	"github.com/emicklei/go-restful"
	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	withTivoli bool
	withStdout bool

	exporters []export.Exporter

	waitHandlerGroup sync.WaitGroup
)

func main() {

	cmd := NewServerCommand()

	if err := cmd.Execute(); err != nil {
		log.Fatalln(err)
	}
}

func AddFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&withTivoli, "with-tivoli", false, "Export to tivoli")
	fs.BoolVar(&withStdout, "with-stdout", false, "Export to stdout")
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

	if withTivoli {
		exporters = append(exporters, tivoli.NewExporter())
	}

	if withStdout {
		exporters = append(exporters, stdout.NewExporter())
	}

	return httpserver()
}

func httpserver() error {
	container := restful.NewContainer()
	ws := new(restful.WebService)
	ws.Path("").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)
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

	alerts, err := common.NewAlerts(body)
	if err != nil {
		err := resp.WriteHeaderAndEntity(http.StatusBadRequest, "")
		if err != nil {
			glog.Errorf("response error %s", err)
		}
		return
	}

	for _, exporter := range exporters {
		if err := exporter.Export(alerts); err != nil {
			fmt.Println(err)
		}
	}

	err = resp.WriteHeaderAndEntity(http.StatusOK, "")
	if err != nil {
		glog.Errorf("response error %s", err)
	}
}

//readiness
func readiness(_ *restful.Request, resp *restful.Response) {

	responseWithHeaderAndEntity(resp, http.StatusOK, "")
}

//preStop
func preStop(_ *restful.Request, resp *restful.Response) {

	waitHandlerGroup.Wait()
	glog.Errorf("msg handler close, wait exporters close")
	for _, e := range exporters {
		_ = e.Close()
	}
	responseWithHeaderAndEntity(resp, http.StatusOK, "")
	glog.Flush()
}

func responseWithHeaderAndEntity(resp *restful.Response, status int, value interface{}) {
	e := resp.WriteHeaderAndEntity(status, value)
	if e != nil {
		glog.Errorf("response error %s", e)
	}
}
