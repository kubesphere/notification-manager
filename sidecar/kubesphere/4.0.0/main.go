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
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/emicklei/go-restful/v3"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/klog"
)

var (
	waitHandlerGroup sync.WaitGroup
	host             string
	username         string
	password         string
	interval         time.Duration

	b *Backend
)

func main() {
	cmd := NewServerCommand()
	if err := cmd.Execute(); err != nil {
		log.Fatalln(err)
	}
}

func AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&host, "host", "ks-apiserver.kubesphere-system", "the host of kubesphere apiserver")
	fs.StringVar(&username, "username", "", "the username of kubesphere")
	fs.StringVar(&password, "password", "", "the password of kubesphere")
	fs.DurationVar(&interval, "interval", time.Minute*5, "interval to reload")
}

func NewServerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "kubesphere-tenant-sidecar",
		Long: `The sidecar to determining which tenant should receive notificaitons`,
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
		klog.Errorf("FLAG: --%s=%q", flag.Name, flag.Value)
	})

	var err error
	b, err = NewBackend(host, username, password, interval)
	if err != nil {
		return err
	}

	b.Run()

	return httpserver()
}

func httpserver() error {
	container := restful.NewContainer()
	ws := new(restful.WebService)
	ws.Path("").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)
	ws.Route(ws.GET("/api/v2/tenant").To(handler))
	ws.Route(ws.GET("/readiness").To(readiness))
	ws.Route(ws.GET("/liveness").To(readiness))
	ws.Route(ws.GET("/preStop").To(preStop))

	container.Add(ws)

	server := &http.Server{
		Addr:    ":19094",
		Handler: container,
	}

	if err := server.ListenAndServe(); err != nil {
		klog.Fatal(err)
	}

	return nil
}

func handler(req *restful.Request, resp *restful.Response) {

	waitHandlerGroup.Add(1)
	defer waitHandlerGroup.Done()

	ns := req.QueryParameter("namespace")
	tenants := b.FromNamespace(ns)
	if tenants == nil {
		responseWithHeaderAndEntity(resp, http.StatusNotFound, "")
		return
	}

	responseWithJson(resp, tenants)
}

// readiness
func readiness(_ *restful.Request, resp *restful.Response) {

	responseWithHeaderAndEntity(resp, http.StatusOK, "")
}

// preStop
func preStop(_ *restful.Request, resp *restful.Response) {

	waitHandlerGroup.Wait()
	klog.Errorf("msg handler close, wait pool close")
	responseWithHeaderAndEntity(resp, http.StatusOK, "")
	klog.Flush()
}

func responseWithJson(resp *restful.Response, value interface{}) {
	e := resp.WriteAsJson(value)
	if e != nil {
		klog.Errorf("response error %s", e)
	}
}

func responseWithHeaderAndEntity(resp *restful.Response, status int, value interface{}) {
	e := resp.WriteHeaderAndEntity(status, value)
	if e != nil {
		klog.Errorf("response error %s", e)
	}
}
