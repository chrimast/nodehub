package api

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vps-control/internal/controlorigin"
)

func TestRepairCommandHTTPSOriginResolution(t *testing.T) {
	for _, tc := range []struct {
		name, site, origins, request, forwarded, reported, want string
		publicPeer, allowHTTP                                   bool
	}{
		{name: "configured Agent origin first", site: "https://panel.example", origins: `["https://agent.example:8443"]`, request: "https://current.example", want: "https://agent.example:8443"},
		{name: "configured management origin next", site: "https://panel.example", request: "https://current.example", want: "https://panel.example"},
		{name: "unconfigured TLS origin", request: "https://current.example:8443"},
		{name: "unconfigured TLS origin when settings HTTP", site: "http://panel.example", request: "https://current.example"},
		{name: "unconfigured forwarded origin", site: "http://panel.example", request: "http://api:8080", forwarded: "current.example:8443"},
		{name: "ambiguous forwarded host", request: "http://api:8080", forwarded: "current.example,other.example"},
		{name: "public peer cannot assert HTTPS", request: "http://api:8080", forwarded: "current.example", publicPeer: true},
		{name: "reported HTTPS alone is not trusted", site: "http://panel.example", request: "http://current.example", reported: "https://reported.example"},
		{name: "reported HTTPS usable after admin configuration", site: "http://panel.example", origins: `["https://reported.example"]`, request: "http://current.example", reported: "https://reported.example", want: "https://reported.example"},
		{name: "configured HTTP opt in", site: "http://current.example", request: "http://current.example", want: "http://current.example", allowHTTP: true},
		{name: "HTTP only", site: "http://panel.example", request: "http://current.example", reported: "http://reported.example"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := testControlDB(t)
			if tc.allowHTTP {
				s.AgentReleaseDir = t.TempDir()
				contents := []byte("fixture-agent-package")
				digest := fmt.Sprintf("%x", sha256.Sum256(contents))
				manifest := map[string]agentPackage{}
				for _, arch := range []string{"amd64", "arm64"} {
					platform := "linux-" + arch
					manifest[platform] = agentPackage{digest, int64(len(contents))}
					if err := os.WriteFile(filepath.Join(s.AgentReleaseDir, platform+".zip"), contents, 0600); err != nil {
						t.Fatal(err)
					}
				}
				raw, _ := json.Marshal(manifest)
				if err := os.WriteFile(filepath.Join(s.AgentReleaseDir, "manifest.json"), raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			h := api018{t, s}
			host := obj018(h.call("POST", "hosts", map[string]string{"name": "repair test"}, 201, nil))
			if tc.site != "" {
				scheme, domain, _ := strings.Cut(tc.site, "://")
				origins := tc.origins
				if origins == "" {
					origins = `[]`
				}
				if _, err := s.DB.Exec(context.Background(), `INSERT INTO site_settings(tenant_id,domain,https,agent_origins) VALUES($1,$2,$3,$4)`, tenant, domain, scheme == "https", origins); err != nil {
					t.Fatal(err)
				}
			}
			if tc.reported != "" {
				h.call("POST", "agent/heartbeat", map[string]string{"control_origin": tc.reported}, 200, host)
			}
			handler := s.Handler()
			request := func(method, path, body string) *httptest.ResponseRecorder {
				r := httptest.NewRequest(method, tc.request+"/api/v1/"+path, strings.NewReader(body))
				r.Header.Set("Authorization", "Bearer "+s.AdminToken)
				if tc.forwarded != "" {
					if !tc.publicPeer {
						r.RemoteAddr = "172.18.0.2:4321"
					}
					r.Header.Set("X-Forwarded-Host", tc.forwarded)
					r.Header.Set("X-Forwarded-Proto", "https")
				}
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, r)
				return w
			}
			id := host["id"].(string)
			for _, action := range []struct{ query, flags string }{
				{"", ""}, {"?upgrade=1", " --upgrade-agent"}, {"?upgrade=1&hopping=1", " --upgrade-agent --enable-port-hopping"},
			} {
				query, flags, curl := action.query, action.flags, "curl --proto '=https' --tlsv1.2 -fsS '"
				if tc.allowHTTP {
					sep := "?"
					if query != "" {
						sep = "&"
					}
					query += sep + "allow_insecure_http=1"
					flags += " --allow-insecure-http"
					curl = "curl --proto '=http' -fsS '"
				}
				w := request(http.MethodGet, "hosts/"+id+"/repair-command"+query, "")
				var result map[string]string
				if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				if tc.want == "" {
					if w.Code != 400 || result["command"] != "" || !strings.Contains(result["error"], "登记 HTTPS 控制地址") {
						t.Fatalf("expected actionable HTTPS error, got %d: %s", w.Code, w.Body)
					}
					continue
				}
				cmd := result["command"]
				if tc.allowHTTP && action.flags != "" {
					digest := fmt.Sprintf("%x", sha256.Sum256([]byte("fixture-agent-package")))
					flags += " --package-amd64=" + digest + " --package-arm64=" + digest
				}
				if w.Code != 200 || result["origin"] != tc.want ||
					!strings.Contains(cmd, curl+tc.want+"/api/v1/agent/repair.py'") ||
					!strings.Contains(cmd, fmt.Sprintf("'%x'", sha256.Sum256(bootstrapSource("repair.py")))) ||
					!strings.Contains(cmd, `sha256sum -c - && $S python3 "$f" '`+tc.want+"'"+flags+";") ||
					strings.Contains(cmd, host["agent_token"].(string)) {
					t.Fatalf("unexpected %s command: %d %s", action.query, w.Code, w.Body)
				}
			}
			if tc.want != "" {
				nonce := strings.Repeat("b", 64)
				body, _ := json.Marshal(map[string]any{"host_id": id, "nonce": nonce, "origin": tc.want, "allow_insecure_http": tc.allowHTTP})
				w := request(http.MethodPost, "agent/verify-origin", string(body))
				var result map[string]string
				if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || w.Code != 200 || result["proof"] != controlorigin.Proof(Hash(host["agent_token"].(string)), nonce, tc.want) {
					t.Fatalf("generated address cannot prove the host identity: %d %s", w.Code, w.Body)
				}
			}
		})
	}
}
