package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestConcurrentAgentAppliesSerializeReadThroughResponse(t *testing.T) {
	for _, firstStatus := range []int{http.StatusOK, http.StatusBadGateway} {
		t.Run(fmt.Sprint(firstStatus), func(t *testing.T) {
			setupSettingTestDB(t)
			arrived := make(chan string, 2)
			release := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v1/status":
					fmt.Fprint(w, `{"id":"agent","status":"ready"}`)
				case "/v1/capabilities":
					fmt.Fprint(w, `{"version":1,"config_save":true,"config_apply":true,"config_disable":true,"runtime_health":false}`)
				case "/v1/config":
					var body struct {
						Config struct {
							Inbounds []struct {
								Tag string `json:"tag"`
							} `json:"inbounds"`
						} `json:"config"`
					}
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Config.Inbounds) != 1 {
						t.Errorf("invalid config: %v", err)
						return
					}
					tag := body.Config.Inbounds[0].Tag
					arrived <- tag
					if tag == "old" {
						<-release
						if firstStatus != http.StatusOK {
							w.WriteHeader(firstStatus)
						}
					}
					fmt.Fprint(w, `{"status":"applied"}`)
				}
			}))
			defer server.Close()
			n := &model.Node{Name: "agent", NodeType: model.NodeTypeAgent, Scheme: "http", Address: "127.0.0.1", Port: server.Listener.Addr().(*net.TCPAddr).Port, ApiToken: "secret", AllowPrivateAddress: true, Enable: true}
			if err := (&NodeService{}).Create(n); err != nil {
				t.Fatal(err)
			}
			in := &model.Inbound{Tag: "old", NodeID: &n.Id, Enable: true, Port: 1234, Protocol: model.VLESS, Settings: `{"clients":[]}`}
			if err := database.GetDB().Create(in).Error; err != nil {
				t.Fatal(err)
			}

			type outcome struct {
				result AgentApplyResult
				err    error
			}
			done := make(chan outcome, 2)
			apply := func() {
				result, err := (&NodeService{}).ApplyAgentConfig(context.Background(), n.Id)
				done <- outcome{result, err}
			}
			go apply()
			if tag := <-arrived; tag != "old" {
				t.Fatalf("first tag = %s", tag)
			}
			if err := database.GetDB().Model(in).Update("tag", "new").Error; err != nil {
				t.Fatal(err)
			}
			go apply()
			earlyTag := ""
			select {
			case tag := <-arrived:
				earlyTag = tag
			case <-time.After(100 * time.Millisecond):
			}
			close(release)
			first, second := <-done, <-done
			if earlyTag != "" {
				t.Fatalf("new request reached agent before old response: %s", earlyTag)
			}
			if tag := <-arrived; tag != "new" {
				t.Fatalf("second tag = %s", tag)
			}
			if firstStatus == http.StatusBadGateway {
				if !((first.err != nil && first.result.Status == "unknown") || (second.err != nil && second.result.Status == "unknown")) {
					t.Fatalf("expected failed/unknown result: %+v %+v", first, second)
				}
			} else if first.err != nil || second.err != nil {
				t.Fatalf("apply errors: %v %v", first.err, second.err)
			}
		})
	}
}

