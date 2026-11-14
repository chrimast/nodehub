package service

import (
	"context"
	"errors"
)

// ScanRealityNeighborsLocal uses the node's already-known public IPv4. It does
// not infer an address from interfaces or accept an arbitrary scan base.
func (s *ServerService) ScanRealityNeighborsLocal(ctx context.Context, limit int) ([]*RealityScanResult, int, error) {
	s.mu.Lock()
	base := s.cachedIPv4
	s.mu.Unlock()
	if base == "" || base == "N/A" {
		return nil, 0, errors.New("local public IPv4 is not available")
	}
	return s.ScanRealityNeighbors(ctx, base, limit)
}
