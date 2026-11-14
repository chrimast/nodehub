package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestRealityNeighborResponseShape(t *testing.T) {
	var response struct {
		Results []*RealityScanResult `json:"results"`
		Scanned int                  `json:"scanned"`
	}
	if err := json.Unmarshal([]byte(`{"results":[{"ip":"198.51.100.8","feasible":true}],"scanned":253}`), &response); err != nil {
		t.Fatal(err)
	}
	if response.Scanned != 253 || len(response.Results) != 1 || !response.Results[0].Feasible {
		t.Fatalf("decoded response=%+v", response)
	}
	if _, _, err := (&ServerService{}).ScanRealityNeighborsLocal(context.Background(), 5); err == nil {
		t.Fatal("a cold local service must refuse a scan without a known public IPv4")
	}
}

func TestRemoteNeighborScanPreservesScannedCount(t *testing.T) {
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "test.db"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Errorf("missing authorization")
		}
		_, _ = w.Write([]byte(`{"success":true,"obj":{"results":[{"ip":"198.51.100.8","feasible":true}],"scanned":253,"nodeId":0}}`))
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	n := &model.Node{Name: "test", Address: u.Hostname(), Port: port, Scheme: "http", BasePath: "/", ApiToken: "token", AllowPrivateAddress: true}
	if err := database.GetDB().Create(n).Error; err != nil {
		t.Fatal(err)
	}
	results, scanned, err := (&NodeService{}).ScanRealityNeighborsForNode(context.Background(), n.Id, 5)
	if err != nil || scanned != 253 || len(results) != 1 || !results[0].Feasible {
		t.Fatalf("results=%v scanned=%d err=%v", results, scanned, err)
	}
}

func TestRemoteNeighborScanOldNodeReportsCompatibility(t *testing.T) {
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "test.db"))
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	n := &model.Node{Name: "old", Address: u.Hostname(), Port: port, Scheme: "http", BasePath: "/", ApiToken: "token", AllowPrivateAddress: true}
	if err := database.GetDB().Create(n).Error; err != nil {
		t.Fatal(err)
	}
	_, _, err := (&NodeService{}).ScanRealityNeighborsForNode(context.Background(), n.Id, 5)
	if err == nil || !strings.Contains(err.Error(), "upgrade") {
		t.Fatalf("expected upgrade error, got %v", err)
	}
}
