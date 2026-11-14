package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

func TestAgentProbeRuntimeHealth(t *testing.T) {
	setupConflictDB(t)
	for _, tc := range []struct {
		name, field, want string
		advertised        bool
	}{
		{"running", `,"xray_status":"running"`, "running", true},
		{"stopped", `,"xray_status":"stopped"`, "stopped", true},
		{"absent", ``, "", true},
		{"invalid", `,"xray_status":"unknown"`, "", true},
		{"wrong-type", `,"xray_status":42`, "", true},
		{"not-advertised", `,"xray_status":"running"`, "running", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v1/status" {
					fmt.Fprintf(w, `{"id":"agent","status":"ready"%s}`, tc.field)
				} else {
					fmt.Fprintf(w, `{"version":1,"config_save":true,"config_apply":true,"runtime_health":%t}`, tc.advertised)
				}
			}))
			defer ts.Close()
			host, port, _ := net.SplitHostPort(ts.Listener.Addr().String())
			p, _ := strconv.Atoi(port)
			n := &model.Node{Name: tc.name, NodeType: model.NodeTypeAgent, Scheme: "http", Address: host, Port: p, ApiToken: "secret", AllowPrivateAddress: true}
			s := &NodeService{}
			if err := s.Create(n); err != nil {
				t.Fatal(err)
			}
			patch, err := s.Probe(context.Background(), n)
			if err != nil {
				t.Fatal(err)
			}
			if patch.XrayState != tc.want || patch.ToUI(true).XrayState != tc.want {
				t.Fatalf("state: %+v", patch)
			}
			if patch.AgentCapabilities == nil || patch.AgentCapabilities.RuntimeHealth != (tc.advertised && tc.want != "") {
				t.Fatalf("capabilities: %+v", patch.AgentCapabilities)
			}
			for _, metric := range []string{"cpu", "mem", "netUp", "netDown", "uptime"} {
				if !slices.Contains(patch.UnavailableMetrics, metric) {
					t.Fatalf("missing unavailable metric %s", metric)
				}
			}
			patch.Status = "online"
			if err := s.UpdateHeartbeat(n.Id, patch); err != nil {
				t.Fatal(err)
			}
			view, err := s.GetViewById(n.Id)
			if err != nil || view.XrayState != tc.want {
				t.Fatalf("view: %+v, error: %v", view, err)
			}
		})
	}
}

func TestAgentDiscoveryAndPrivatePolicyRevocation(t *testing.T) {
	for _, tlsEnabled := range []bool{false, true} {
		name := "http"
		if tlsEnabled {
			name = "https"
		}
		t.Run(name, func(t *testing.T) {
			var calls atomic.Int32
			h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Header.Get("Authorization") != "Bearer secret" {
					http.Error(w, "unauthorized", 401)
					return
				}
				switch r.URL.Path {
				case "/v1/status":
					fmt.Fprint(w, `{"id":"agent-id","status":"ready"}`)
				case "/v1/capabilities":
					fmt.Fprint(w, `{"version":1,"config_save":true,"config_apply":false,"runtime_health":false}`)
				default:
					http.NotFound(w, r)
				}
			})
			var ts *httptest.Server
			if tlsEnabled {
				ts = httptest.NewTLSServer(h)
			} else {
				ts = httptest.NewServer(h)
			}
			defer ts.Close()
			host, port, _ := net.SplitHostPort(ts.Listener.Addr().String())
			p, _ := strconv.Atoi(port)
			n := &model.Node{NodeType: model.NodeTypeAgent, Scheme: name, Address: host, Port: p, ApiToken: "secret", AllowPrivateAddress: true}
			if tlsEnabled {
				pin := sha256.Sum256(ts.Certificate().Raw)
				n.TlsVerifyMode = "pin"
				n.PinnedCertSha256 = base64.StdEncoding.EncodeToString(pin[:])
			}
			patch, err := (&NodeService{}).Probe(context.Background(), n)
			if err != nil {
				t.Fatal(err)
			}
			if patch.AgentCapabilities == nil || !patch.AgentCapabilities.ConfigSave || patch.AgentCapabilities.ConfigApply || patch.AgentCapabilities.RuntimeHealth {
				t.Fatalf("capabilities: %+v", patch.AgentCapabilities)
			}
			before := calls.Load()
			n.AllowPrivateAddress = false
			if _, err := (&NodeService{}).Probe(context.Background(), n); err == nil {
				t.Fatal("private address accepted after revocation")
			}
			if calls.Load() != before {
				t.Fatal("request reached private server after revocation")
			}
		})
	}
}

