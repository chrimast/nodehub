package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"vps-control/internal/live"
)

func (f subscriptionFixture) compatible(limit int) {
	f.h.t.Helper()
	f.h.call("PUT", "proxy-users/"+f.user, map[string]any{"name": "existing-user", "enabled": true, "expires_at": time.Now().Add(24 * time.Hour), "device_limit_mode": "compatible", "max_devices": limit}, 200, nil)
	f.ack()
}

func TestUnlimitedObservationBoundAndTelegramCount(t *testing.T) {
	f := newSubscriptionFixture(t, 0)
	f.compatible(0)
	for i := 1; i <= 32; i++ {
		r := httptest.NewRequest("GET", "https://vps.example.com/s/fixture", nil)
		r.RemoteAddr = fmt.Sprintf("192.0.2.%d:1234", i)
		r.Header.Set("User-Agent", "Clash-Verge/2.4")
		if err := f.h.s.observeSubscription(httptest.NewRecorder(), r, f.user, "mihomo"); err != nil {
			t.Fatal(err)
		}
	}
	rows := f.observations()
	if len(rows) != 30 {
		t.Fatalf("wanted 30 most recent observations, got %d", len(rows))
	}
	summary, err := f.h.s.notificationSummary(context.Background(), "devices")
	if err != nil || !strings.Contains(summary, "兼容访问记录 30") || !strings.Contains(summary, "非物理设备数") {
		t.Fatalf("incorrect Telegram summary: %q %v", summary, err)
	}
	// A disabled observation is not discarded by the capacity cap.
	id := rows[0]["id"].(string)
	f.h.call("PUT", "proxy-users/"+f.user+"/devices/"+id, map[string]any{"name": "blocked", "enabled": false, "version": rows[0]["version"]}, 200, nil)
	r := httptest.NewRequest("GET", "https://vps.example.com/s/fixture", nil)
	r.RemoteAddr = "192.0.2.100:1234"
	if err := f.h.s.observeSubscription(httptest.NewRecorder(), r, f.user, "mihomo"); err != nil {
		t.Fatal(err)
	}
	var enabled bool
	if err := f.h.s.DB.QueryRow(context.Background(), `SELECT enabled FROM subscription_clients WHERE id=$1`, id).Scan(&enabled); err != nil || enabled {
		t.Fatalf("blocked observation removed or enabled: %v", err)
	}
}

