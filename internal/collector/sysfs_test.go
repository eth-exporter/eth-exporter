package collector

import "testing"

func TestValidIfaceName(t *testing.T) {
	ok := []string{"eth0", "enp1s0", "bond0", "wlan0"}
	bad := []string{"", ".", "..", "a/b", "eth0\x00", "thisnameiswaytoolong"}
	for _, n := range ok {
		if !validIfaceName(n) {
			t.Fatalf("expected valid %q", n)
		}
	}
	for _, n := range bad {
		if validIfaceName(n) {
			t.Fatalf("expected invalid %q", n)
		}
	}
}
