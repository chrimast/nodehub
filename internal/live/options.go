package live

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

// ProtocolOptions are a bounded, versioned subset of native engine options.
// Embedding keeps API, encrypted storage and Agent wire fields identical.
type ProtocolOptions struct {
	HY2ClientMode     string `json:"hy2_client_mode,omitempty"`
	HY2ClientUpMbps   int    `json:"hy2_client_up_mbps,omitempty"`
	HY2ClientDownMbps int    `json:"hy2_client_down_mbps,omitempty"`
	HY2BBRProfile     string `json:"hy2_bbr_profile,omitempty"`
	TUICRelayMode     string `json:"tuic_udp_relay_mode,omitempty"`
	TUICHeartbeat     int    `json:"tuic_heartbeat_seconds,omitempty"`
	AnyTLSIdleCheck   int    `json:"anytls_idle_check_seconds,omitempty"`
	AnyTLSIdleTimeout int    `json:"anytls_idle_timeout_seconds,omitempty"`
	AnyTLSMinIdle     int    `json:"anytls_min_idle_sessions,omitempty"`
	HopPorts          string `json:"hop_ports,omitempty"`
	HopInterval       int    `json:"hop_interval_seconds,omitempty"`
	ListenNetwork     string `json:"listen_network,omitempty"`
	Cipher            string `json:"cipher,omitempty"`
	SnellVersion      int    `json:"snell_version,omitempty"`
	SnellObfs         string `json:"snell_obfs,omitempty"`
	SnellHost         string `json:"snell_host,omitempty"`
	SnellMode         string `json:"snell_mode,omitempty"`
	HY2Obfs           string `json:"hy2_obfs_password,omitempty"`
	HysteriaObfs      string `json:"hysteria_obfs_password,omitempty"`
	UpMbps            int    `json:"up_mbps,omitempty"`
	DownMbps          int    `json:"down_mbps,omitempty"`
	HY2ObfsMode       string `json:"hy2_obfs_mode,omitempty"`
	HY2ObfsMinPacket  int    `json:"hy2_obfs_min_packet_size,omitempty"`
	HY2ObfsMaxPacket  int    `json:"hy2_obfs_max_packet_size,omitempty"`
	HY2ServerBBRProfile string `json:"hy2_server_bbr_profile,omitempty"`
	TUICAuthTimeout   string `json:"tuic_auth_timeout,omitempty"`
	TUICHeartbeatSec  int    `json:"tuic_server_heartbeat_seconds,omitempty"`
	TCPFastOpen       bool   `json:"tcp_fast_open,omitempty"`
	TCPMultiPath      bool   `json:"tcp_multi_path,omitempty"`
	TCPKeepAlive      string `json:"tcp_keep_alive,omitempty"`
	TLSHandshakeTimeout string `json:"tls_handshake_timeout,omitempty"`
}

