package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStartupFenceRecovery(t *testing.T) {
	config := `{"outbounds":[{"protocol":"freedom"}]}`
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(config), &obj); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := requestFingerprint("config", obj)
	if err != nil {
		t.Fatal(err)
	}
	disableFingerprint, err := requestFingerprint("disable", struct{}{})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name        string
		fence       configFence
		data        string
		wantRunning bool
	}{
		{"pending disable", configFence{1, "disable", disableFingerprint, false}, config, false},
		{"pending config", configFence{1, "config", fingerprint, false}, config, false},
		{"applied disable missing marker", configFence{1, "disable", disableFingerprint, true}, config, false},
		{"applied config mismatched backup", configFence{1, "config", fingerprint, true}, `{"outbounds":[]}`, false},
		{"applied config matching", configFence{1, "config", fingerprint, true}, config, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			target := filepath.Join(dir, "config.json")
			xray := filepath.Join(dir, "xray")
			if err := os.WriteFile(xray, []byte("#!/bin/sh\nif [ \"$2\" = -test ]; then exit 0; fi\nexec sleep 30\n"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(target, []byte(tc.data), 0600); err != nil {
				t.Fatal(err)
			}
			a := &agent{target: target, xray: xray}
			if err := a.saveFence(tc.fence); err != nil {
				t.Fatal(err)
			}
			b := &agent{target: target, xray: xray}
			defer b.shutdown()
			if err := b.startup(); err != nil {
				t.Fatal(err)
			}
			if got := b.running(); got != tc.wantRunning {
				t.Fatalf("running = %v, want %v", got, tc.wantRunning)
			}
		})
	}
}

func TestStandaloneHTTPOptIn(t *testing.T) {
	xray := os.Getenv("XRAY_E2E_BINARY")
	if xray == "" {
		t.Skip("set XRAY_E2E_BINARY")
	}
	dir := t.TempDir()
	binary := filepath.Join(dir, "agent")
	if out, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	token := strings.Repeat("a", 32)
	tokenPath := filepath.Join(dir, "token")
	if err := os.WriteFile(tokenPath, []byte(token), 0600); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	listener.Close()
	args := []string{"-listen", addr, "-token-file", tokenPath, "-identity-file", filepath.Join(dir, "identity"), "-config-file", filepath.Join(dir, "config"), "-xray", xray}
	if err := exec.Command(binary, args...).Run(); err == nil {
		t.Fatal("started without TLS or explicit HTTP opt-in")
	}
	cmd := exec.Command(binary, append(args, "-allow-http")...)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Signal(os.Interrupt); _ = cmd.Wait() })
	client := &http.Client{Timeout: time.Second}
	var response *http.Response
	for i := 0; i < 100; i++ {
		req, _ := http.NewRequest(http.MethodGet, "http://"+addr+"/v1/status", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		response, err = client.Do(req)
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("HTTP listener did not start: %v", err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), `"status":"ready"`) {
		t.Fatalf("HTTP status: %d %s", response.StatusCode, body)
	}
	req, _ := http.NewRequest(http.MethodPost, "http://"+addr+"/v1/config", strings.NewReader(`{"config":{"inbounds":[]}}`))
	req.Header.Set("Authorization", "Bearer wrong")
	response, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong token: %d", response.StatusCode)
	}
	req, _ = http.NewRequest(http.MethodPost, "http://"+addr+"/v1/config", strings.NewReader(`{"config":{"outbounds":[{"protocol":"freedom"}]}}`))
	req.Header.Set("Authorization", "Bearer "+token)
	response, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("config status: %d", response.StatusCode)
	}
	config, err := os.ReadFile(filepath.Join(dir, "config"))
	if err != nil || string(config) != `{"outbounds":[{"protocol":"freedom"}]}` {
		t.Fatalf("saved config: %s %v", config, err)
	}
}

func TestIdentityPersists(t *testing.T) {
	p := filepath.Join(t.TempDir(), "identity")
	a, err := identity(p)
	if err != nil {
		t.Fatal(err)
	}
	b, err := identity(p)
	if err != nil || a != b {
		t.Fatalf("identity not persistent: %q %q %v", a, b, err)
	}
	s, err := os.Stat(p)
	if err != nil || s.Mode().Perm() != 0600 {
		t.Fatalf("identity permissions: %v %v", s, err)
	}
}

func TestApplyRejectsInvalidAndPreservesPrevious(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "config.json")
	if err := os.WriteFile(target, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	a := &agent{token: "secret", target: target, xray: "/bin/false", id: "node"}
	for _, tc := range []struct {
		token, body string
		status      int
	}{
		{"wrong", `{"config":{}}`, http.StatusUnauthorized},
		{"secret", `{"config":{},"unknown":true}`, http.StatusBadRequest},
		{"secret", `{"config":{}}`, http.StatusUnprocessableEntity},
	} {
		r := httptest.NewRequest(http.MethodPost, "/v1/config", bytes.NewBufferString(tc.body))
		r.Header.Set("Authorization", "Bearer "+tc.token)
		w := httptest.NewRecorder()
		a.serveHTTP(w, r)
		if w.Code != tc.status {
			t.Fatalf("status %d, expected %d: %s", w.Code, tc.status, w.Body.String())
		}
		b, err := os.ReadFile(target)
		if err != nil || string(b) != "old" {
			t.Fatalf("config changed: %q %v", b, err)
		}
	}
}

