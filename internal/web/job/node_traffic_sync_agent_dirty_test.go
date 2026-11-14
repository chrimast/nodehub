package job

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/op/go-logging"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	xuilogger "github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

func TestHeartbeatProbesDisabledDirtyAgentForRecovery(t *testing.T) {
	xuilogger.InitLogger(logging.ERROR)
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/status":
			fmt.Fprint(w, `{"id":"agent","status":"ready"}`)
		case "/v1/capabilities":
			fmt.Fprint(w, `{"version":1,"config_save":true,"config_apply":true,"config_disable":true,"runtime_health":false}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	node := &model.Node{
		Name: "disabled-dirty-agent", NodeType: model.NodeTypeAgent, Scheme: "http",
		Address: "127.0.0.1", Port: server.Listener.Addr().(*net.TCPAddr).Port,
		ApiToken: "token", AllowPrivateAddress: true, Enable: false, Status: "offline",
		ConfigDirty: true, ConfigDirtyAt: time.Now().UnixMilli(),
	}
	if err := database.GetDB().Create(node).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Model(node).Update("enable", false).Error; err != nil {
		t.Fatal(err)
	}

	NewNodeHeartbeatJob().Run()

	var stored model.Node
	if err := database.GetDB().First(&stored, node.Id).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != "online" {
		t.Fatalf("disabled dirty agent status = %q, want online after heartbeat", stored.Status)
	}
}

func TestNodeTrafficSyncRunRetriesDisabledOfflineDirtyAgent(t *testing.T) {
	xuilogger.InitLogger(logging.ERROR)
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))
	runtime.SetManager(runtime.NewManager(runtime.LocalDeps{APIPort: func() int { return 0 }, SetNeedRestart: func() {}}))
	t.Cleanup(func() { runtime.SetManager(nil) })
	var disables atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/status":
			fmt.Fprint(w, `{"id":"agent","status":"ready"}`)
		case "/v1/capabilities":
			fmt.Fprint(w, `{"version":1,"config_save":true,"config_apply":true,"config_disable":true,"runtime_health":false}`)
		case "/v1/disable":
			disables.Add(1)
			fmt.Fprint(w, `{"status":"disabled"}`)
		}
	}))
	defer server.Close()
	node := &model.Node{
		Name: "pending-disabled-agent", NodeType: model.NodeTypeAgent, Scheme: "http",
		Address: "127.0.0.1", Port: server.Listener.Addr().(*net.TCPAddr).Port,
		ApiToken: "token", AllowPrivateAddress: true, Enable: false, Status: "offline",
		ConfigDirty: true, ConfigDirtyAt: time.Now().UnixMilli(),
	}
	if err := database.GetDB().Create(node).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Model(node).Update("enable", false).Error; err != nil {
		t.Fatal(err)
	}

	NewNodeTrafficSyncJob().Run()

	if got := disables.Load(); got != 1 {
		t.Fatalf("disable requests = %d, want one during Run", got)
	}
	var stored model.Node
	if err := database.GetDB().First(&stored, node.Id).Error; err != nil {
		t.Fatal(err)
	}
	if stored.ConfigDirty || stored.Enable {
		t.Fatalf("disable did not leave a clean disabled node: %+v", stored)
	}
}

func TestNodeTrafficSyncAppliesDirtyAgentConfigOnTick(t *testing.T) {
	tests := []struct {
		name       string
		response   string
		statusCode int
		withConfig bool
		wantDirty  bool
	}{
		{name: "applied success clears dirty", response: `{"status":"applied"}`, withConfig: true, wantDirty: false},
		{name: "disabled success clears dirty", response: `{"status":"disabled"}`, wantDirty: false},
		{name: "http failure keeps dirty", response: `{"error":"failed"}`, statusCode: http.StatusBadGateway, wantDirty: true},
		{name: "unknown acknowledgement keeps dirty", response: `{"status":"saved"}`, wantDirty: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			xuilogger.InitLogger(logging.ERROR)
			dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v1/status":
					fmt.Fprint(w, `{"id":"agent-1","status":"ready"}`)
				case "/v1/capabilities":
					configApply := "false"
					if tc.withConfig {
						configApply = "true"
					}
					fmt.Fprintf(w, `{"version":1,"config_save":true,"config_apply":%s,"config_disable":true,"runtime_health":false}`, configApply)
				case "/v1/disable":
					if tc.statusCode != 0 {
						w.WriteHeader(tc.statusCode)
					}
					fmt.Fprint(w, tc.response)
				case "/v1/config":
					if tc.statusCode != 0 {
						w.WriteHeader(tc.statusCode)
					}
					fmt.Fprint(w, tc.response)
				default:
					http.NotFound(w, r)
				}
			}))
			t.Cleanup(server.Close)

			node := &model.Node{
				Name:                "dirty-agent",
				NodeType:            model.NodeTypeAgent,
				Scheme:              "http",
				Address:             "127.0.0.1",
				Port:                server.Listener.Addr().(*net.TCPAddr).Port,
				ApiToken:            "agent-token",
				AllowPrivateAddress: true,
				Enable:              true,
				Status:              "online",
			}
			if err := (&service.NodeService{}).Create(node); err != nil {
				t.Fatal(err)
			}
			if tc.withConfig {
				if err := database.GetDB().Create(&model.Inbound{
					Tag: "agent-inbound", NodeID: &node.Id, Enable: true, Port: 12345,
					Protocol: model.VLESS, Settings: `{"clients":[]}`,
				}).Error; err != nil {
					t.Fatal(err)
				}
			}
			dirtyAt := time.Now().UnixMilli()
			if err := database.GetDB().Model(&model.Node{}).Where("id = ?", node.Id).Updates(map[string]any{
				"config_dirty":    true,
				"config_dirty_at": dirtyAt,
			}).Error; err != nil {
				t.Fatal(err)
			}
			node.ConfigDirty = true
			node.ConfigDirtyAt = dirtyAt

			mgr := runtime.NewManager(runtime.LocalDeps{APIPort: func() int { return 0 }, SetNeedRestart: func() {}})
			(&NodeTrafficSyncJob{}).syncOne(mgr, node, false)

			var stored model.Node
			if err := database.GetDB().First(&stored, node.Id).Error; err != nil {
				t.Fatal(err)
			}
			if stored.ConfigDirty != tc.wantDirty {
				t.Fatalf("ConfigDirty = %v, want %v", stored.ConfigDirty, tc.wantDirty)
			}
		})
	}
}
