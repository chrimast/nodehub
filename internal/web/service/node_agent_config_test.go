package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestAgentConfigPushScopedAndGated(t *testing.T) {
	setupSettingTestDB(t)
	var applied bool
	reply := `{"status":"applied"}`
	var body struct {
		ConfigVersion int64 `json:"config_version"`
		Config struct {
			Inbounds []struct {
				Tag      string `json:"tag"`
				Settings struct {
					Clients []struct {
						Email string `json:"email"`
					} `json:"clients"`
				} `json:"settings"`
			} `json:"inbounds"`
		} `json:"config"`
	}
	disabled := false
	_ = disabled
	enabled := false
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			http.Error(w, "unauthorized", 401)
			return
		}
		switch r.URL.Path {
		case "/base/v1/status":
			fmt.Fprint(w, `{"id":"a","status":"ready"}`)
		case "/base/v1/capabilities":
			fmt.Fprintf(w, `{"version":1,"config_save":true,"config_apply":%t,"config_disable":%t,"config_version":true,"runtime_health":false}`, enabled, enabled)
		case "/base/v1/config":
			applied = true
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body.ConfigVersion <= 0 {
				t.Errorf("config_version = %d, want positive", body.ConfigVersion)
			}
			fmt.Fprint(w, reply)
		case "/base/v1/disable":
			var request struct {
				ConfigVersion int64 `json:"config_version"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
			}
			if request.ConfigVersion <= 0 {
				t.Errorf("disable config_version = %d, want positive", request.ConfigVersion)
			}
			disabled = true
			fmt.Fprint(w, `{"status":"disabled"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()
	host, port, _ := net.SplitHostPort(ts.Listener.Addr().String())
	p, _ := strconv.Atoi(port)
	pin := sha256.Sum256(ts.Certificate().Raw)
	n := &model.Node{Name: "agent", NodeType: model.NodeTypeAgent, Scheme: "https", Address: host, Port: p, BasePath: "/base", ApiToken: "secret", AllowPrivateAddress: true, TlsVerifyMode: "pin", PinnedCertSha256: base64.StdEncoding.EncodeToString(pin[:]), Enable: true}
	if err := (&NodeService{}).Create(n); err != nil {
		t.Fatal(err)
	}
	other := n.Id + 100
	for _, in := range []*model.Inbound{
		{Tag: "target", NodeID: &n.Id, Enable: true, Port: 1234, Protocol: model.VLESS, Settings: `{"clients":[],"decryption":"none"}`},
		{Tag: "other", NodeID: &other, Enable: true, Port: 1235, Protocol: model.VLESS, Settings: `{"clients":[]}`},
		{Tag: "disabled", NodeID: &n.Id, Enable: false, Port: 1236, Protocol: model.VLESS, Settings: `{"clients":[]}`},
	} {
		if err := database.GetDB().Create(in).Error; err != nil {
			t.Fatal(err)
		}
		if in.Tag == "target" {
			if err := (&ClientService{}).SyncInbound(nil, in.Id, []model.Client{{Email: "on", ID: "11111111-1111-1111-1111-111111111111", Enable: true}, {Email: "off", ID: "22222222-2222-2222-2222-222222222222", Enable: false}}); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := (&NodeService{}).ApplyAgentConfig(context.Background(), n.Id); err == nil || applied {
		t.Fatal("capability gate failed")
	}
	enabled = true
	result, err := (&NodeService{}).ApplyAgentConfig(context.Background(), n.Id)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || !applied {
		t.Fatal("not applied")
	}
	if _, err := database.GetDB().Model(&model.Inbound{}).Where("node_id = ?", n.Id).Update("enable", false).Rows(); err != nil {
		t.Fatal(err)
	}
	enabled = true
	result, err = (&NodeService{}).ApplyAgentConfig(context.Background(), n.Id)
	if err != nil || !result.Applied || !disabled {
		t.Fatalf("empty scope disable: %+v %v disabled=%v", result, err, disabled)
	}
	if _, err := database.GetDB().Model(&model.Inbound{}).Where("node_id = ?", n.Id).Update("enable", true).Rows(); err != nil {
		t.Fatal(err)
	}
	if len(body.Config.Inbounds) != 1 || body.Config.Inbounds[0].Tag != "target" || len(body.Config.Inbounds[0].Settings.Clients) != 1 || body.Config.Inbounds[0].Settings.Clients[0].Email != "on" {
		t.Fatalf("unexpected config: %+v", body)
	}
	reply = `{"status":"saved"}`
	result, err = (&NodeService{}).ApplyAgentConfig(context.Background(), n.Id)
	if err == nil || result.Applied {
		t.Fatal("unconfirmed application reported as applied")
	}
	if err := database.GetDB().Model(&model.Node{}).Where("id = ?", n.Id).Update("allow_private_address", false).Error; err != nil {
		t.Fatal(err)
	}
	applied = false
	result, err = (&NodeService{}).ApplyAgentConfig(context.Background(), n.Id)
	if err == nil || result.Applied || applied {
		t.Fatal("private-address policy bypassed")
	}
}

func TestAgentConfigRejectsEmptyAndUnsupportedScope(t *testing.T) {
	setupSettingTestDB(t)
	n := &model.Node{Name: "agent", NodeType: model.NodeTypeAgent, Scheme: "https", Address: "127.0.0.1", Port: 9443, ApiToken: "secret", Enable: true}
	if err := (&NodeService{}).Create(n); err != nil {
		t.Fatal(err)
	}
	if _, err := (&XrayService{}).GetAgentXrayConfig(n.Id); err != nil {
		t.Fatalf("empty scope should be represented explicitly: %v", err)
	}
	in := &model.Inbound{Tag: "unsupported", NodeID: &n.Id, Enable: true, Port: 1234, Protocol: model.TUIC, Settings: `{}`}
	if err := database.GetDB().Create(in).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := (&XrayService{}).GetAgentXrayConfig(n.Id); err == nil {
		t.Fatal("unsupported protocol accepted")
	}
}

func TestAgentConfigRejectsUnenforcedClientLimits(t *testing.T) {
	setupSettingTestDB(t)
	n := &model.Node{Name: "agent", NodeType: model.NodeTypeAgent, Scheme: "https", Address: "127.0.0.1", Port: 9443, ApiToken: "secret", Enable: true}
	if err := (&NodeService{}).Create(n); err != nil {
		t.Fatal(err)
	}
	in := &model.Inbound{Tag: "limited", NodeID: &n.Id, Enable: true, Port: 1234, Protocol: model.VLESS, Settings: `{"clients":[]}`}
	if err := database.GetDB().Create(in).Error; err != nil {
		t.Fatal(err)
	}
	if err := (&ClientService{}).SyncInbound(nil, in.Id, []model.Client{{Email: "expiring", ID: "11111111-1111-1111-1111-111111111111", Enable: true, ExpiryTime: 1}}); err != nil {
		t.Fatal(err)
	}
	if _, err := (&XrayService{}).GetAgentXrayConfig(n.Id); err == nil {
		t.Fatal("agent config accepted an expiry it cannot enforce")
	}
}

func TestAgentConfigRejectsClientIPLimits(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		t.Run(fmt.Sprintf("enabled=%t", enabled), func(t *testing.T) {
			setupSettingTestDB(t)
			n := &model.Node{Name: "agent", NodeType: model.NodeTypeAgent, Scheme: "http", Address: "127.0.0.1", Port: 9443, ApiToken: "secret", Enable: true}
			if err := (&NodeService{}).Create(n); err != nil {
				t.Fatal(err)
			}
			in := &model.Inbound{Tag: "ip-limited", NodeID: &n.Id, Enable: true, Port: 1234, Protocol: model.VLESS, Settings: `{"clients":[]}`}
			if err := database.GetDB().Create(in).Error; err != nil {
				t.Fatal(err)
			}
			if err := (&ClientService{}).SyncInbound(nil, in.Id, []model.Client{{Email: "ip-limited", ID: "11111111-1111-1111-1111-111111111111", Enable: enabled, LimitIP: 2}}); err != nil {
				t.Fatal(err)
			}
			if _, err := (&XrayService{}).GetAgentXrayConfig(n.Id); err == nil {
				t.Fatal("agent config accepted an IP limit it cannot enforce")
			}
		})
	}
}

func TestAgentConfigRejectsSharedClientTrafficLimit(t *testing.T) {
	setupSettingTestDB(t)
	n := &model.Node{Name: "agent", NodeType: model.NodeTypeAgent, Scheme: "http", Address: "127.0.0.1", Port: 9443, ApiToken: "secret", Enable: true}
	if err := (&NodeService{}).Create(n); err != nil {
		t.Fatal(err)
	}
	in := &model.Inbound{Tag: "shared", NodeID: &n.Id, Enable: true, Port: 1234, Protocol: model.VLESS, Settings: `{"clients":[]}`}
	if err := database.GetDB().Create(in).Error; err != nil {
		t.Fatal(err)
	}
	if err := (&ClientService{}).SyncInbound(nil, in.Id, []model.Client{{Email: "shared", ID: "11111111-1111-1111-1111-111111111111", Enable: true}}); err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Create(&xray.ClientTraffic{Email: "shared", InboundId: in.Id + 100, Enable: true, Total: 10}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := (&XrayService{}).GetAgentXrayConfig(n.Id); err == nil {
		t.Fatal("shared traffic limit ignored")
	}
}

func TestAgentConfigRejectsDisabledClientWithTrafficLimit(t *testing.T) {
	setupSettingTestDB(t)
	n := &model.Node{Name: "agent", NodeType: model.NodeTypeAgent, Scheme: "http", Address: "127.0.0.1", Port: 9443, ApiToken: "secret", Enable: true}
	if err := (&NodeService{}).Create(n); err != nil {
		t.Fatal(err)
	}
	in := &model.Inbound{Tag: "disabled-limited", NodeID: &n.Id, Enable: true, Port: 1234, Protocol: model.VLESS, Settings: `{"clients":[]}`}
	if err := database.GetDB().Create(in).Error; err != nil {
		t.Fatal(err)
	}
	if err := (&ClientService{}).SyncInbound(nil, in.Id, []model.Client{{Email: "disabled-limited", ID: "11111111-1111-1111-1111-111111111111", Enable: true}}); err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Create(&xray.ClientTraffic{Email: "disabled-limited", InboundId: in.Id, Enable: false, Total: 10}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := (&XrayService{}).GetAgentXrayConfig(n.Id); err == nil {
		t.Fatal("disabled limited client was silently skipped")
	}
}
