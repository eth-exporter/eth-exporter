package collector

import "testing"

func TestFormatIPv4Hex(t *testing.T) {
	// Kernel prints ntohl(224.0.0.1) → 0x010000E0 on little-endian hosts.
	if got := formatIPv4Hex("010000E0"); got != "224.0.0.1" {
		t.Fatalf("got %q", got)
	}
	if got := formatIPv4Hex("224.0.0.22"); got != "224.0.0.22" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatMACHex(t *testing.T) {
	if got := formatMACHex("01005e000001"); got != "01:00:5e:00:00:01" {
		t.Fatalf("got %q", got)
	}
}
