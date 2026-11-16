package api

import (
	"net/netip"
	"testing"
)

func TestScanAddressesRejectsBroadAndPrivatePrefixes(t *testing.T) {
	for _, value := range []string{"0.0.0.0/0", "10.0.0.0/24", "192.168.1.0/24", "2001:db8::/64", "198.51.100.0/24", "203.0.113.0/24"} {
		prefix, _ := netip.ParsePrefix(value)
		if _, err := scanAddresses(prefix); err == nil { t.Errorf("expected %s to be rejected", value) }
	}
}

func TestScanAddressesAcceptsSmallPublicPrefix(t *testing.T) {
	for _, item := range []struct{ prefix string; want int }{{"8.8.8.0/29", 8}, {"8.8.8.0/24", 256}} {
		prefix, _ := netip.ParsePrefix(item.prefix)
		addresses, err := scanAddresses(prefix)
		if err != nil { t.Fatal(err) }
		if len(addresses) != item.want { t.Fatalf("%s: got %d addresses", item.prefix, len(addresses)) }
	}
}
