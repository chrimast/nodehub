package service

import "testing"

func TestLocalRealityScanMarkerIsReserved(t *testing.T) {
	if !isLocalRealityScanMarker("@local24") {
		t.Fatal("@local24 must select the node's own /24")
	}
	for _, value := range []string{"", "local24", "@local24:443", "198.51.100.0/24"} {
		if isLocalRealityScanMarker(value) {
			t.Fatalf("%q must not select the node's local scan", value)
		}
	}
}