func (s Spec) snellVersion() int {
	if s.SnellVersion == 6 {
		return 6
	}
	return 5
}
func (s Spec) RequiresOptions() bool {
	return (s.Engine == "xray" && s.Kind == "shadowsocks") || s.ProtocolOptions != (ProtocolOptions{}) || (s.Outbound != nil && s.Outbound.ProtocolOptions != (ProtocolOptions{}))
}
func MeterCapable(version string) bool {
	if !ProtocolCapable(version) {
		return false
	}
	p := strings.Split(version, ".")
	major, _ := strconv.Atoi(p[0])
	minor, _ := strconv.Atoi(p[1])
	return major > 0 || minor >= 10
}
func OptionsCapable(version string) bool {
	if !ProtocolCapable(version) {
		return false
	}
	p := strings.Split(version, ".")
	major, _ := strconv.Atoi(p[0])
	minor, _ := strconv.Atoi(p[1])
	return major > 0 || minor >= 9
}
func (s Spec) validateOptions() error {
	if err := s.validateLatestOptions(); err != nil { return err }
	if err := s.validateClientTuning(); err != nil {
		return err
	}
	if err := s.validateHopping(); err != nil {
		return err
	}
	if s.ListenNetwork != "" && (s.Kind != "shadowsocks" || s.ListenNetwork != "tcp" && s.ListenNetwork != "udp" && s.ListenNetwork != "tcp,udp") {
		return errors.New("native listen_network is only supported by Shadowsocks: tcp, udp or tcp,udp")
	}
	if s.HysteriaObfs != "" && (s.Kind != "hysteria" || len(s.HysteriaObfs) < 8 || len(s.HysteriaObfs) > 256 || strings.ContainsAny(s.HysteriaObfs, "\r\n\x00")) {
		return errors.New("Hysteria v1 obfuscation requires 8-256 bytes and its own protocol")
	}
	if s.HY2Obfs != "" && s.Kind != "hysteria2" {
		return errors.New("Salamander requires Hysteria2; Hysteria v1 uses hysteria_obfs_password")
	}
	if s.Cipher != "" && s.Kind != "shadowsocks" {
		return errors.New("cipher selection requires Shadowsocks")
	}
	if s.Kind == "shadowsocks" && s.SSMethod() != Method && s.SSMethod() != "2022-blake3-aes-128-gcm" && s.SSMethod() != "2022-blake3-chacha20-poly1305" && s.SSMethod() != "aes-128-gcm" && s.SSMethod() != "aes-256-gcm" && s.SSMethod() != "chacha20-ietf-poly1305" {
		return errors.New("unsupported Shadowsocks cipher")
	}
	if s.Kind != "snell" && (s.SnellVersion != 0 || s.SnellObfs != "" || s.SnellHost != "" || s.SnellMode != "") {
		return errors.New("Snell options require Snell protocol")
	}
	if s.Kind == "snell" {
		if s.SnellVersion != 0 && s.SnellVersion != 5 && s.SnellVersion != 6 {
			return errors.New("Snell server version must be 5 (v4 compatible) or 6")
		}
		if s.SnellObfs != "" && s.SnellObfs != "none" && s.SnellObfs != "http" {
			return errors.New("Snell v5 supports none or http obfuscation")
		}
		if s.SnellHost != "" && (s.SnellObfs != "http" || !Address(s.SnellHost)) {
			return errors.New("Snell HTTP obfuscation host must be a valid host")
		}
		if s.snellVersion() == 6 {
			if s.SnellObfs != "" || s.SnellHost != "" || len(s.Password) < 12 || len(s.Password) > 255 {
				return errors.New("Snell v6 requires 12-255 byte PSK and no HTTP obfuscation")
			}
			if s.SnellMode != "" && s.SnellMode != "default" && s.SnellMode != "unshaped" {
				return errors.New("Snell v6 mode must be default or unshaped")
			}
		} else if s.SnellMode != "" {
			return errors.New("traffic shaping mode requires Snell v6")
		}
	}
	if s.Kind != "hysteria2" && s.Kind != "hysteria" && (s.HY2Obfs != "" || s.UpMbps != 0 || s.DownMbps != 0) {
		return errors.New("bandwidth and salamander options require Hysteria2")
	}
	if s.UpMbps < 0 || s.DownMbps < 0 || s.UpMbps > 100000 || s.DownMbps > 100000 {
		return errors.New("bandwidth must be 0-100000 Mbps")
	}
	if s.HY2Obfs != "" && (len(s.HY2Obfs) < 8 || len(s.HY2Obfs) > 256) {
		return errors.New("Hysteria2 salamander password must contain 8-256 bytes")
	}
	return nil
}
func (s Spec) validateLatestOptions() error {
	o:=s.ProtocolOptions
	if s.Engine == "xray" && (o.HY2ObfsMode!=""||o.HY2ObfsMinPacket!=0||o.HY2ObfsMaxPacket!=0||o.HY2ServerBBRProfile!=""||o.TUICAuthTimeout!=""||o.TUICHeartbeatSec!=0||o.TCPFastOpen||o.TCPMultiPath||o.TCPKeepAlive!=""||o.TLSHandshakeTimeout!="") { return errors.New("latest native options in this group require sing-box") }
	if s.Kind!="hysteria2"&&(o.HY2ObfsMode!=""||o.HY2ObfsMinPacket!=0||o.HY2ObfsMaxPacket!=0||o.HY2ServerBBRProfile!="") { return errors.New("HY2 server options require Hysteria2") }
	if o.HY2ObfsMode!=""&&o.HY2ObfsMode!="salamander"&&o.HY2ObfsMode!="gecko" { return errors.New("HY2 obfuscation must be salamander or gecko") }
	if o.HY2ObfsMode=="salamander"&&s.HY2Obfs==""||o.HY2ObfsMode=="gecko"&&s.HY2Obfs!="" { return errors.New("HY2 obfuscation mode and password do not match") }
	if o.HY2ObfsMode=="gecko"&&(o.HY2ObfsMinPacket<1||o.HY2ObfsMinPacket>1200||o.HY2ObfsMaxPacket<o.HY2ObfsMinPacket||o.HY2ObfsMaxPacket>1200) { return errors.New("Gecko packet size must be 1-1200 and min must not exceed max") }
	if o.HY2ObfsMode!="gecko"&&(o.HY2ObfsMinPacket!=0||o.HY2ObfsMaxPacket!=0) { return errors.New("HY2 packet size options require Gecko obfuscation") }
	if o.HY2ServerBBRProfile!=""&&o.HY2ServerBBRProfile!="conservative"&&o.HY2ServerBBRProfile!="standard"&&o.HY2ServerBBRProfile!="aggressive" { return errors.New("unsupported HY2 BBR profile") }
	if s.Kind!="tuic"&&(o.TUICAuthTimeout!=""||o.TUICHeartbeatSec!=0) { return errors.New("TUIC server options require TUIC") }
	if o.TUICAuthTimeout!="" { d,e:=time.ParseDuration(o.TUICAuthTimeout);if e!=nil||d<time.Second||d>time.Minute{return errors.New("TUIC auth timeout must be 1s-1m")} }
	if o.TUICHeartbeatSec<0||o.TUICHeartbeatSec>300{return errors.New("TUIC heartbeat must be 0-300 seconds")}
	if s.Transport=="quic"&&(o.TCPFastOpen||o.TCPMultiPath||o.TCPKeepAlive!=""){return errors.New("TCP listener options cannot be used with QUIC transport")}
	if o.TCPKeepAlive!=""{d,e:=time.ParseDuration(o.TCPKeepAlive);if e!=nil||d<time.Second||d>24*time.Hour{return errors.New("TCP keepalive must be 1s-24h")}}
	if o.TLSHandshakeTimeout!=""{d,e:=time.ParseDuration(o.TLSHandshakeTimeout);if e!=nil||d<time.Second||d>time.Minute{return errors.New("TLS handshake timeout must be 1s-1m")}}
	return nil
}

func (s Spec) SSMethod() string {
	if s.Cipher != "" {
		return s.Cipher
	}
	return Method
}
