package auth

import (
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestCheckHealthReusesConnection(t *testing.T) {
	var newConns atomic.Int32
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	srv.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			newConns.Add(1)
		}
	}
	srv.StartTLS()
	defer srv.Close()

	client := srv.Client()
	for range 10 {
		checkHealth(client, srv.URL+"/api/v1/health", 0)
	}

	if got := newConns.Load(); got != 1 {
		t.Fatalf("expected 1 connection across 10 health checks, got %d", got)
	}
}
