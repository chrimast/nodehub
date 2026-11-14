package runtime

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestRemoteScanRealityNeighborsUsesAuthenticatedExistingRoute(t *testing.T) {
	var gotMethod, gotPath, gotBase, gotLimit string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		_ = r.ParseForm()
		gotBase, gotLimit = r.Form.Get("targets"), r.Form.Get("limit")
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Errorf("authorization missing")
		}
		_, _ = w.Write([]byte(`{"success":true,"obj":{"results":[],"scanned":253,"nodeId":0}}`))
	}))
	defer srv.Close()
	addr := strings.TrimPrefix(srv.URL, "http://")
	host, portText, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	r := NewRemote(&model.Node{Address: host, Port: port, Scheme: "http", AllowPrivateAddress: true, ApiToken: "token", BasePath: "/"}, nil)
	obj, err := r.ScanRealityNeighbors(context.Background(), 5)
	if err != nil {
		t.Fatalf("ScanRealityNeighbors: %v", err)
	}
	if string(obj) != `{"results":[],"scanned":253,"nodeId":0}` {
		t.Fatalf("obj=%s", obj)
	}
	if gotMethod != http.MethodPost || gotPath != "/panel/api/server/scanRealityNeighbors" || gotBase != "" || gotLimit != "5" {
		t.Fatalf("request=%s %s targets=%q limit=%q", gotMethod, gotPath, gotBase, gotLimit)
	}
}

func TestRemoteNeighborScanExceedsOrdinaryTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10500 * time.Millisecond)
		_, _ = w.Write([]byte(`{"success":true,"obj":{"results":[],"scanned":253,"nodeId":0}}`))
	}))
	defer srv.Close()
	host, portText, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	port, _ := strconv.Atoi(portText)
	r := NewRemote(&model.Node{Address: host, Port: port, Scheme: "http", AllowPrivateAddress: true, ApiToken: "token", BasePath: "/"}, nil)
	if _, err := r.ScanRealityNeighbors(context.Background(), 5); err != nil {
		t.Fatalf("long scan: %v", err)
	}
}
