package live

import (
	"encoding/json"
	"testing"
)

func TestLatestInboundOptionsRenderForPinnedSingBox(t *testing.T) {
	cert,key,err:=Certificate("example.com");if err!=nil{t.Fatal(err)}
	s := Spec{Engine:"sing-box", Kind:"tuic", Listen:"127.0.0.1", Address:"192.0.2.2", Port:24443, UUID:"00000000-0000-4000-8000-000000000001", Password:"fixture-password", ServerName:"example.com", Congestion:"cubic", TLSMode:"tls", Certificate:cert,PrivateKey:key, ProtocolOptions:ProtocolOptions{TUICAuthTimeout:"7s",TUICHeartbeatSec:15,TCPFastOpen:true,TCPKeepAlive:"4m",TLSHandshakeTimeout:"8s"}}
	_, files, err := s.Render(); if err != nil { t.Fatal(err) }
	var config map[string]any
	if err=json.Unmarshal(files["config.json"],&config);err!=nil{t.Fatal(err)}
	in:=config["inbounds"].([]any)[0].(map[string]any)
	for key,want:=range map[string]any{"auth_timeout":"7s","heartbeat":"15s","tcp_fast_open":true,"tcp_keep_alive":"4m"}{if in[key]!=want{t.Errorf("%s=%v want %v",key,in[key],want)}}
	tlsOptions := in["tls"].(map[string]any)
	if tlsOptions["handshake_timeout"] != "8s" { t.Fatalf("TLS options: %#v",tlsOptions) }
	if in["tcp_fast_open"] != true || in["tcp_keep_alive"] != "4m" { t.Fatalf("TCP options: %#v",in) }
}

func TestHY2GeckoAndBBRProfileRender(t *testing.T) {
	cert,key,err:=Certificate("example.com");if err!=nil{t.Fatal(err)}
	s:=Spec{Engine:"sing-box",Kind:"hysteria2",Listen:"0.0.0.0",Address:"192.0.2.2",Port:24444,Password:"fixture-password",ServerName:"example.com",TLSMode:"tls",Certificate:cert,PrivateKey:key,ProtocolOptions:ProtocolOptions{HY2ObfsMode:"gecko",HY2ObfsMinPacket:600,HY2ObfsMaxPacket:1100,HY2ServerBBRProfile:"aggressive"}}
	_,files,err:=s.Render();if err!=nil{t.Fatal(err)};var config map[string]any;if err=json.Unmarshal(files["config.json"],&config);err!=nil{t.Fatal(err)}
	in:=config["inbounds"].([]any)[0].(map[string]any);obfs:=in["obfs"].(map[string]any);if obfs["type"]!="gecko"||obfs["min_packet_size"]!=float64(600)||obfs["max_packet_size"]!=float64(1100){t.Fatalf("unexpected obfs: %#v",obfs)};if in["bbr_profile"]!="aggressive"{t.Fatalf("bbr_profile=%v",in["bbr_profile"])}
}

func TestLatestOptionsRejectUnsupportedEngineOrInvalidRanges(t *testing.T) {
	for _,options:=range []ProtocolOptions{{HY2ObfsMode:"gecko",HY2ObfsMinPacket:1201,HY2ObfsMaxPacket:1300},{HY2ServerBBRProfile:"turbo"},{TUICAuthTimeout:"0s"},{TLSHandshakeTimeout:"72h"}}{
		s:=Spec{Engine:"sing-box",Kind:"hysteria2",Listen:"0.0.0.0",Address:"192.0.2.1",Port:24445,Password:"fixture-password",ServerName:"example.com",ProtocolOptions:options};if err:=s.Validate();err==nil{t.Errorf("accepted invalid options %#v",options)}
	}
}
