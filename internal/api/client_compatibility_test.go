package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"vps-control/internal/live"
)

func TestV2rayNXrayRealitySubscription(t *testing.T) {
	t.Run("tcp", func(t *testing.T) { testV2rayNXrayRealitySubscription(t, "tcp", "xtls-rprx-vision", "", "") })
	t.Run("xhttp", func(t *testing.T) {
		testV2rayNXrayRealitySubscription(t, "xhttp", "", "/video/test", "edge.example.com")
	})
}

func TestXHTTPPaddingSaveAndReadBack(t *testing.T) {
	s := testControlDB(t)
	h := api018{t, s}
	host := obj018(h.call("POST", "hosts", map[string]any{"name": "xhttp-test"}, 201, nil))
	h.call("POST", "agent/heartbeat", map[string]any{"mode": "simulation", "version": "0.25.7", "platform": "linux/amd64"}, 200, host)
	body := map[string]any{"name": "xhttp-padding", "host_id": host["id"], "kind": "vless", "engine": "xray", "address": "192.0.2.10", "port": 26334, "listen": "127.0.0.1", "transport": "xhttp", "path": "/video", "tls_mode": "reality", "server_name": "example.com", "reality": map[string]any{"target": "example.com", "port": 443}, "xhttp_padding": "100-1000"}
	id := obj018(h.call("POST", "live", body, 201, nil))["id"].(string)
	check := func(want string) any {
		raw := h.call("GET", "live/"+id, nil, 200, nil)
		got := obj018(raw)
		if got["spec"].(map[string]any)["xhttp_padding"] != want || !strings.Contains(string(raw), `"xPaddingBytes":"`+want+`"`) {
			t.Fatalf("padding %q was not saved and rendered", want)
		}
		return got["version"]
	}
	body["version"] = check("100-1000")
	body["xhttp_padding"] = "500-50"
	h.call("PUT", "live/"+id, body, 400, nil)
	body["xhttp_padding"] = "50-500"
	h.call("PUT", "live/"+id, body, 200, nil)
	check("50-500")
}

func TestEncryptedXHTTPSubscription(t *testing.T) {
	f := newSubscriptionFixture(t, 0)
	body := map[string]any{"name": "encrypted-xhttp", "host_id": f.host["id"], "kind": "vless", "engine": "xray", "address": "192.0.2.10", "port": 26333, "transport": "xhttp", "path": "/video", "tls_mode": "reality", "server_name": "www.apple.com", "flow": "xtls-rprx-vision", "reality": map[string]any{"target": "www.apple.com", "port": 443}, "managed_users": true, "vless_encryption": true}
	f.h.call("POST", "live", body, 409, nil)
	f.h.call("POST", "agent/heartbeat", map[string]any{"mode": "simulation", "version": "0.25.7", "platform": "linux/amd64"}, 200, f.host)
	f.h.call("POST", "agent/live/sync", map[string]any{"bundle_mode": true, "capabilities": []live.Capability{{Engine: "xray", Available: true, Version: "Xray 26.10.31"}}}, 200, f.host)
	id := obj018(f.h.call("POST", "live", body, 201, nil))["id"].(string)
	detail := f.h.call("GET", "live/"+id, nil, 200, nil)
	if obj018(detail)["vless_encryption"] != true || strings.Contains(string(detail), "mlkem768x25519plus") {
		t.Fatal("detail did not safely report the encryption setting")
	}
	f.h.call("POST", "live/"+id+"/toggle", map[string]any{"enabled": true}, 200, nil)
	f.changeGroup([]string{id})
	f.ack()
	raw := strings.Split(f.url, "?")[0]
	w := f.share(raw+"?client=v2rayn&policy=nodes-only", nil, 200)
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(w.Body.String()))
	if err != nil {
		t.Fatal("invalid node list")
	}
	u, err := url.Parse(strings.TrimSpace(string(data)))
	if err != nil || u.Scheme != "vless" {
		t.Fatal("invalid node URI")
	}
	q := u.Query()
	value, ok := strings.CutPrefix(q.Get("encryption"), "mlkem768x25519plus.native.0rtt.")
	pub, err := base64.RawURLEncoding.DecodeString(value)
	if !ok || err != nil || len(pub) != 1184 || q.Has("decryption") || q.Get("type") != "xhttp" || q.Get("flow") != "xtls-rprx-vision" {
		t.Fatal("invalid encrypted XHTTP subscription")
	}
	f.share(raw+"?client=xray", nil, 200)
	f.share(raw+"?client=mihomo", nil, 422)
}

