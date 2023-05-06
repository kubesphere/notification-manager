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
	"flag"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/emicklei/go-restful"
	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var waitHandlerGroup sync.WaitGroup

func main() {

	cmd := NewServerCommand()

	if err := cmd.Execute(); err != nil {
		log.Fatalln(err)
	}
}

func NewServerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "notification-adapter",
		Long: `The webhook to receive alert from notification manager, and send to socket`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return Run()
		},
	}
	cmd.Flags().AddGoFlagSet(flag.CommandLine)

	return cmd
}

func Run() error {

	pflag.VisitAll(func(flag *pflag.Flag) {
		glog.Errorf("FLAG: --%s=%q", flag.Name, flag.Value)
	})

	return httpserver()
}

func httpserver() error {
	container := restful.NewContainer()
	ws := new(restful.WebService)
	ws.Path("/api/v2").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)
	ws.Route(ws.GET("/tenant").To(handler))
	ws.Route(ws.GET("/readiness").To(readiness))
	ws.Route(ws.GET("/liveness").To(readiness))
	ws.Route(ws.GET("/preStop").To(preStop))

	container.Add(ws)

	server := &http.Server{
		Addr:    ":19094",
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

	ns := req.QueryParameter("namespace")
	if len(ns) == 0 {
		responseWithHeaderAndEntity(resp, http.StatusBadRequest, "namespace must not be nil")
		return
	}

	fmt.Printf("get tenants with namespace `%s`", ns)

	tenants := []string{ns}
	responseWithJson(resp, tenants)
}

// readiness
func readiness(_ *restful.Request, resp *restful.Response) {

	responseWithHeaderAndEntity(resp, http.StatusOK, "")
}

// preStop
func preStop(_ *restful.Request, resp *restful.Response) {

	glog.Errorf("waitting for message handler close")
	waitHandlerGroup.Wait()
	glog.Errorf("message handler closed")
	responseWithHeaderAndEntity(resp, http.StatusOK, "")
	glog.Flush()
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
