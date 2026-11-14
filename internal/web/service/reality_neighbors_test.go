package service

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"
)

func TestNeighborResultRequiresResolvableCertificateName(t *testing.T) {
	r := &RealityScanResult{IP: "198.51.100.8", Port: 443, Host: "dead.example", ServerNames: []string{"dead.example", "valid.example"}, Feasible: true}
	lookup := func(ctx context.Context, name string) ([]net.IPAddr, error) {
		if name == "valid.example" {
			return []net.IPAddr{{IP: net.ParseIP("198.51.100.8")}}, nil
		}
		return nil, errors.New("no such host")
	}
	if !validateRealityNeighborNames(context.Background(), r, lookup) || r.Host != "valid.example" || r.Target != "valid.example:443" || len(r.ServerNames) != 1 {
		t.Fatalf("unexpected result: %+v", r)
	}
}

func TestRealityNeighborCancellationBoundsActiveProbes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, _, err := scanRealityNeighbors(ctx, "198.51.100.7", 5, func(ctx context.Context, _ string) *RealityScanResult {
		<-ctx.Done()
		return nil
	})
	t.Logf("scan elapsed: %s", time.Since(start))
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > time.Second {
		t.Fatalf("scan did not stop at deadline: %v, %s", err, time.Since(start))
	}
}

func TestScanRealityNeighborAddressesRejectsUnsafeBases(t *testing.T) {
	for _, base := range []string{"", "not-an-ip", "2001:db8::1", "10.0.0.7", "127.0.0.1", "169.254.169.254", "100.64.0.7"} {
		if _, err := scanRealityNeighborAddresses(base); err == nil {
			t.Errorf("scanRealityNeighborAddresses(%q) accepted an unsafe base", base)
		}
	}
}

func TestScanRealityNeighborAddressesEnumeratesOnlyTheLocal24(t *testing.T) {
	got, err := scanRealityNeighborAddresses("198.51.100.7")
	if err != nil {
		t.Fatalf("scanRealityNeighborAddresses: %v", err)
	}
	if len(got) != 253 {
		t.Fatalf("got %d addresses, want 253 usable hosts", len(got))
	}
	for _, ip := range got {
		a := netip.MustParseAddr(ip)
		if a.As4()[0] != 198 || a.As4()[1] != 51 || a.As4()[2] != 100 || a.As4()[3] == 0 || a.As4()[3] == 255 || ip == "198.51.100.7" {
			t.Fatalf("address escaped local /24 or included the base: %q", ip)
		}
	}
}

func TestScanRealityNeighborsUsesBoundedConcurrentProbesAndRanksResults(t *testing.T) {
	var mu sync.Mutex
	var probed []string
	var active, maxActive int
	probe := func(_ context.Context, ip string) *RealityScanResult {
		mu.Lock()
		active++
		if active > maxActive {
			maxActive = active
		}
		probed = append(probed, ip)
		mu.Unlock()
		mu.Lock()
		active--
		mu.Unlock()
		return &RealityScanResult{IP: ip, Target: ip + ":443", Host: "neighbor.example", Feasible: ip == "198.51.100.8" || ip == "198.51.100.9", LatencyMs: int(netip.MustParseAddr(ip).As4()[3])}
	}

	got, scanned, err := scanRealityNeighbors(context.Background(), "198.51.100.7", 1, probe)
	if err != nil {
		t.Fatalf("scanRealityNeighbors: %v", err)
	}
	if scanned != 253 || len(probed) != 253 {
		t.Fatalf("scanned=%d probes=%d, want 253 each", scanned, len(probed))
	}
	if maxActive == 0 || maxActive > realityNeighborConcurrency {
		t.Fatalf("max concurrent probes=%d, want 1..%d", maxActive, realityNeighborConcurrency)
	}
	if len(got) != 1 || got[0].IP != "198.51.100.8" {
		t.Fatalf("results=%+v, want the fastest feasible neighbor", got)
	}
}

func TestScanRealityNeighborsDoesNotProbeAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	probe := func(context.Context, string) *RealityScanResult {
		called = true
		return &RealityScanResult{}
	}
	got, scanned, err := scanRealityNeighbors(ctx, "198.51.100.7", 5, probe)
	if err == nil {
		t.Fatal("canceled scan returned nil error")
	}
	if scanned != 0 || len(got) != 0 || called {
		t.Fatalf("canceled scan produced scanned=%d results=%d called=%v", scanned, len(got), called)
	}
}