func testV2rayNXrayRealitySubscription(t *testing.T, transport, flow, path, host string) {
	f := newSubscriptionFixture(t, 0)
	f.h.call("POST", "agent/live/sync", map[string]any{"bundle_mode": true, "capabilities": []live.Capability{{Engine: "xray", Available: true, Version: "Xray 26.10.21"}}}, 200, f.host)
	padding := ""
	if transport == "xhttp" {
		padding = "250-700"
	}
	id := obj018(f.h.call("POST", "live", map[string]any{"name": "xray-reality", "host_id": f.host["id"], "kind": "vless", "engine": "xray", "listen": "127.0.0.1", "address": "192.0.2.10", "port": 26332, "transport": transport, "xhttp_padding": padding, "tls_mode": "reality", "server_name": "www.apple.com", "flow": flow, "path": path, "ws_host": host, "reality": map[string]any{"target": "www.apple.com", "port": 443, "fingerprint": "chrome"}, "managed_users": true}, 201, nil))["id"].(string)
	f.h.call("POST", "live/"+id+"/toggle", map[string]any{"enabled": true}, 200, nil)
	f.changeGroup([]string{id})
	f.ack()
	raw := strings.Split(f.url, "?")[0]
	for _, link := range []string{raw + "?client=v2rayn&policy=nodes-only", raw} {
		w := f.share(link, map[string]string{"User-Agent": "v2rayN"}, 200)
		data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(w.Body.String()))
		if err != nil {
			t.Fatal("not a base64 node list", err)
		}
		u, err := url.Parse(strings.TrimSpace(string(data)))
		if err != nil || u.Scheme != "vless" || u.Port() != "26332" || u.User.Username() == "" {
			t.Fatal("not a VLESS node")
		}
		q := u.Query()
		if padding != "" {
			var extra map[string]any
			if err := json.Unmarshal([]byte(q.Get("extra")), &extra); err != nil || extra["xPaddingBytes"] != padding || len(extra) != 1 {
				t.Fatal("node subscription did not preserve XHTTP padding in extra")
			}
		} else if q.Has("extra") {
			t.Fatal("unexpected XHTTP extra on TCP node")
		}

		for key, value := range map[string]string{"type": transport, "security": "reality", "encryption": "none", "sni": "www.apple.com", "fp": "chrome", "flow": flow, "path": path, "host": host} {
			if q.Get(key) != value {
				t.Errorf("missing or wrong %s", key)
			}
		}
		if pub, err := base64.RawURLEncoding.DecodeString(q.Get("pbk")); err != nil || len(pub) != 32 || q.Get("sid") == "" {
			t.Fatal("missing REALITY key or short ID")
		}
	}
	f.share(raw+"?client=mihomo", nil, 422)
	f.share(raw+"?client=xray", nil, 200)
}

func TestAppSubscriptionsHaveSelectableCompatibleNodes(t *testing.T) {
	f := newSubscriptionFixture(t, 0)
	snell := obj018(f.h.call("POST", "live", map[string]any{"name": "only-snell", "host_id": f.host["id"], "kind": "snell", "engine": "sing-box", "listen": "127.0.0.1", "address": "192.0.2.10", "port": 26322}, 201, nil))["id"].(string)
	f.h.call("POST", "live/"+snell+"/toggle", map[string]any{"enabled": true}, 200, nil)
	f.changeGroup([]string{f.node, snell})
	f.ack()
	f.h.call("POST", "client-policies", map[string]any{"name": "filter-test", "client_id": "all", "source": "proxy-groups:\n- name: removed\n  type: select\n  proxies: [protocol:snell]\n- name: usable\n  type: select\n  proxies: [removed, ALL]\nrules: ['DOMAIN-SUFFIX,blocked.example,removed', 'MATCH,usable']\n"}, 200, nil)
	raw := strings.Split(f.url, "?")[0]
	for _, c := range []string{"v2rayng", "v2rayn", "nekobox-uri", "shadowrocket", "happ", "v2raytun"} {
		w := f.share(raw+"?client="+c, nil, 200)
		decoded, err := base64.StdEncoding.DecodeString(w.Body.String())
		if err != nil || !strings.HasPrefix(string(decoded), "vmess://") || strings.Contains(string(decoded), "snell://") || w.Header().Get("X-Starhub-Excluded-Nodes") != "1" {
			t.Fatal(c, w.Header(), err)
		}
	}
	w := f.share(raw+"?client=nekobox", nil, 200)
	if len(proxies019(t, w)) != 1 || strings.Contains(w.Body.String(), "proxy-groups:") || strings.Contains(w.Body.String(), "rules:") {
		t.Fatal("NekoBox must receive plain supported proxies only")
	}
	for _, c := range []string{"hiddify", "sing-box", "sfa"} {
		w := f.share(raw+"?client="+c, nil, 200)
		var cfg map[string]any
		json.Unmarshal(w.Body.Bytes(), &cfg)
		if e := validateClientGraph("sing-box", cfg); e != nil {
			t.Fatal(e)
		}
		hasTun := false
		for _, v := range cfg["inbounds"].([]any) {
			if object(v)["type"] == "tun" {
				hasTun = true
			}
		}
		if !hasTun || object(object(cfg["experimental"])["clash_api"])["external_controller"] != "127.0.0.1:9090" {
			t.Fatal(c, "missing App TUN/selector API")
		}
		if strings.Contains(w.Body.String(), "only-snell") {
			t.Fatal(c, "received Snell without a confirmed compatible core")
		}
		if strings.Contains(w.Body.String(), `"tag":"removed"`) {
			t.Fatal("empty group survived")
		}
	}
	f.changeGroup([]string{snell})
	f.ack()
	for _, c := range []string{"hiddify", "v2rayng", "nekobox"} {
		f.share(raw+"?client="+c, nil, 422)
	}
}

