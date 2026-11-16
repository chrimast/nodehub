package api

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"sort"
	"strings"
	"sync"
	"time"
)

const maxDomainScanHosts = 256
const maxDomainScanResults = 64
const scanWorkers = 32

var domainScanDialContext = (&net.Dialer{Timeout: 1200 * time.Millisecond}).DialContext

func isDocumentationIP(ip netip.Addr) bool {
	for _, prefix := range []string{"192.0.2.0/24", "198.51.100.0/24", "203.0.113.0/24", "2001:db8::/32"} {
		rangePrefix, _ := netip.ParsePrefix(prefix)
		if rangePrefix.Contains(ip) { return true }
	}
	return false
}

func scanAddresses(prefix netip.Prefix) ([]netip.Addr, error) {
	if !prefix.IsValid() || prefix.Addr().Is4In6() || isDocumentationIP(prefix.Addr()) { return nil, fmt.Errorf("请输入有效的 IPv4 或 IPv6 网段") }
	masked := prefix.Masked()
	if !masked.Addr().IsGlobalUnicast() || masked.Addr().IsPrivate() || masked.Addr().IsLoopback() || masked.Addr().IsLinkLocalUnicast() || masked.Addr().IsMulticast() || masked.Addr().IsUnspecified() || isDocumentationIP(masked.Addr()) || !masked.Addr().IsGlobalUnicast() { return nil, fmt.Errorf("仅允许扫描公网 IP 网段") }
	prefix = prefix.Masked()
	hostBits := prefix.Addr().BitLen() - prefix.Bits()
	if hostBits > 8 { return nil, fmt.Errorf("网段地址数超过上限，最多扫描 %d 个地址", maxDomainScanHosts) }
	count := 1 << uint(hostBits)
	ips := make([]netip.Addr, 0, count)
	for i := 0; i < count; i++ {
		ip := prefix.Addr()
		for n := count-i-1; n > 0; n-- { ip = ip.Next() }
		if ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsUnspecified() && !ip.IsMulticast() { ips = append(ips, ip) }
	}
	if len(ips) == 0 { return nil, fmt.Errorf("网段内没有可扫描的公网地址") }
	return ips, nil
}

func scanTLSNames(ctx context.Context, prefix netip.Prefix) []string {
	ips, err := scanAddresses(prefix)
	if err != nil { return nil }
	jobs := make(chan netip.Addr)
	found := make(chan string, maxDomainScanResults*2)
	var wg sync.WaitGroup
	for i := 0; i < scanWorkers; i++ { wg.Add(1); go func() { defer wg.Done(); for ip := range jobs { scanTLSNamesAtIP(ctx, ip, found) } }() }
	go func() { defer close(jobs); for _, ip := range ips { select { case jobs <- ip: case <-ctx.Done(): return } } }()
	go func() { wg.Wait(); close(found) }()
	set := map[string]bool{}
	for name := range found { if !set[name] { set[name] = true } }
	out := make([]string, 0, len(set)); for name := range set { out = append(out, name) }; sort.Strings(out)
	if len(out) > maxDomainScanResults { out = out[:maxDomainScanResults] }
	return out
}

func scanTLSNamesAtIP(ctx context.Context, ip netip.Addr, found chan<- string) {
	ctx, cancel := context.WithTimeout(ctx, 1600*time.Millisecond); defer cancel()
	conn, err := domainScanDialContext(ctx, "tcp", net.JoinHostPort(ip.String(), "443")); if err != nil { return }; defer conn.Close()
	tlsConn := tls.Client(conn, &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12, NextProtos: []string{"h2", "http/1.1"}})
	if err = tlsConn.HandshakeContext(ctx); err != nil { return }
	state := tlsConn.ConnectionState(); if len(state.PeerCertificates) == 0 { return }
	cert := state.PeerCertificates[0]
	names := append([]string{cert.Subject.CommonName}, cert.DNSNames...)
	if len(names) == 0 { names = append(names, ip.String()) }
	seen := map[string]bool{}
	for _, raw := range names {
		name := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(raw)), "*.")
		if name == "" || seen[name] || strings.Contains(name, "*") || len(name) > 253 { continue }; seen[name] = true
		select { case found <- name: default: return }
	}
}

func (s *Server) realityDomainScan(w http.ResponseWriter, r *http.Request) error {
	var body struct { Prefix string `json:"prefix"` }
	if err := decode(w, r, &body); err != nil { return err }
	prefix, err := netip.ParsePrefix(body.Prefix)
	if err != nil { return bad("请输入有效的 IP 网段，例如 203.0.113.0/24") }
	if _, err = scanAddresses(prefix); err != nil { return bad(err.Error()) }
	send(w, http.StatusOK, map[string]any{"domains": scanTLSNames(r.Context(), prefix)})
	return nil
}
