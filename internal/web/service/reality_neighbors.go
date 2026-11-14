package service

import (
	"context"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/util/netsafe"
)

const realityNeighborConcurrency = 32

func scanRealityNeighborAddresses(baseIP string) ([]string, error) {
	base, err := netip.ParseAddr(strings.TrimSpace(baseIP))
	if err != nil || !base.Is4() || !base.IsGlobalUnicast() || netsafe.IsBlockedIP(net.ParseIP(base.String())) {
		return nil, common.NewError("neighbor scan requires a public IPv4 address")
	}
	prefix, _ := base.Prefix(24)
	addresses := make([]string, 0, 253)
	for ip := prefix.Addr(); prefix.Contains(ip); ip = ip.Next() {
		last := ip.As4()[3]
		if last == 0 || last == 255 || ip == base {
			continue
		}
		addresses = append(addresses, ip.String())
	}
	return addresses, nil
}

func scanRealityNeighbors(ctx context.Context, baseIP string, limit int, probe func(context.Context, string) *RealityScanResult) ([]*RealityScanResult, int, error) {
	addresses, err := scanRealityNeighborAddresses(baseIP)
	if err != nil {
		return nil, 0, err
	}
	if limit <= 0 || limit > realityScanMaxTotal {
		limit = 20
	}

	jobs := make(chan string)
	results := make(chan *RealityScanResult, len(addresses))
	workers := realityNeighborConcurrency
	if workers > len(addresses) {
		workers = len(addresses)
	}
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ip := range jobs {
				if ctx.Err() != nil {
					return
				}
				if result := probe(ctx, ip); result != nil && result.Feasible {
					results <- result
				}
			}
		}()
	}

	scanned := 0
	for _, ip := range addresses {
		if ctx.Err() != nil {
			break
		}
		select {
		case jobs <- ip:
			scanned++
		case <-ctx.Done():
			goto dispatched
		}
		if ctx.Err() != nil {
			break
		}
	}

dispatched:
	close(jobs)
	wg.Wait()
	close(results)

	found := make([]*RealityScanResult, 0, len(results))
	for result := range results {
		found = append(found, result)
	}
	sortRealityResults(found)
	if len(found) > limit {
		found = found[:limit]
	}
	if err := ctx.Err(); err != nil {
		return found, scanned, err
	}
	return found, scanned, nil
}

// ScanRealityNeighbors scans only the node's public /24 and never accepts a
// private, shared, loopback, link-local, multicast, or IPv6 base address.
func (s *ServerService) ScanRealityNeighbors(ctx context.Context, baseIP string, limit int) ([]*RealityScanResult, int, error) {
	return scanRealityNeighbors(ctx, baseIP, limit, func(probeCtx context.Context, ip string) *RealityScanResult {
		probeCtx, cancel := context.WithTimeout(probeCtx, realityDiscoverTimeout)
		defer cancel()
		result := s.probeRealityAddrWithContext(probeCtx, ip, 443, "", realityDiscoverTimeout, false)
		if result.Feasible && !validateRealityNeighborNames(probeCtx, result, net.DefaultResolver.LookupIPAddr) {
			return nil
		}
		return result
	})
}

func validateRealityNeighborNames(ctx context.Context, result *RealityScanResult, lookup func(context.Context, string) ([]net.IPAddr, error)) bool {
	var valid []string
	for _, name := range result.ServerNames {
		if ctx.Err() != nil {
			break
		}
		ips, err := lookup(ctx, name)
		if err != nil {
			continue
		}
		for _, addr := range ips {
			if addr.IP.Equal(net.ParseIP(result.IP)) {
				valid = append(valid, name)
				break
			}
		}
	}
	result.ServerNames = valid
	if len(valid) == 0 {
		return false
	}
	result.Host = valid[0]
	result.Target = net.JoinHostPort(valid[0], strconv.Itoa(result.Port))
	return true
}

func (s *ServerService) probeRealityAddrWithContext(ctx context.Context, dialHost string, port int, sni string, timeout time.Duration, allowPrivate bool) *RealityScanResult {
	return s.probeRealityAddrContext(ctx, dialHost, port, sni, timeout, 0, allowPrivate)
}