func TestUserSharedCountersAreNotPersonalBilling(t *testing.T) {
	f := newSubscriptionFixture(t, 0)
	users := f.h.call("GET", "proxy-users", nil, 200, nil)
	if !strings.Contains(string(users), `"metered_nodes": 1`) && !strings.Contains(string(users), `"metered_nodes":1`) {
		t.Fatal(string(users))
	}
}

func TestAndroidVLESSRealitySubscriptions(t *testing.T) {
	f := newSubscriptionFixture(t, 0)
	id := obj018(f.h.call("POST", "live", map[string]any{"name": "vless-android", "host_id": f.host["id"], "kind": "vless", "engine": "sing-box", "listen": "127.0.0.1", "address": "192.0.2.10", "port": 26331, "tls_mode": "reality", "server_name": "www.apple.com", "flow": "xtls-rprx-vision", "reality": map[string]any{"target": "www.apple.com", "port": 443, "fingerprint": "chrome"}, "managed_users": true}, 201, nil))["id"].(string)
	f.h.call("POST", "live/"+id+"/toggle", map[string]any{"enabled": true}, 200, nil)
	f.changeGroup([]string{id})
	f.ack()
	raw := strings.Split(f.url, "?")[0]
	w := f.share(raw+"?client=nekobox", nil, 200)
	nodes := proxies019(t, w)
	if len(nodes) != 1 || nodes[0]["type"] != "vless" || nodes[0]["servername"] != "www.apple.com" || nodes[0]["uuid"] == "" || nodes[0]["reality-opts"] == nil {
		t.Fatal("NekoBox VLESS/REALITY fields missing")
	}
	for _, c := range []string{"nekobox-uri", "v2rayng"} {
		w = f.share(raw+"?client="+c, nil, 200)
		decoded, e := base64.StdEncoding.DecodeString(w.Body.String())
		if e != nil || !strings.HasPrefix(string(decoded), "vless://") || !strings.Contains(string(decoded), "security=reality") || !strings.Contains(string(decoded), "pbk=") {
			t.Fatal(c, "VLESS URI fields missing")
		}
	}
	auto := f.share(raw, map[string]string{"User-Agent": "Dart/3.8 (dart:io)"}, 200)
	plain, e := base64.StdEncoding.DecodeString(auto.Body.String())
	if e != nil || !strings.HasPrefix(string(plain), "vless://") {
		t.Fatal("unrecognized Hiddify-style UA must receive portable nodes")
	}
	for _, c := range []string{"sing-box", "hiddify"} {
		w = f.share(raw+"?client="+c, nil, 200)
		var cfg map[string]any
		if e = json.Unmarshal(w.Body.Bytes(), &cfg); e != nil || validateClientGraph("sing-box", cfg) != nil {
			t.Fatal(c, "invalid config")
		}
	}
}

func TestAppDefaultPolicyValidatesWithOfficialCore(t *testing.T) {
	binary := os.Getenv("TEST_SING_BOX_BINARY")
	if binary == "" {
		t.Skip("official sing-box binary required")
	}
	f := newSubscriptionFixture(t, 0)
	if err := f.h.s.EnsureDefaultClientPolicy(context.Background()); err != nil {
		t.Fatal(err)
	}
	raw := strings.Split(f.url, "?")[0]
	for _, c := range []string{"sing-box", "sing-box-latest", "sfa", "hiddify"} {
		w := f.share(raw+"?client="+c, nil, 200)
		var config map[string]any
		json.Unmarshal(w.Body.Bytes(), &config)
		if e := validateClientGraph("sing-box", config); e != nil {
			t.Fatal(e)
		}
		for _, v := range array(object(config["route"])["rule_set"]) {
			set := object(v)
			detour := set["download_detour"]
			if c == "sing-box-latest" {
				detour = object(set["http_client"])["detour"]
				if set["download_detour"] != nil {
					t.Fatal("modern config contains deprecated download_detour")
				}
			}
			if detour != object(config["route"])["final"] {
				t.Fatal("rule set downloader references removed default selector")
			}
		}
		path := filepath.Join(t.TempDir(), "config.json")
		os.WriteFile(path, w.Body.Bytes(), 0600)
		result, err := exec.Command(binary, "check", "-c", path).CombinedOutput()
		if err != nil {
			t.Fatalf("%s rejected by official core: %s", c, result)
		}
	}
}
