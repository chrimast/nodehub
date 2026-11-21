package live

import (
	"encoding/json"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestXHTTPVLESSEncryption(t *testing.T) {
	t.Run("xhttp padding renders in server and client", func(t *testing.T) {
		r := NewReality()
		r.Target = "example.com"
		r.Port = 443
		r.Fingerprint = "chrome"
		s := Spec{Engine: "xray", Kind: "vless", Transport: "xhttp", ProtocolOptions: ProtocolOptions{XHTTPPadding: "50-500"}, Path: "/video", TLSMode: "reality", Reality: r, ServerName: "example.com", Address: "192.0.2.1", Listen: "0.0.0.0", Port: 24443, UUID: "00000000-0000-4000-8000-000000000001", Password: Secret()}
		_, files, err := (Work{Spec: s, Enabled: true}).Render()
		if err != nil {
			t.Fatal(err)
		}
		var config map[string]any
		if err = json.Unmarshal(files["config.json"], &config); err != nil {
			t.Fatal(err)
		}
		in := config["inbounds"].([]any)[0].(map[string]any)
		stream := in["streamSettings"].(map[string]any)
		extra := stream["xhttpSettings"].(map[string]any)["extra"].(map[string]any)
		if extra["xPaddingBytes"] != "50-500" {
			t.Fatalf("unexpected inbound XHTTP padding: %#v", extra)
		}
		out := s.xrayOutbound("test")["streamSettings"].(map[string]any)["xhttpSettings"].(map[string]any)["extra"].(map[string]any)
		if out["xPaddingBytes"] != "50-500" {
			t.Fatalf("unexpected outbound XHTTP padding: %#v", out)
		}
	})
	t.Run("disabled by default", func(t *testing.T) {
		r := NewReality()
		r.Target = "example.com"
		r.Port = 443
		r.Fingerprint = "chrome"
		s := Spec{Engine: "xray", Kind: "vless", Transport: "xhttp", ProtocolOptions: ProtocolOptions{XHTTPPadding: "100-1000"}, Path: "/video", TLSMode: "reality", Reality: r, ServerName: "example.com", Address: "192.0.2.1", Listen: "127.0.0.1", Port: 24443, UUID: "00000000-0000-4000-8000-000000000001"}
		if err := s.Validate(); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("enabled", func(t *testing.T) {
		r := NewReality()
		r.Target = "example.com"
		s := Spec{Engine: "xray", Kind: "vless", Transport: "xhttp", ProtocolOptions: ProtocolOptions{XHTTPPadding: "100-1000"}, Path: "/video", TLSMode: "reality", Reality: r, ServerName: "example.com", Address: "192.0.2.1", Listen: "127.0.0.1", Port: 24443, UUID: "00000000-0000-4000-8000-000000000001", Flow: "xtls-rprx-vision"}
		if s.Validate() == nil {
			t.Fatal("accepted unencrypted XHTTP Vision")
		}
		if err := s.EnableVLESSEncryption(); err != nil {
			t.Fatal(err)
		}
		_, files, err := s.Render()
		if err != nil {
			t.Fatal(err)
		}
		var cfg struct {
			Inbounds []struct{ Settings struct{ Decryption string } }
		}
		if json.Unmarshal(files["config.json"], &cfg) != nil || len(cfg.Inbounds) != 1 || cfg.Inbounds[0].Settings.Decryption != s.Decryption {
			t.Fatal("inbound lost decryption")
		}
		_, _, link, err := s.Client("test")
		if err != nil {
			t.Fatal(err)
		}
		u, err := url.Parse(link)
		if err != nil || u.Query().Get("encryption") != s.Encryption || u.Query().Get("flow") != s.Flow {
			t.Fatal("URI lost encryption or flow")
		}
		for _, format := range []string{"sing-box", "mihomo", "base64"} {
			if s.ClientFormat(format) == nil {
				t.Fatal("accepted unsupported format", format)
			}
		}
		s.StripServerSecrets()
		if s.Decryption != "" || s.Encryption == "" {
			t.Fatal("incorrect secret stripping")
		}
		if err := s.validateClient(); err != nil {
			t.Fatal(err)
		}
		if binary := os.Getenv("TEST_XRAY_BINARY"); binary != "" {
			client, err := json.Marshal(map[string]any{"outbounds": []any{s.XrayClient("entry")}})
			if err != nil {
				t.Fatal(err)
			}
			for name, data := range map[string][]byte{"inbound": files["config.json"], "outbound": client} {
				path := filepath.Join(t.TempDir(), name+".json")
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
				if err := exec.Command(binary, "run", "-test", "-c", path).Run(); err != nil {
					t.Fatalf("Xray rejected %s configuration: %v", name, err)
				}
			}
		}
	})
}