func TestAgentProbeTLSAndAuthorization(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/base/v1/status" || r.Header.Get("Authorization") != "Bearer secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		fmt.Fprint(w, `{"id":"agent-id","status":"ready"}`)
	}))
	defer ts.Close()
	host, port, _ := net.SplitHostPort(ts.Listener.Addr().String())
	p, _ := strconv.Atoi(port)
	pin := sha256.Sum256(ts.Certificate().Raw)
	n := &model.Node{NodeType: model.NodeTypeAgent, Scheme: "https", Address: host, Port: p, BasePath: "/base", ApiToken: "secret", AllowPrivateAddress: true, TlsVerifyMode: "pin", PinnedCertSha256: base64.StdEncoding.EncodeToString(pin[:])}
	patch, err := (&NodeService{}).Probe(context.Background(), n)
	if err != nil {
		t.Fatal(err)
	}
	if patch.Guid != "" || patch.XrayState != "" || patch.PanelVersion != "" || len(patch.UnavailableMetrics) < 4 {
		t.Fatalf("agent fabricated status: %+v", patch)
	}
	n.TlsVerifyMode = "verify"
	if _, err := (&NodeService{}).Probe(context.Background(), n); err == nil {
		t.Fatal("untrusted certificate accepted")
	}
	n.TlsVerifyMode = "pin"
	n.ApiToken = "wrong"
	if _, err := (&NodeService{}).Probe(context.Background(), n); err == nil {
		t.Fatal("wrong token accepted")
	}
	n.ApiToken = "secret"
	n.PinnedCertSha256 = base64.StdEncoding.EncodeToString(make([]byte, 32))
	if _, err := (&NodeService{}).Probe(context.Background(), n); err == nil {
		t.Fatal("wrong pin accepted")
	}
	n.PinnedCertSha256 = base64.StdEncoding.EncodeToString(pin[:])
	n.AllowPrivateAddress = false
	if _, err := (&NodeService{}).Probe(context.Background(), n); err == nil {
		t.Fatal("private address accepted")
	}
}

func TestAgentRejectsUnsupportedOperations(t *testing.T) {
	setupConflictDB(t)
	n := &model.Node{Name: "agent", NodeType: model.NodeTypeAgent, Scheme: "https", Address: "127.0.0.1", Port: 9443, ApiToken: "secret", Enable: true}
	s := &NodeService{}
	if err := s.Create(n); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.NewManager(runtime.LocalDeps{}).RemoteFor(n); err == nil {
		t.Fatal("agent treated as panel runtime")
	}
	if err := s.MarkNodeDirty(n.Id); err != nil {
		t.Fatalf("mark agent dirty: %v", err)
	}
	var stored model.Node
	if err := database.GetDB().First(&stored, n.Id).Error; err != nil {
		t.Fatal(err)
	}
	if !stored.ConfigDirty || stored.ConfigDirtyAt <= 0 {
		t.Fatal("agent not marked dirty")
	}
	if _, err := s.GetRemoteInboundOptions(context.Background(), n); err == nil {
		t.Fatal("agent inbound listing accepted")
	}
	if _, err := runtime.NewRemote(n, nil).FetchTrafficSnapshot(context.Background()); err == nil {
		t.Fatal("direct agent panel request accepted")
	}
	if _, err := (&InboundService{}).SetRemoteTraffic(n.Id, &runtime.TrafficSnapshot{}, false, false); err == nil {
		t.Fatal("agent traffic snapshot accepted")
	}
	update := *n
	update.NodeType = model.NodeTypePanel
	if err := s.Update(n.Id, &update); err == nil {
		t.Fatal("node type transition accepted")
	}
}