func TestConfigVersionFencingRejectsReplayAndPersistsAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	xray := filepath.Join(dir, "xray")
	if err := os.WriteFile(xray, []byte("#!/bin/sh\nif [ \"$2\" = -test ]; then exit 0; fi\nsleep 30\n"), 0700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "config.json")
	send := func(a *agent, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
		r.Header.Set("Authorization", "Bearer secret")
		w := httptest.NewRecorder()
		a.serveHTTP(w, r)
		return w
	}
	a := &agent{token: "secret", target: target, xray: xray, id: "node"}
	defer a.shutdown()
	config := `{"config_version":7,"config":{"outbounds":[{"protocol":"freedom"}]}}`
	if w := send(a, "/v1/config", config); w.Code != http.StatusOK {
		t.Fatalf("apply v7: %d %s", w.Code, w.Body)
	}
	if w := send(a, "/v1/config", config); w.Code != http.StatusOK {
		t.Fatalf("same request retry: %d %s", w.Code, w.Body)
	}
	if w := send(a, "/v1/disable", `{"config_version":7}`); w.Code != http.StatusConflict {
		t.Fatalf("same version different operation: %d", w.Code)
	}
	if w := send(a, "/v1/config", `{"config_version":6,"config":{"outbounds":[{"protocol":"freedom"}]}}`); w.Code != http.StatusConflict {
		t.Fatalf("stale config: %d", w.Code)
	}
	a.shutdown()
	b := &agent{token: "secret", target: target, xray: xray, id: "node"}
	if err := b.startup(); err != nil {
		t.Fatal(err)
	}
	defer b.shutdown()
	if w := send(b, "/v1/config", config); w.Code != http.StatusOK {
		t.Fatalf("post-restart replay: %d %s", w.Code, w.Body)
	}
	if w := send(b, "/v1/config", `{"config_version":6,"config":{"outbounds":[{"protocol":"freedom"}]}}`); w.Code != http.StatusConflict {
		t.Fatalf("post-restart stale config: %d", w.Code)
	}
}

func TestApplyValidConfig(t *testing.T) {
	xray := os.Getenv("XRAY_E2E_BINARY")
	if xray == "" {
		t.Skip("set XRAY_E2E_BINARY")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "config.json")
	a := &agent{token: "secret", target: target, xray: xray, id: "node"}
	defer a.shutdown()
	r := httptest.NewRequest(http.MethodPost, "/v1/config", bytes.NewBufferString(`{"config":{"outbounds":[{"protocol":"freedom"}]}}`))
	r.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()
	a.serveHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	b, err := os.ReadFile(target)
	if err != nil || string(b) != `{"outbounds":[{"protocol":"freedom"}]}` {
		t.Fatalf("config: %q %v", b, err)
	}
	s, err := os.Stat(target)
	if err != nil || s.Mode().Perm() != 0600 {
		t.Fatalf("config permissions: %v %v", s, err)
	}
}

func TestRealXrayTLSValidation(t *testing.T) {
	binary := os.Getenv("XRAY_E2E_BINARY")
	if binary == "" {
		t.Skip("set XRAY_E2E_BINARY to test a real Xray executable")
	}
	target := filepath.Join(t.TempDir(), "config.json")
	a := &agent{token: "secret", target: target, xray: binary, id: "node"}
	defer a.shutdown()
	server := httptest.NewTLSServer(http.HandlerFunc(a.serveHTTP))
	defer server.Close()
	send := func(body string) int {
		req, err := http.NewRequest(http.MethodPost, server.URL+"/v1/config", bytes.NewBufferString(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer secret")
		resp, err := server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		return resp.StatusCode
	}
	valid := `{"config":{"outbounds":[{"protocol":"freedom"}]}}`
	if status := send(valid); status != http.StatusOK {
		t.Fatalf("valid config: %d", status)
	}
	before, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if status := send(`{"config":{"outbounds":[{"protocol":"not-a-protocol"}]}}`); status != http.StatusUnprocessableEntity {
		t.Fatalf("invalid config: %d", status)
	}
	after, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("previous config changed: %v", err)
	}
	if status := send(`{"config":{"unsupported":{}}}`); status != http.StatusBadRequest {
		t.Fatalf("unsupported config: %d", status)
	}
}

func TestPrivateFileRejectsPublicPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(path, []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := readPrivate(path); err == nil {
		t.Fatal("accepted public secret")
	}
}
