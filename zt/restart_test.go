package zt

import (
	"encoding/json"
	"encoding/pem"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-openapi/runtime"
	"github.com/hanzozt/channel/v4"
	"github.com/hanzozt/channel/v4/latency"
	"github.com/hanzozt/edge-api/rest_client_api_client/service"
	"github.com/hanzozt/edge-api/rest_model"
	"github.com/hanzozt/identity"
	"github.com/hanzozt/metrics"
	edgeapis "github.com/hanzozt/sdk-golang/edge-apis"
	"github.com/hanzozt/sdk-golang/zt/edge"
	"github.com/hanzozt/transport/v2"
	"github.com/hanzozt/transport/v2/tcp"
	"github.com/stretchr/testify/require"
)

// These tests restart the two things a hosted service depends on — the
// controller (it answers 502 while down) and the edge router (its channel
// closes and its port refuses) — and assert the service comes back by itself.

const restartSvc = "restart-svc"

// ctrl is a controller's edge client API that can be taken down.
type ctrl struct {
	srv        *httptest.Server
	router     string
	down       atomic.Bool
	failed     atomic.Int32 // requests answered 502
	listFailed atomic.Int32 // GET /services answered 502
}

func newCtrl(t *testing.T, router string) *ctrl {
	c := &ctrl{router: router}
	c.srv = httptest.NewTLSServer(http.HandlerFunc(c.serve))
	t.Cleanup(c.srv.Close)
	return c
}

func (c *ctrl) serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	path := strings.TrimPrefix(r.URL.Path, "/edge/client/v1")
	if c.down.Load() {
		c.failed.Add(1)
		if r.Method == http.MethodGet && path == "/services" {
			c.listFailed.Add(1)
		}
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("{}"))
		return
	}

	switch {
	case r.Method == http.MethodPost && path == "/sessions":
		c.reply(w, http.StatusCreated, c.session())
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/sessions/"):
		c.reply(w, http.StatusOK, c.session())
	case r.Method == http.MethodGet && path == "/current-api-session/service-updates":
		c.reply(w, http.StatusOK, map[string]any{"lastChangeAt": "2026-01-01T00:00:00.000Z"})
	case r.Method == http.MethodGet && path == "/services":
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []*rest_model.ServiceDetail{hosted()},
			"meta": map[string]any{"pagination": map[string]any{"limit": 500, "offset": 0, "totalCount": 1}},
		})
	default:
		c.reply(w, http.StatusNotFound, map[string]any{})
	}
}

func (c *ctrl) reply(w http.ResponseWriter, status int, data any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"data": data, "meta": map[string]any{}})
}

func (c *ctrl) session() *rest_model.SessionDetail {
	id, token, kind := "session", "session-token", rest_model.DialBindBind
	svc, name := *hosted().ID, "router"
	return &rest_model.SessionDetail{
		BaseEntity: rest_model.BaseEntity{ID: &id},
		ServiceID:  &svc,
		Token:      &token,
		Type:       &kind,
		EdgeRouters: []*rest_model.SessionEdgeRouter{{
			CommonEdgeRouterProperties: rest_model.CommonEdgeRouterProperties{
				Name:               &name,
				SupportedProtocols: map[string]string{"tcp": c.router},
			},
		}},
	}
}

// hosted is the service the tests bind.
func hosted() *rest_model.ServiceDetail {
	id, name, encrypted := "svc-id", restartSvc, false
	return &rest_model.ServiceDetail{
		BaseEntity:         rest_model.BaseEntity{ID: &id},
		Name:               &name,
		EncryptionRequired: &encrypted,
		Permissions:        []rest_model.DialBind{rest_model.DialBindBind},
	}
}

// newHost is a context signed in to c, with the hosted service in its list.
func newHost(t *testing.T, c *ctrl, options *Options) *ContextImpl {
	cert, err := newTestSelfSignedCert("host", nil, time.Hour)
	require.NoError(t, err)
	caPem := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.srv.Certificate().Raw})
	id, err := identity.LoadIdentity(identity.Config{
		Cert: "pem:" + string(cert.CertPEM),
		Key:  "pem:" + string(cert.KeyPEM),
		CA:   "pem:" + string(caPem),
	})
	require.NoError(t, err)

	ztx, err := NewContextWithOpts(&Config{
		ZtAPI:       c.srv.URL + "/edge/client/v1",
		Credentials: edgeapis.NewIdentityCredentials(id),
	}, options)
	require.NoError(t, err)

	ctx := ztx.(*ContextImpl)
	var session edgeapis.ApiSession = edgeapis.NewApiSessionLegacy("api-session-token")
	ctx.CtrlClt.ApiSession.Store(&session)
	ctx.metrics = metrics.NewRegistry("host", nil)
	ctx.services.Set(restartSvc, hosted())
	t.Cleanup(ctx.Close)
	return ctx
}

// A service refresh the controller does not answer is retried within seconds,
// not after a whole refresh interval.
func TestServiceRefreshRetriesUnansweredRefresh(t *testing.T) {
	c := newCtrl(t, "tcp:127.0.0.1:1")
	c.down.Store(true)

	interval := 4 * time.Second
	ctx := newHost(t, c, &Options{RefreshInterval: interval})
	ctx.services.Remove(restartSvc)
	go ctx.runRefreshes()

	// a refresh asks for the update time, then the list: the list's 502 fails it
	require.Eventually(t, func() bool { return c.listFailed.Load() > 0 }, interval+2*time.Second, 10*time.Millisecond,
		"the first refresh never reached the controller")
	failedAt := time.Now()
	c.down.Store(false)

	require.Eventually(t, func() bool {
		_, found := ctx.GetService(restartSvc)
		return found
	}, interval-time.Second, 10*time.Millisecond, "the refresh was not retried before the next interval")
	t.Logf("service list loaded %v after the 502", time.Since(failedAt).Round(time.Millisecond))
}