func TestSubscriptionRenameRetainsLinkAndTitle(t *testing.T) {
	f := newSubscriptionFixture(t, 0)
	var expiry time.Time
	var version int
	if err := f.h.s.DB.QueryRow(context.Background(), `SELECT expires_at,version FROM subscriptions WHERE id=$1`, f.subscription).Scan(&expiry, &version); err != nil {
		t.Fatal(err)
	}
	f.h.call("PUT", "subscriptions/"+f.subscription, map[string]any{"name": "新的订阅", "profile_id": "", "expires_at": expiry, "version": version}, 200, nil)
	w := f.share(f.url, nil, 200)
	if got := w.Header().Get("Profile-Title"); got != "base64:"+base64.StdEncoding.EncodeToString([]byte("新的订阅")) {
		t.Fatalf("wrong title: %s", got)
	}
	got := obj018(f.h.call("GET", "subscriptions/"+f.subscription+"/link", nil, 200, nil))
	if got["url"] != strings.Split(f.url, "?")[0] {
		t.Fatal("renaming changed subscription link")
	}
}
func (f subscriptionFixture) observations() []map[string]any {
	f.h.t.Helper()
	var rows []map[string]any
	if e := json.Unmarshal(f.h.call("GET", "proxy-users/"+f.user+"/devices", nil, 200, nil), &rows); e != nil {
		f.h.t.Fatal(e)
	}
	return rows
}
func TestClashCompatibleDeviceLimitAndObservation(t *testing.T) {
	f := newSubscriptionFixture(t, 1)
	f.compatible(1)
	raw := strings.Split(f.url, "?")[0]
	for _, ua := range []string{"Clash-Verge/2.4", "FlClash/0.8"} {
		w := f.share(raw, map[string]string{"User-Agent": ua}, 200)
		if len(proxies019(t, w)) != 1 || w.Header().Get("X-Starhub-Device-Recognition") != "source_and_client" {
			t.Fatal("ordinary Clash failed")
		}
	}
	rows := f.observations()
	if len(rows) != 1 || rows[0]["registration_source"] != "observed_client" || rows[0]["user_agent"] != "FlClash/0.8" || rows[0]["source_ip"] == nil {
		t.Fatal("missing ordinary client metadata", rows)
	}
	if _, ok := rows[0]["fingerprint"]; ok {
		t.Fatal("private fingerprint exposed")
	}
	// A direct public peer cannot spoof the reverse proxy's trusted address header.
	f.share(raw, map[string]string{"User-Agent": "FlClash/0.8", "X-Real-IP": "203.0.113.2"}, 200)
	f.share(raw, map[string]string{"User-Agent": "Shadowrocket/2.2"}, 403)
	id := rows[0]["id"].(string)
	f.h.call("PUT", "proxy-users/"+f.user+"/devices/"+id, map[string]any{"name": "Laptop", "enabled": false, "version": rows[0]["version"]}, 200, nil)
	f.share(raw, map[string]string{"User-Agent": "FlClash/0.8"}, 403)
	f.share(raw, map[string]string{"User-Agent": "Shadowrocket/2.2"}, 200)
	f.h.call("PUT", "proxy-users/"+f.user+"/devices/"+id, map[string]any{"name": "Laptop", "enabled": true, "version": 2}, 409, nil)
	f.h.call("DELETE", "proxy-users/"+f.user+"/devices/"+id, nil, 200, nil)
	var count int
	f.h.s.DB.QueryRow(context.Background(), "SELECT count(*) FROM user_devices WHERE user_id=$1", f.user).Scan(&count)
	if count != 0 {
		t.Fatal("compatibility creates unnecessary proxy identities")
	}
}
func TestUnlimitedClientsRecordedAndAdminCheckDoesNotReserveSlots(t *testing.T) {
	f := newSubscriptionFixture(t, 0)
	f.compatible(0)
	f.h.call("GET", "subscriptions/"+f.subscription+"/check?client=mihomo", nil, 200, nil)
	if len(f.observations()) != 0 {
		t.Fatal("admin check reserves a client slot")
	}
	f.share(f.url, map[string]string{"User-Agent": "Clash-Verge/2.4"}, 200)
	if len(f.observations()) != 1 {
		t.Fatal("unlimited user client not recorded")
	}
	// Compatible subscription import doesn't change the deployed account version.
	var pending bool
	f.h.s.DB.QueryRow(context.Background(), "SELECT state!='running' FROM live_services WHERE id=$1", f.node).Scan(&pending)
	if pending {
		t.Fatal("observation queued a deployment")
	}
}
func TestUserShareReusesOriginalLinkAndFormatMatrix(t *testing.T) {
	f := newSubscriptionFixture(t, 0)
	f.compatible(0)
	for i := 0; i < 2; i++ {
		got := obj018(f.h.call("POST", "proxy-users/"+f.user+"/subscription", map[string]any{}, 200, nil))
		if got["id"] != f.subscription || got["url"] != strings.Split(f.url, "?")[0] || got["reused"] != true {
			t.Fatal("share recreated or rotated original", got["id"])
		}
	}
	raw := strings.Split(f.url, "?")[0]
	// Every application alias produces an appropriate parseable configuration.
	for _, c := range clients {
		if c.ID == "naive-proxy" {
			continue
		} // VMess is deliberately incompatible with Naive.
		query := "?client=" + c.ID
		check := obj018(f.h.call("GET", "subscriptions/"+f.subscription+"/check"+query, nil, 200, nil))
		if check["ready"] != true || check["client"] != c.ID {
			t.Fatalf("%s: wrong readiness %v", c.ID, check)
		}
		w := f.share(raw+query, nil, 200)
		content := f.h.call("GET", "subscriptions/"+f.subscription+"/content"+query, nil, 200, nil)
		if string(content) != w.Body.String() {
			t.Fatalf("%s: content differs from public download", c.ID)
		}
		switch c.Format {
		case "sing-box", "xray":
			var config map[string]any
			if e := json.Unmarshal(w.Body.Bytes(), &config); e != nil || len(config["outbounds"].([]any)) == 0 {
				t.Fatal(c.ID, "invalid JSON")
			}
		case "mihomo", "mihomo-nodes":
			if len(proxies019(t, w)) != 1 {
				t.Fatal(c.ID, "missing proxy")
			}
		case "base64", "shadowrocket":
			plain, e := base64.StdEncoding.DecodeString(w.Body.String())
			if e != nil || !strings.Contains(string(plain), "vmess://") {
				t.Fatal(c.ID, "invalid node subscription")
			}
		case "shadowrocket-config":
			if !strings.Contains(w.Body.String(), "[Rule]") {
				t.Fatal("missing separate Shadowrocket rules")
			}
		}
	}
	auto := f.share(raw, map[string]string{"User-Agent": "Shadowrocket/2.2"}, 200)
	manual := f.share(raw+"?client=shadowrocket", nil, 200)
	if auto.Body.String() != manual.Body.String() {
		t.Fatal("automatic/manual Shadowrocket differ")
	}
	f.share(raw+"?client=flclash", nil, 200)
}

