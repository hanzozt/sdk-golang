package main

import (
	"context"
	"github.com/hanzozt/sdk-golang/zt"
	"io"
	"net"
	"net/http"
	"os"
)

func newZitiClient() *http.Client {
	zt.DefaultCollection.ForAll(func(ctx zt.Context) {
		ctx.Authenticate()
	})
	ztTransport := http.DefaultTransport.(*http.Transport).Clone() // copy default transport
	ztTransport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		dialer := zt.DefaultCollection.NewDialer()
		return dialer.Dial(network, addr)
	}
	ztTransport.TLSClientConfig.InsecureSkipVerify = true
	return &http.Client{Transport: ztTransport}
}

// this is a clone of ../curlz but showing the use of zt.Dialer
// identities are loaded from ZITI_IDENTITIES environment variable -- ';'-separated list of identity files
//
// saple usage:
// ```
//
//	$ export ZITI_IDENTITIES=<path to id file>
//	$ http-client http://<intercepted address>/path
//
// ```
func main() {
	resp, err := newZitiClient().Get(os.Args[1])
	if err != nil {
		panic(err)
	}

	_, err = io.Copy(os.Stdout, resp.Body)
	if err != nil {
		panic(err)
	}
}
