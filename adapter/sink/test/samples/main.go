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
	"flag"
	"fmt"
	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
	"io/ioutil"
	"log"
	"net"
)

var (
	port int
)

func AddFlags(fs *pflag.FlagSet) {
	fs.IntVar(&port, "port", 8080, "Socket port")
}

func main() {
	cmd := newServerCommand()

	if err := cmd.Execute(); err != nil {
		log.Fatalln(err)
	}
}

func newServerCommand() *cobra.Command {
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

	l, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		return err
	}

	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Printf("accept error, %s\n", err.Error())
			continue
		}

		go func() {
			bs, err := ioutil.ReadAll(conn)
			if err != nil {
				fmt.Printf("read error, %s\n", err.Error())
				return
			}

			reader := transform.NewReader(bytes.NewReader(bs), simplifiedchinese.GBK.NewDecoder())
			d, err := ioutil.ReadAll(reader)
			if err != nil {
				fmt.Printf("transform error, %s\n", err.Error())
				return
			}

			fmt.Println(string(d))
		}()
	}
}