func TestTransient(t *testing.T) {
	refused := &url.Error{Op: "Get", URL: "https://ctrl", Err: &net.OpError{Op: "dial", Err: errors.New("connection refused")}}
	for name, c := range map[string]struct {
		err  error
		want bool
	}{
		"502":         {runtime.NewAPIError("listServices", nil, http.StatusBadGateway), true},
		"503":         {runtime.NewAPIError("listServices", nil, http.StatusServiceUnavailable), true},
		"404":         {runtime.NewAPIError("detailSession", nil, http.StatusNotFound), false},
		"401 typed":   {&service.ListServicesUnauthorized{}, false},
		"no response": {refused, true},
		"local":       {errors.New("token is not a JWT"), false},
		"no error":    {nil, false},
	} {
		require.Equal(t, c.want, transient(c.err), name)
	}
}

// router is an edge router that accepts binds and can be restarted on its port.
type router struct {
	t     *testing.T
	addr  transport.Address
	url   string
	binds chan struct{}

	sync.Mutex
	ln  channel.UnderlayListener
	chs []channel.Channel
}

func newRouter(t *testing.T) *router {
	transport.AddAddressParser(tcp.AddressParser{})

	probe, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	url := "tcp:" + probe.Addr().String()
	require.NoError(t, probe.Close())

	addr, err := transport.ParseAddress(url)
	require.NoError(t, err)

	r := &router{t: t, addr: addr, url: url, binds: make(chan struct{}, 16)}
	r.start()
	t.Cleanup(r.stop)
	return r
}

func (r *router) start() {
	ln := channel.NewClassicListener(&identity.TokenId{Token: "router"}, r.addr, channel.ListenerConfig{
		ConnectOptions: channel.DefaultConnectOptions(),
	})
	require.NoError(r.t, ln.Listen())

	r.Lock()
	r.ln = ln
	r.Unlock()

	go func() {
		for {
			ch, err := channel.NewChannel("router", ln, channel.BindHandlerF(r.bind), channel.DefaultOptions())
			if err != nil {
				return
			}
			r.Lock()
			r.chs = append(r.chs, ch)
			r.Unlock()
		}
	}()
}

func (r *router) bind(b channel.Binding) error {
	b.AddTypedReceiveHandler(&latency.LatencyHandler{})
	b.AddReceiveHandlerF(edge.ContentTypeBind, func(m *channel.Message, ch channel.Channel) {
		connId, _ := m.GetUint32Header(edge.ConnIdHeader)
		reply := edge.NewStateConnectedMsg(connId)
		reply.ReplyTo(m)
		if err := ch.Send(reply); err == nil {
			r.binds <- struct{}{}
		}
	})
	return nil
}

// stop closes every channel and the port, as a router going down does.
func (r *router) stop() {
	r.Lock()
	defer r.Unlock()
	if r.ln != nil {
		_ = r.ln.Close()
		r.ln = nil
	}
	for _, ch := range r.chs {
		_ = ch.Close()
	}
	r.chs = nil
}

func (r *router) awaitBind(t *testing.T, within time.Duration, why string) {
	t.Helper()
	select {
	case <-r.binds:
	case <-time.After(within):
		t.Fatalf("no bind within %v: %s", within, why)
	}
}

// A service bound while the controller answers 502 binds once it answers,
// however long that takes — it does not give up and leave a listener that never binds.
func TestListenerBindsOnceControllerAnswers(t *testing.T) {
	r := newRouter(t)
	c := newCtrl(t, r.url)
	c.down.Store(true)
	ctx := newHost(t, c, nil)

	listener, err := ctx.ListenWithOptions(restartSvc, &ListenOptions{ConnectTimeout: time.Second})
	require.NoError(t, err)

	time.Sleep(3 * time.Second)
	c.down.Store(false)

	r.awaitBind(t, 10*time.Second, "controller answered, listener never bound")
	require.False(t, listener.(interface{ IsClosed() bool }).IsClosed())
}

// A hosted service re-binds as soon as its router is back, while the
// controller is still answering 502 — the session it holds is still good.
func TestListenerRebindsWhileControllerDown(t *testing.T) {
	if testing.Short() {
		t.Skip("waits out the 30s session refresh floor")
	}
	r := newRouter(t)
	c := newCtrl(t, r.url)
	ctx := newHost(t, c, nil)

	listener, err := ctx.ListenWithOptions(restartSvc, &ListenOptions{ConnectTimeout: 20 * time.Second})
	require.NoError(t, err)
	r.awaitBind(t, 5*time.Second, "first bind")

	// A session refresh is skipped within 30s of the last one; the restart
	// being modeled comes long after the bind.
	time.Sleep(31 * time.Second)

	c.down.Store(true)
	r.stop()
	time.Sleep(2 * time.Second)
	r.start()

	r.awaitBind(t, 5*time.Second, "router back, listener did not re-bind")
	require.False(t, listener.(interface{ IsClosed() bool }).IsClosed())
	require.Positive(t, c.failed.Load(), "the controller outage was never seen")
}