func TestAgentEncryptedTokenAndMetricHistory(t *testing.T) {
	setupConflictDB(t)
	enableNodeTokenEncryption(t)
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			http.Error(w, "unauthorized", 401)
			return
		}
		fmt.Fprint(w, `{"id":"agent-id","status":"ready"}`)
	}))
	defer ts.Close()
	host, port, _ := net.SplitHostPort(ts.Listener.Addr().String())
	p, _ := strconv.Atoi(port)
	n := &model.Node{Name: "encrypted-agent", NodeType: model.NodeTypeAgent, Scheme: "https", Address: host, Port: p, ApiToken: "secret", AllowPrivateAddress: true, TlsVerifyMode: "skip"}
	s := &NodeService{}
	if err := s.Create(n); err != nil {
		t.Fatal(err)
	}
	var stored model.Node
	if err := database.GetDB().First(&stored, n.Id).Error; err != nil {
		t.Fatal(err)
	}
	if stored.ApiToken == "secret" {
		t.Fatal("token not encrypted")
	}
	patch, err := s.Probe(context.Background(), &stored)
	if err != nil {
		t.Fatal(err)
	}
	old := nodeMetrics
	nodeMetrics = newMetricHistory()
	t.Cleanup(func() { nodeMetrics = old })
	patch.Status = "online"
	if err := s.UpdateHeartbeat(n.Id, patch); err != nil {
		t.Fatal(err)
	}
	for _, metric := range []string{"cpu", "mem", "netUp", "netDown"} {
		if points := s.AggregateNodeMetric(n.Id, metric, 2, 60); len(points) != 0 {
			t.Fatalf("fake %s history: %v", metric, points)
		}
	}
}

func TestAgentCreationRequiresToken(t *testing.T) {
	setupConflictDB(t)
	for _, n := range []*model.Node{

		{Name: "no-token", NodeType: model.NodeTypeAgent, Scheme: "https", Address: "127.0.0.1", Port: 9443, TlsVerifyMode: "mtls"},
	} {
		if err := (&NodeService{}).Create(n); err == nil {
			t.Fatalf("accepted %s", n.Name)
		}
	}
}

func TestAgentHTTPCreateAndProbe(t *testing.T) {
	setupConflictDB(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/status" || r.Header.Get("Authorization") != "Bearer secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		fmt.Fprint(w, `{"id":"agent-id","status":"ready"}`)
	}))
	defer ts.Close()
	host, port, _ := net.SplitHostPort(ts.Listener.Addr().String())
	p, _ := strconv.Atoi(port)
	n := &model.Node{Name: "http-agent", NodeType: model.NodeTypeAgent, Scheme: "http", Address: host, Port: p, ApiToken: "secret", AllowPrivateAddress: true}
	s := &NodeService{}
	if err := s.Create(n); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Probe(context.Background(), n); err != nil {
		t.Fatal(err)
	}
	n.ApiToken = "wrong"
	if _, err := s.Probe(context.Background(), n); err == nil {
		t.Fatal("wrong token accepted")
	}
}

func TestAgentRejectsInvalidStatus(t *testing.T) {
	for _, body := range []string{`{}`, `{"id":"x","status":"bad"}`, `not-json`, strings.Repeat("x", maxProbeBodyBytes+1)} {
		ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
		host, port, _ := net.SplitHostPort(ts.Listener.Addr().String())
		p, _ := strconv.Atoi(port)
		n := &model.Node{NodeType: model.NodeTypeAgent, Scheme: "https", Address: host, Port: p, ApiToken: "secret", AllowPrivateAddress: true, TlsVerifyMode: "skip"}
		if _, err := (&NodeService{}).Probe(context.Background(), n); err == nil {
			t.Errorf("accepted invalid body %.40q", body)
		}
		ts.Close()
	}
}
