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

func main() {
	target := os.Args[2]
	helloUrl := fmt.Sprintf("http://%s/hello", target)
	httpClient := createZitifiedHttpClient(os.Args[1])
	resp, e := httpClient.Get(helloUrl)
	if e != nil {
		panic(e)
	}
	body, _ := io.ReadAll(resp.Body)
	fmt.Println("Hello response:", string(body))

	a := 1
	b := 2
	addUrl := fmt.Sprintf("http://%s/add?a=%d&b=%d", target, a, b)
	resp, e = httpClient.Get(addUrl)
	if e != nil {
		panic(e)
	}
	body, _ = io.ReadAll(resp.Body)
	fmt.Println("Add Result:", string(body))
}

var ztContext zt.Context

func Dial(_ context.Context, _ string, addr string) (net.Conn, error) {
	service := strings.Split(addr, ":")[0] // will always get passed host:port
	return ztContext.Dial(service)
}
func createZitifiedHttpClient(idFile string) http.Client {
	cfg, err := zt.NewConfigFromFile(idFile)
	if err != nil {
		panic(err)
	}

	ztContext, err = zt.NewContext(cfg)

	if err != nil {
		panic(err)
	}

	ztTransport := http.DefaultTransport.(*http.Transport).Clone() // copy default transport
	ztTransport.DialContext = Dial                                 //ztDialContext.Dial
	return http.Client{Transport: ztTransport}
}
