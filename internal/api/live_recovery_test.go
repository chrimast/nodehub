package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestLiveRecoveryReencryptAndForceClean(t *testing.T) {
	s := testControlDB(t)
	call := func(method, path string, body any, want int) map[string]any {
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest(method, "/api/v1/"+path, bytes.NewReader(raw))
		r.Header.Set("Authorization", "Bearer "+s.AdminToken)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s: got %d, want %d: %s", path, w.Code, want, w.Body.String())
		}
		var out map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return out
	}
	host := call("POST", "hosts", map[string]any{"name": "recovery-host", "provider": "fixture", "address": "192.0.2.44"}, 201)["id"].(string)
	_, _ = s.DB.Exec(context.Background(), "UPDATE hosts SET agent_version='0.25.5',last_seen=now() WHERE id=$1", host)
	service := call("POST", "live", map[string]any{"name": "recovery-node", "host_id": host, "kind": "shadowsocks", "engine": "sing-box", "address": "192.0.2.44", "listen": "0.0.0.0", "port": 25444}, 201)["id"].(string)
	call("POST", "live/"+service+"/reencrypt", nil, 200)
	call("POST", "live/"+service+"/force-clean", map[string]any{"confirm": true}, 200)
	var count int
	if err := s.DB.QueryRow(context.Background(), "SELECT count(*) FROM live_services WHERE id=$1", service).Scan(&count); err != nil || count != 0 {
		t.Fatalf("service remains after force clean: count=%d err=%v", count, err)
	}
}
