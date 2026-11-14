package main

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestRealXrayActivationRollback(t *testing.T) {
	binary := os.Getenv("XRAY_E2E_BINARY")
	if binary == "" {
		t.Skip("set XRAY_E2E_BINARY")
	}
	a := &agent{token: "secret", target: filepath.Join(t.TempDir(), "config.json"), xray: binary}
	defer a.shutdown()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	config := func(p int) string {
		return fmt.Sprintf(`{"config":{"inbounds":[{"listen":"127.0.0.1","port":%d,"protocol":"socks"}],"outbounds":[{"protocol":"freedom"}]}}`, p)
	}
	send := func(body string) int {
		r := httptest.NewRequest(http.MethodPost, "/v1/config", bytes.NewBufferString(body))
		r.Header.Set("Authorization", "Bearer secret")
		w := httptest.NewRecorder()
		a.serveHTTP(w, r)
		return w.Code
	}
	if code := send(config(port)); code != 200 {
		t.Fatalf("activation: %d", code)
	}
	check := func() {
		t.Helper()
		c, e := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
		if e != nil {
			t.Fatalf("Xray not listening: %v", e)
		}
		c.Close()
	}
	check()
	before, err := os.ReadFile(a.target)
	if err != nil {
		t.Fatal(err)
	}
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	if code := send(config(occupied.Addr().(*net.TCPAddr).Port)); code == 200 {
		t.Fatal("collision accepted")
	}
	after, err := os.ReadFile(a.target)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("rollback lost config: %v", err)
	}
	check()
}

func TestRealXrayDisableLifecycle(t *testing.T) {
	binary := os.Getenv("XRAY_E2E_BINARY")
	if binary == "" {
		t.Skip("set XRAY_E2E_BINARY")
	}
	a := &agent{token: "secret", target: filepath.Join(t.TempDir(), "config.json"), xray: binary}
	defer a.shutdown()
	send := func(path, body, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", path, bytes.NewBufferString(body))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		capabilityHandler(a).ServeHTTP(w, r)
		return w
	}
	config := `{"config":{"outbounds":[{"protocol":"freedom"}]}}`
	if w := send("/v1/config", config, "secret"); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if w := send("/v1/disable", `{}`, "wrong"); w.Code != 401 {
		t.Fatal(w.Code)
	}
	if w := send("/v1/disable", `{"unexpected":true}`, "secret"); w.Code != 400 {
		t.Fatal(w.Code)
	}
	if !a.running() {
		t.Fatal("rejected disable stopped Xray")
	}
	if w := send("/v1/disable", `{}`, "secret"); w.Code != 200 || !bytes.Contains(w.Body.Bytes(), []byte(`"status":"disabled"`)) {
		t.Fatal(w.Code, w.Body)
	}
	if a.running() {
		t.Fatal("Xray still running")
	}
	if err := a.startup(); err != nil || a.running() {
		t.Fatalf("disabled startup: %v", err)
	}
	previous, err := os.ReadFile(a.target)
	if err != nil {
		t.Fatal(err)
	}
	if w := send("/v1/config", `{"config":{"outbounds":[{"protocol":"not-a-protocol"}]}}`, "secret"); w.Code != 422 {
		t.Fatalf("invalid re-enable config: %d", w.Code)
	}
	if a.running() {
		t.Fatal("invalid config restarted Xray")
	}
	current, err := os.ReadFile(a.target)
	if err != nil || !bytes.Equal(previous, current) {
		t.Fatalf("failed config changed previous config: %v", err)
	}
	if _, err := os.Stat(a.target + ".disabled"); err != nil {
		t.Fatalf("failed config cleared disabled state: %v", err)
	}
	if w := send("/v1/disable", `{}`, "secret"); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w := send("/v1/config", config, "secret"); w.Code != 200 || !a.running() {
		t.Fatal(w.Code, w.Body)
	}
}

func TestRealXrayConcurrentApplies(t *testing.T) {
	binary := os.Getenv("XRAY_E2E_BINARY")
	if binary == "" {
		t.Skip("set XRAY_E2E_BINARY")
	}
	a := &agent{token: "secret", target: filepath.Join(t.TempDir(), "config.json"), xray: binary}
	defer a.shutdown()
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := httptest.NewRequest("POST", "/v1/config", bytes.NewBufferString(`{"config":{"outbounds":[{"protocol":"freedom"}]}}`))
			r.Header.Set("Authorization", "Bearer secret")
			w := httptest.NewRecorder()
			a.serveHTTP(w, r)
			if w.Code != 200 {
				t.Errorf("apply: %d", w.Code)
			}
		}()
	}
	wg.Wait()
	a.mu.Lock()
	running := a.running()
	a.mu.Unlock()
	if !running {
		t.Fatal("no managed child")
	}
}

func TestRealXrayStartupExitAndShutdown(t *testing.T) {
	binary := os.Getenv("XRAY_E2E_BINARY")
	if binary == "" {
		t.Skip("set XRAY_E2E_BINARY")
	}
	target := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(target, []byte(`{"outbounds":[{"protocol":"freedom"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	a := &agent{target: target, xray: binary, token: "secret"}
	defer a.shutdown()
	if err := a.startup(); err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	pid := a.child.Process.Pid
	err := a.child.Process.Kill()
	a.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		r := httptest.NewRequest("GET", "/v1/status", nil)
		r.Header.Set("Authorization", "Bearer secret")
		w := httptest.NewRecorder()
		a.serveHTTP(w, r)
		if bytes.Contains(w.Body.Bytes(), []byte(`"xray_status":"stopped"`)) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("exit not reflected in status")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := a.startup(); err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	child := a.child.Process
	a.mu.Unlock()
	if child.Pid == pid {
		t.Fatal("process not restarted")
	}
	a.shutdown()
	if err := child.Signal(os.Interrupt); err == nil {
		t.Fatal("child survived shutdown")
	}
}
