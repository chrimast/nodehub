package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

type agent struct {
	token, target, xray, id string
	mu                      sync.Mutex
	child                   *exec.Cmd
	done                    chan error
	closing                 bool
	fenceLoaded             bool
	fence                   configFence
}

// configFence is persisted beside the managed config. Versions are opaque
// non-negative integers; the panel owns allocation and must send them in
// strictly increasing order for each agent. A request fingerprint makes a
// retry of the exact operation idempotent without re-running Xray.
type configFence struct {
	Version     int64  `json:"config_version"`
	Operation   string `json:"operation"`
	Fingerprint string `json:"fingerprint"`
	Applied     bool   `json:"applied"`
}

func (a *agent) fencePath() string { return a.target + ".version" }

func (a *agent) loadFence() error {
	if a.fenceLoaded {
		return nil
	}
	b, err := os.ReadFile(a.fencePath())
	if os.IsNotExist(err) {
		a.fenceLoaded = true
		return nil
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, &a.fence); err != nil || a.fence.Version < 0 || (a.fence.Version > 0 && (a.fence.Operation == "" || a.fence.Fingerprint == "")) {
		return errors.New("invalid config version state")
	}
	a.fenceLoaded = true
	return nil
}

func (a *agent) saveFence(fence configFence) error {
	b, err := json.Marshal(fence)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(a.fencePath()+".tmp", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(b); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(name, a.fencePath()); err != nil {
		return err
	}
	// A successful rename already changes the visible state even when the
	// following directory fsync fails. Never allow this process to go back.
	a.fence = fence
	a.fenceLoaded = true
	if err = syncDirectory(filepath.Dir(a.target)); err != nil {
		return err
	}
	return nil
}

func requestFingerprint(operation string, payload any) (string, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	_, _ = io.WriteString(h, operation)
	_, _ = h.Write([]byte{0})
	_, _ = h.Write(b)
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (a *agent) checkFence(operation string, version int64, fingerprint string) (bool, error) {
	if err := a.loadFence(); err != nil {
		return false, err
	}
	if version <= 0 {
		if a.fence.Version > 0 {
			return false, errors.New("config version required")
		}
		return false, nil
	}
	if a.fence.Version == 0 {
		return false, nil
	}
	if version < a.fence.Version {
		return false, errors.New("stale config version")
	}
	if version == a.fence.Version {
		if a.fence.Operation == operation && a.fence.Fingerprint == fingerprint {
			return a.fence.Applied, nil
		}
		return false, errors.New("config version replay conflict")
	}
	return false, nil
}

func (a *agent) commitFence(operation string, version int64, fingerprint string, applied bool) error {
	if version <= 0 {
		return nil
	}
	return a.saveFence(configFence{Version: version, Operation: operation, Fingerprint: fingerprint, Applied: applied})
}

func identity(path string) (string, error) {
	b, err := readPrivate(path)
	if err == nil {
		if len(b) != 32 {
			return "", errors.New("invalid identity")
		}
		return hex.EncodeToString(b), nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	b = make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(err) {
		return identity(path)
	}
	if err != nil {
		return "", err
	}
	if _, err = f.Write(b); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func readPrivate(path string) ([]byte, error) {
	return readPrivateLimit(path, 4097)
}

func readPrivateLimit(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	s, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !s.Mode().IsRegular() || s.Mode().Perm()&0077 != 0 {
		return nil, errors.New("private file must be regular and inaccessible to group/others")
	}
	return io.ReadAll(io.LimitReader(f, limit))
}

func (a *agent) serveHTTP(w http.ResponseWriter, r *http.Request) {
	expected := sha256.Sum256([]byte("Bearer " + a.token))
	supplied := sha256.Sum256([]byte(r.Header.Get("Authorization")))
	if subtle.ConstantTimeCompare(expected[:], supplied[:]) != 1 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.URL.Path == "/v1/status" && r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		a.mu.Lock()
		state := "stopped"
		if a.running() {
			state = "running"
		}
		a.mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]string{"id": a.id, "status": "ready", "xray_status": state})
		return
	}
	if r.URL.Path == "/v1/disable" && r.Method == http.MethodPost {
		a.disableHTTP(w, r)
		return
	}
	if r.URL.Path != "/v1/config" || r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		Config        json.RawMessage `json:"config"`
		ConfigVersion *int64          `json:"config_version"`
	}
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(&req); err != nil || d.Decode(&struct{}{}) != io.EOF {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(req.Config, &obj); err != nil || obj == nil {
		http.Error(w, "config must be an object", http.StatusBadRequest)
		return
	}
	allowed := map[string]bool{"log": true, "api": true, "dns": true, "routing": true, "policy": true, "inbounds": true, "outbounds": true, "transport": true, "stats": true, "reverse": true, "fakedns": true, "metrics": true, "observatory": true, "burstObservatory": true}
	for k := range obj {
		if !allowed[k] {
			http.Error(w, "unsupported config section", http.StatusBadRequest)
			return
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closing {
		http.Error(w, "agent shutting down", http.StatusServiceUnavailable)
		return
	}
	version := int64(0)
	if req.ConfigVersion != nil {
		version = *req.ConfigVersion
		if version <= 0 {
			http.Error(w, "config_version must be a positive integer", http.StatusBadRequest)
			return
		}
	}
	fingerprint, err := requestFingerprint("config", obj)
	if err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	idempotent, err := a.checkFence("config", version, fingerprint)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	if idempotent {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"applied"}`)
		return
	}
	if version > 0 {
		if err := a.commitFence("config", version, fingerprint, false); err != nil {
			http.Error(w, "config version persistence failed", http.StatusInternalServerError)
			return
		}
	}
	if err := a.apply(req.Config); err != nil {
		http.Error(w, "config validation or activation failed", http.StatusUnprocessableEntity)
		log.Print("config activation failed")
		return
	}
	if err := a.commitFence("config", version, fingerprint, true); err != nil {
		http.Error(w, "config version persistence failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, `{"status":"applied"}`)
}

func (a *agent) disableHTTP(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	var req struct {
		ConfigVersion *int64 `json:"config_version"`
	}
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(&req); err != nil || d.Decode(&struct{}{}) != io.EOF {
		http.Error(w, "disable request must contain only config_version", http.StatusBadRequest)
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closing {
		http.Error(w, "agent shutting down", http.StatusServiceUnavailable)
		return
	}
	version := int64(0)
	if req.ConfigVersion != nil {
		version = *req.ConfigVersion
		if version <= 0 {
			http.Error(w, "config_version must be a positive integer", http.StatusBadRequest)
			return
		}
	}
	fingerprint, err := requestFingerprint("disable", struct{}{})
	if err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	idempotent, err := a.checkFence("disable", version, fingerprint)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	if idempotent {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"disabled"}`)
		return
	}
	if version > 0 {
		if err := a.commitFence("disable", version, fingerprint, false); err != nil {
			http.Error(w, "config version persistence failed", http.StatusInternalServerError)
			return
		}
	}
	if err := a.disable(); err != nil {
		http.Error(w, "disable failed", http.StatusInternalServerError)
		return
	}
	if err := a.commitFence("disable", version, fingerprint, true); err != nil {
		http.Error(w, "config version persistence failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, `{"status":"disabled"}`)
}

func (a *agent) disable() error {
	f, err := os.CreateTemp(filepath.Dir(a.target), ".nodehub-disabled-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0600); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(name, a.target+".disabled"); err != nil {
		return err
	}
	if err = syncDirectory(filepath.Dir(a.target)); err != nil {
		return err
	}
	a.stop()
	return nil
}

func syncDirectory(path string) error {
	d, err := os.Open(path)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

func (a *agent) apply(data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(a.target), ".nodehub-config-*.json")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	// No shell or user-supplied command arguments are involved.
	runErr := exec.CommandContext(ctx, a.xray, "run", "-test", "-c", name).Run()
	if runErr != nil {
		return fmt.Errorf("xray validation: %w", runErr)
	}
	_, err = readPrivate(a.target)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	hadPrevious := err == nil
	a.stop()
	if err = a.start(name); err != nil {
		if hadPrevious {
			if recoveryErr := a.start(a.target); recoveryErr != nil {
				return errors.New("activation and recovery failed")
			}
		}
		return err
	}
	if err = os.Rename(name, a.target); err != nil {
		a.stop()
		if hadPrevious {
			if recoveryErr := a.start(a.target); recoveryErr != nil {
				return errors.New("commit and recovery failed")
			}
		}
		return err
	}
	if err = os.Remove(a.target + ".disabled"); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err = syncDirectory(filepath.Dir(a.target)); err != nil {
		return err
	}
	return nil
}

// All lifecycle operations are serialized by mu.
func (a *agent) running() bool {
	if a.child == nil {
		return false
	}
	select {
	case <-a.done:
		a.child = nil
		a.done = nil
		return false
	default:
		return true
	}
}

func (a *agent) start(path string) error {
	cmd := exec.Command(a.xray, "run", "-c", path)
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
		return errors.New("xray exited during startup")
	case <-time.After(500 * time.Millisecond):
		a.child = cmd
		a.done = done
		return nil
	}
}

func (a *agent) stop() {
	if !a.running() {
		return
	}
	_ = a.child.Process.Signal(syscall.SIGTERM)
	select {
	case <-a.done:
	case <-time.After(3 * time.Second):
		_ = a.child.Process.Kill()
		<-a.done
	}
	a.child = nil
	a.done = nil
}

func (a *agent) shutdown() { a.mu.Lock(); defer a.mu.Unlock(); a.closing = true; a.stop() }

func (a *agent) startup() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.loadFence(); err != nil {
		return err
	}
	if a.fence.Version > 0 && (!a.fence.Applied || a.fence.Operation != "config") {
		return nil
	}
	if _, err := os.Lstat(a.target + ".disabled"); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	data, err := readPrivateLimit(a.target, (1<<20)+1)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if a.fence.Version > 0 {
		var obj map[string]json.RawMessage
		if len(data) > 1<<20 || json.Unmarshal(data, &obj) != nil || obj == nil {
			return nil
		}
		fingerprint, err := requestFingerprint("config", obj)
		if err != nil || fingerprint != a.fence.Fingerprint {
			return nil
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := exec.CommandContext(ctx, a.xray, "run", "-test", "-c", a.target).Run(); err != nil {
		return errors.New("existing config validation failed")
	}
	return a.start(a.target)
}

func main() {
	listen := flag.String("listen", "127.0.0.1:9443", "listen address")
	allowHTTP := flag.Bool("allow-http", false, "serve unencrypted HTTP instead of HTTPS (credentials and configuration are exposed)")
	cert := flag.String("tls-cert", "", "TLS certificate file")
	key := flag.String("tls-key", "", "private TLS key file")
	tokenFile := flag.String("token-file", "", "private bearer token file")
	identityFile := flag.String("identity-file", "", "private persistent identity file")
	target := flag.String("config-file", "", "dedicated Xray config path")
	xray := flag.String("xray", "", "absolute path to Xray binary")
	flag.Parse()
	if *tokenFile == "" || *identityFile == "" || *target == "" || !filepath.IsAbs(*xray) {
		log.Fatal("token-file, identity-file, config-file and absolute xray path are required")
	}
	if *allowHTTP {
		if *cert != "" || *key != "" {
			log.Fatal("allow-http cannot be combined with TLS certificate or key")
		}
		log.Print("WARNING: HTTP exposes bearer credentials and configuration without encryption")
	} else {
		if *cert == "" || *key == "" {
			log.Fatal("tls-cert and tls-key are required unless allow-http is set")
		}
		if _, err := readPrivate(*key); err != nil {
			log.Fatalf("TLS key: %v", err)
		}
	}
	b, err := readPrivate(*tokenFile)
	if err != nil {
		log.Fatalf("token: %v", err)
	}
	token := strings.TrimSpace(string(b))
	if len(token) < 32 || len(token) > 4096 || strings.ContainsAny(token, "\r\n") {
		log.Fatal("token must be 32-4096 characters on one line")
	}
	id, err := identity(*identityFile)
	if err != nil {
		log.Fatalf("identity: %v", err)
	}
	a := &agent{token: token, target: *target, xray: *xray, id: id}
	if err := a.startup(); err != nil {
		log.Fatalf("xray startup failed: %v", err)
	}
	defer a.shutdown()
	server := &http.Server{Addr: *listen, Handler: capabilityHandler(a), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 25 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 8192}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()
	var serveErr error
	if *allowHTTP {
		serveErr = server.ListenAndServe()
	} else {
		serveErr = server.ListenAndServeTLS(*cert, *key)
	}
	if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
		a.shutdown()
		log.Fatalf("server: %v", serveErr)
	}
}