func TestSetEnableWithResultDoesNotClearConcurrentGeneration(t *testing.T) {
	setupSettingTestDB(t)
	arrived := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/status":
			fmt.Fprint(w, `{"id":"agent","status":"ready"}`)
		case "/v1/capabilities":
			fmt.Fprint(w, `{"version":1,"config_save":true,"config_apply":true,"config_disable":true,"runtime_health":false}`)
		case "/v1/config":
			close(arrived)
			<-release
			fmt.Fprint(w, `{"status":"applied"}`)
		}
	}))
	defer server.Close()
	n := &model.Node{Name: "agent", NodeType: model.NodeTypeAgent, Scheme: "http", Address: "127.0.0.1", Port: server.Listener.Addr().(*net.TCPAddr).Port, ApiToken: "secret", AllowPrivateAddress: true, Enable: true}
	if err := (&NodeService{}).Create(n); err != nil {
		t.Fatal(err)
	}
	in := &model.Inbound{Tag: "in", NodeID: &n.Id, Enable: true, Port: 1234, Protocol: model.VLESS, Settings: `{"clients":[]}`}
	if err := database.GetDB().Create(in).Error; err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := (&NodeService{}).SetEnableWithResult(n.Id, true); done <- err }()
	<-arrived
	var before model.Node
	if err := database.GetDB().First(&before, n.Id).Error; err != nil {
		t.Fatal(err)
	}
	if err := (&NodeService{}).MarkNodeDirty(n.Id); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	var after model.Node
	if err := database.GetDB().First(&after, n.Id).Error; err != nil {
		t.Fatal(err)
	}
	if !after.ConfigDirty || after.ConfigDirtyAt <= before.ConfigDirtyAt {
		t.Fatalf("concurrent generation cleared: before=%d after=%+v", before.ConfigDirtyAt, after)
	}
}

func TestSetEnableAgentTransactionRollbackAndJSON(t *testing.T) {
	for _, enable := range []bool{false, true} {
		t.Run(fmt.Sprint(enable), func(t *testing.T) {
			setupSettingTestDB(t)
			n := &model.Node{Name: "agent", NodeType: model.NodeTypeAgent, Address: "127.0.0.1", Port: 9443, ApiToken: "secret", Enable: true, Status: "offline"}
			if err := (&NodeService{}).Create(n); err != nil {
				t.Fatal(err)
			}
			if err := database.GetDB().Model(n).Update("enable", !enable).Error; err != nil {
				t.Fatal(err)
			}
			if err := database.GetDB().Exec(`CREATE TRIGGER fail_enable BEFORE UPDATE OF enable ON nodes BEGIN SELECT RAISE(ABORT, 'rejected'); END`).Error; err != nil {
				t.Fatal(err)
			}
			if _, err := (&NodeService{}).SetEnableWithResult(n.Id, enable); err == nil {
				t.Fatal("expected transaction error")
			}
			var stored model.Node
			if err := database.GetDB().First(&stored, n.Id).Error; err != nil {
				t.Fatal(err)
			}
			if stored.Enable == enable || stored.ConfigDirty || stored.ConfigDirtyAt != 0 {
				t.Fatalf("enable and dirty state did not roll back: %+v", stored)
			}
		})
	}
	data, err := json.Marshal(AgentApplyResult{Applied: true, Status: "applied", Generation: 123})
	if err != nil || string(data) != `{"applied":true,"status":"applied"}` {
		t.Fatalf("result JSON = %s, err=%v", data, err)
	}
}

func TestMarkNodeDirtyAdvancesGeneration(t *testing.T) {
	setupSettingTestDB(t)
	n := &model.Node{Name: "agent", NodeType: model.NodeTypeAgent, Address: "127.0.0.1", Port: 9443, ApiToken: "secret", Enable: true}
	if err := (&NodeService{}).Create(n); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Hour).UnixMilli()
	if err := database.GetDB().Model(n).Updates(map[string]any{"config_dirty": true, "config_dirty_at": future}).Error; err != nil {
		t.Fatal(err)
	}
	if err := (&NodeService{}).MarkNodeDirty(n.Id); err != nil {
		t.Fatal(err)
	}
	if err := (&NodeService{}).ClearNodeDirty(n.Id, future); err != nil {
		t.Fatal(err)
	}
	var stored model.Node
	if err := database.GetDB().First(&stored, n.Id).Error; err != nil {
		t.Fatal(err)
	}
	if !stored.ConfigDirty || stored.ConfigDirtyAt <= future {
		t.Fatalf("new dirty generation lost: %+v", stored)
	}
}
