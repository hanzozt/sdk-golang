/*
	Copyright 2019 NetFoundry Inc.

	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at

	https://www.apache.org/licenses/LICENSE-2.0

	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
*/

package main

import (
	"context"
	"fmt"
	"github.com/hanzozt/sdk-golang/zt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
)

type ZitiDialContext struct {
	context zt.Context
}

func (dc *ZitiDialContext) Dial(_ context.Context, _ string, addr string) (net.Conn, error) {
	service := strings.Split(addr, ":")[0] // will always get passed host:port
	return dc.context.Dial(service)
}

func newZitiClient() *http.Client {
	// Get identity config
	cfg, err := zt.NewConfigFromFile(os.Args[2])
	if err != nil {
		panic(err)
	}

	ctx, err := zt.NewContext(cfg)

	if err != nil {
		panic(err)
	}

	ztDialContext := ZitiDialContext{context: ctx}

	ztTransport := http.DefaultTransport.(*http.Transport).Clone() // copy default transport
	ztTransport.DialContext = ztDialContext.Dial
	ztTransport.TLSClientConfig.InsecureSkipVerify = true
	return &http.Client{Transport: ztTransport}
}

func main() {
	if len(os.Args) < 3 {
		fmt.Printf("Insufficient arguments provided\n\nUsage: ./curlz <serviceName> <identityFile>\n\n")
		return
	}

	resp, err := newZitiClient().Get(os.Args[1])
	if err != nil {
		panic(err)
	}

	_, err = io.Copy(os.Stdout, resp.Body)
	if err != nil {
		panic(err)
	}
}
