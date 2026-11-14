package controller

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

func newAgentConfigControllerTest(t *testing.T, handler http.Handler) (*gin.Engine, *model.Node) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	dbtest.InitDB(t, filepath.Join(dbDir, "x-ui.db"))

	remote := httptest.NewServer(handler)
	t.Cleanup(remote.Close)
	host, portString, err := net.SplitHostPort(strings.TrimPrefix(remote.URL, "http://"))
	if err != nil {
		t.Fatalf("parse remote address %q: %v", remote.URL, err)
	}
	var port int
	if _, err := fmt.Sscanf(portString, "%d", &port); err != nil {
		t.Fatalf("parse remote port %q: %v", portString, err)
	}
	node := &model.Node{
		Name:                "agent",
		NodeType:            model.NodeTypeAgent,
		Scheme:              "http",
		Address:             host,
		Port:                port,
		BasePath:            "/",
		ApiToken:            "agent-secret",
		Enable:              true,
		AllowPrivateAddress: true,
	}
	if err := (&service.NodeService{}).Create(node); err != nil {
		t.Fatalf("seed node: %v", err)
	}

	engine := gin.New()
	NewNodeController(engine.Group("/panel/api/nodes"))
	return engine, node
}

func agentReplyHandler(delay time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer agent-secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if delay > 0 {
			time.Sleep(delay)
		}
		switch r.URL.Path {
		case "/v1/status":
			_, _ = w.Write([]byte(`{"id":"agent","status":"ready"}`))
		case "/v1/capabilities":
			_, _ = w.Write([]byte(`{"version":1,"config_save":true,"config_apply":true,"config_disable":true,"runtime_health":false}`))
		case "/v1/config":
			_, _ = w.Write([]byte(`{"status":"applied"}`))
		case "/v1/disable":
			_, _ = w.Write([]byte(`{"status":"disabled"}`))
		default:
			http.NotFound(w, r)
		}
	})
}

func decodeAgentApplyEnvelope(t *testing.T, body string) map[string]any {
	t.Helper()
	var envelope struct {
		Obj map[string]any `json:"obj"`
	}
	if err := json.Unmarshal([]byte(body), &envelope); err != nil {
		t.Fatalf("decode response %q: %v", body, err)
	}
	return envelope.Obj
}

func TestNodeControllerApplyAgentConfigReturnsTruthfulStatus(t *testing.T) {
	engine, node := newAgentConfigControllerTest(t, agentReplyHandler(0))

	inbound := &model.Inbound{
		Tag:      "agent-inbound",
		NodeID:   &node.Id,
		Enable:   true,
		Port:     443,
		Protocol: model.VLESS,
		Settings: `{"clients":[],"decryption":"none"}`,
	}
	if err := database.GetDB().Create(inbound).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}

	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodPost, fmt.Sprintf("/panel/api/nodes/applyAgentConfig/%d", node.Id), nil))
	if w.Code != http.StatusOK {
		t.Fatalf("applied status = %d body=%s", w.Code, w.Body.String())
	}
	if got := decodeAgentApplyEnvelope(t, w.Body.String())["status"]; got != "applied" {
		t.Fatalf("status = %v, want applied; body=%s", got, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "agent-secret") {
		t.Fatalf("response leaked agent secret: %s", w.Body.String())
	}

	if err := database.GetDB().Model(&model.Inbound{}).Where("id = ?", inbound.Id).Update("enable", false).Error; err != nil {
		t.Fatalf("disable inbound: %v", err)
	}
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodPost, fmt.Sprintf("/panel/api/nodes/applyAgentConfig/%d", node.Id), nil))
	if got := decodeAgentApplyEnvelope(t, w.Body.String())["status"]; got != "disabled" {
		t.Fatalf("status = %v, want disabled; body=%s", got, w.Body.String())
	}
}