func TestUserShareFallsBackToRequestOriginWithoutSiteSettings(t *testing.T) {
	f := newSubscriptionFixture(t, 0)
	if _, e := f.h.s.DB.Exec(context.Background(), `DELETE FROM site_settings WHERE tenant_id=$1`, tenant); e != nil {
		t.Fatal(e)
	}
	w := obj018(f.h.call("POST", "proxy-users/"+f.user+"/subscription", map[string]any{}, 200, nil))
	if !strings.HasPrefix(w["url"].(string), "http://example.com/s/") {
		t.Fatalf("unexpected fallback origin: %v", w["url"])
	}
}
func TestSubscriptionPeerTrustBoundary(t *testing.T) {
	for _, v := range []struct{ peer, header, want string }{{"192.0.2.1:5000", "203.0.113.1", "192.0.2.1"}, {"127.0.0.1:8091", "203.0.113.1", "203.0.113.1"}, {"[::1]:8091", "2001:db8::2", "2001:db8::2"}, {"127.0.0.1:8091", "garbage", "127.0.0.1"}} {
		r := httptest.NewRequest("GET", "http://example.test", nil)
		r.RemoteAddr = v.peer
		r.Header.Set("X-Real-IP", v.header)
		if got := subscriptionPeer(r); got != v.want {
			t.Fatal(v, got)
		}
	}
}

func TestCatalogPoliciesReachPreviewAndDownload(t *testing.T) {
	f := newSubscriptionFixture(t, 0)
	for _, c := range clients {
		if !c.Full {
			continue
		}
		t.Run(c.ID, func(t *testing.T) {
			f := f
			f.h.t = t
			marker := c.ID + ".example.test"
			body := map[string]any{"name": c.ID, "user_id": f.user, "client_id": c.ID, "source": "rules: ['DOMAIN," + marker + ",DIRECT','MATCH,PROXY']"}
			if c.Format == "shadowrocket-config" {
				body["native_options"] = map[string]any{c.Format: map[string]any{"general": map[string]any{"ipv6": false}}}
			}
			f.h.call("POST", "client-policies", body, 200, nil)
			preview := f.h.call("POST", "client-policies/preview", bodyWithoutName(body), 200, nil)
			raw := strings.Split(f.url, "?")[0] + "?client=" + c.ID
			w := f.share(raw, nil, 200)
			for _, data := range []string{string(preview), w.Body.String()} {
				if !strings.Contains(data, marker) {
					t.Fatal("missing selected client's policy")
				}
				if c.Format == "shadowrocket-config" && !strings.Contains(data, "ipv6 = false") {
					t.Fatal("missing Shadowrocket override")
				}
				if c.Format == "sing-box" && !strings.Contains(data, "tun-in") {
					t.Fatal("missing sing-box app inlet")
				}
			}
		})
	}
}

func bodyWithoutName(body map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range body {
		if k != "name" {
			out[k] = v
		}
	}
	return out
}

func TestNaiveShareIgnoresIncompatibleEndpointResolver(t *testing.T) {
	f := newSubscriptionFixture(t, 0)
	req := httptest.NewRequest("GET", "/", nil)
	spec, err := f.h.s.loadSpec(req, f.node)
	if err != nil {
		t.Fatal(err)
	}
	spec.Kind = "naive"
	spec.ServerName = "naive.example.test"
	spec.TLSMode = "tls"
	sealed, err := live.Seal(f.h.s.MasterKey, mustJSON(spec))
	if err != nil {
		t.Fatal(err)
	}
	other := f.addNode("other", 26020)
	f.changeGroup([]string{f.node, other})
	f.ack()
	if _, err = f.h.s.DB.Exec(context.Background(), "UPDATE live_services SET kind='naive',sealed=$2 WHERE id=$1", f.node, sealed); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{"?client=naive-proxy", "?format=naive"} {
		check := obj018(f.h.call("GET", "subscriptions/"+f.subscription+"/check"+q, nil, 200, nil))
		if check["ready"] != true {
			t.Fatal(check)
		}
		w := f.share(strings.Split(f.url, "?")[0]+q, nil, 200)
		got := obj018(w.Body.Bytes())
		if got["host-resolver-rules"] != "MAP naive.example.test 192.0.2.10" || !strings.HasSuffix(got["proxy"].(string), "@naive.example.test:26019") {
			t.Fatal("wrong Naive endpoint")
		}
	}
}

func TestDedicatedPolicyRejectsUnsupportedGroupsBeforeSave(t *testing.T) {
	h := api018{t, testControlDB(t)}
	for _, client := range []string{"xray", "sing-box", "sfa", "hiddify", "base64"} {
		h.call("POST", "client-policies", map[string]any{"name": "invalid", "client_id": client, "source": "proxy-groups: [{name: Group, type: fallback, proxies: [ALL]}]\nrules: ['MATCH,Group']"}, 400, nil)
	}
}
