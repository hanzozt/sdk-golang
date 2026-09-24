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
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-openapi/runtime"
	"github.com/hanzozt/edge-api/rest_client_api_client/service"
	"github.com/hanzozt/edge-api/rest_model"
	"github.com/hanzozt/identity"
	"github.com/hanzozt/metrics"
	edgeapis "github.com/hanzozt/sdk-golang/edge-apis"
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
