package main

import "testing"

func TestUnboundListen(t *testing.T) {
	cases := []struct {
		addr string
		want bool
	}{
		{"127.0.0.1:9417", false},
		{"localhost:9417", false},
		{"[::1]:9417", false},
		{":9417", true},
		{"0.0.0.0:9417", true},
		{"[::]:9417", true},
		{"192.168.1.10:9417", true},
	}
	for _, tc := range cases {
		if got := unboundListen(tc.addr); got != tc.want {
			t.Fatalf("unboundListen(%q)=%v want %v", tc.addr, got, tc.want)
		}
	}
}
