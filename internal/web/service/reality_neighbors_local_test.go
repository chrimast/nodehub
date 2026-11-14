package service

import (
	"context"
	"testing"
)

func TestScanRealityNeighborsLocalRequiresAResolvedPublicIPv4(t *testing.T) {
	for _, base := range []string{"", "N/A", "10.0.0.1", "2001:db8::1"} {
		svc := &ServerService{cachedIPv4: base}
		if _, _, err := svc.ScanRealityNeighborsLocal(context.Background(), 1); err == nil {
			t.Errorf("cached IPv4 %q was accepted for local neighbor scan", base)
		}
	}
}