func TestNodeControllerApplyAgentConfigClassifiesFailuresAndTimeouts(t *testing.T) {
	engine, node := newAgentConfigControllerTest(t, agentReplyHandler(0))
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodPost, fmt.Sprintf("/panel/api/nodes/applyAgentConfig/%d", node.Id), nil))
	if got := decodeAgentApplyEnvelope(t, w.Body.String())["status"]; got != "disabled" {
		t.Fatalf("empty scope status = %v, want disabled; body=%s", got, w.Body.String())
	}

	// An unknown node is still a structured failure and must not expose service
	// internals such as credentials in the response.
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/panel/api/nodes/applyAgentConfig/9999", nil))
	obj := decodeAgentApplyEnvelope(t, w.Body.String())
	if obj["status"] != "failed" || strings.Contains(w.Body.String(), "agent-secret") {
		t.Fatalf("unknown node response = %s", w.Body.String())
	}

	oldTimeout := agentApplyTimeout
	agentApplyTimeout = 10 * time.Millisecond
	t.Cleanup(func() { agentApplyTimeout = oldTimeout })
	// The timeout path is exercised through the real Gin route while keeping
	// the test fast; production keeps the timeout at 30 seconds.
	engine, node = newAgentConfigControllerTest(t, agentReplyHandler(100*time.Millisecond))
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodPost, fmt.Sprintf("/panel/api/nodes/applyAgentConfig/%d", node.Id), nil))
	obj = decodeAgentApplyEnvelope(t, w.Body.String())
	if obj["status"] != "unknown" || obj["reason"] != "timeout" {
		t.Fatalf("timeout response = %s", w.Body.String())
	}
}

func TestNodeControllerApplyAgentConfigLostResponseIsUnknown(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/disable" {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			conn.Close()
			return
		}
		agentReplyHandler(0).ServeHTTP(w, r)
	})
	engine, node := newAgentConfigControllerTest(t, handler)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodPost, fmt.Sprintf("/panel/api/nodes/applyAgentConfig/%d", node.Id), nil))
	if obj := decodeAgentApplyEnvelope(t, w.Body.String()); obj["status"] != "unknown" {
		t.Fatalf("lost response must be unknown: %s", w.Body.String())
	}
}

func TestNodeControllerDisablingAgentRevokesRemotelyAndKeepsRetryState(t *testing.T) {
	var failDisable bool
	var disableCalls int
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/disable" {
			disableCalls++
			if failDisable {
				http.Error(w, "offline", http.StatusBadGateway)
				return
			}
			_, _ = w.Write([]byte(`{"status":"disabled"}`))
			return
		}
		agentReplyHandler(0).ServeHTTP(w, r)
	})
	engine, node := newAgentConfigControllerTest(t, handler)
	inbound := &model.Inbound{
		Tag: "still-running", NodeID: &node.Id, Enable: true, Port: 443,
		Protocol: model.VLESS, Settings: `{"clients":[],"decryption":"none"}`,
	}
	if err := database.GetDB().Create(inbound).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}

	failDisable = true
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodPost,
		fmt.Sprintf("/panel/api/nodes/setEnable/%d", node.Id),
		strings.NewReader(`{"enable":false}`)))
	obj := decodeAgentApplyEnvelope(t, w.Body.String())
	var envelope struct {
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode failure response: %v", err)
	}
	if envelope.Success || obj["status"] != "unknown" {
		t.Fatalf("failed revoke was reported successful: %s", w.Body.String())
	}
	var stored model.Node
	if err := database.GetDB().First(&stored, node.Id).Error; err != nil {
		t.Fatalf("read failed revoke state: %v", err)
	}
	if stored.Enable || !stored.ConfigDirty {
		t.Fatalf("failed revoke must remain locally disabled and dirty: %+v", stored)
	}

	failDisable = false
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodPost,
		fmt.Sprintf("/panel/api/nodes/setEnable/%d", node.Id),
		strings.NewReader(`{"enable":false}`)))
	obj = decodeAgentApplyEnvelope(t, w.Body.String())
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode recovery response: %v", err)
	}
	if !envelope.Success || obj["status"] != "disabled" {
		t.Fatalf("recovered revoke response = %s", w.Body.String())
	}
	if disableCalls != 2 {
		t.Fatalf("disable calls = %d, want 2", disableCalls)
	}
	if err := database.GetDB().First(&stored, node.Id).Error; err != nil {
		t.Fatalf("read recovered state: %v", err)
	}
	if stored.ConfigDirty {
		t.Fatalf("successful revoke must clear retry state: %+v", stored)
	}
}
