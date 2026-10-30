package collector

import (
	"testing"
	"unicode/utf8"

	"github.com/prometheus/client_golang/prometheus"
)

// Regression: AMD Pollara QSFP112 EEPROM returned vendor bytes "\x80$" which
// made MustNewConstMetric panic and crashed the exporter on every scrape; and
// LLDP port IDs with binary subtypes collapsed to the same label (duplicate
// series -> whole scrape failed).
func TestUTF8Label(t *testing.T) {
	cases := map[string]string{
		"\x80$":             "0x8024",
		"AMD\x00\x00 ":      "AMD",
		"  FINISAR CORP.  ": "FINISAR CORP.",
		"ok":                "ok",
		"Z\x01":             "0x5a01",
		"Z\x02":             "0x5a02",
		"":                  "",
	}
	for in, want := range cases {
		got := utf8Label(in)
		if got != want || !utf8.ValidString(got) {
			t.Fatalf("utf8Label(%q)=%q want %q", in, got, want)
		}
	}
	d := prometheus.NewDesc("x", "x", []string{"vendor"}, nil)
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("panicked: %v", r)
			}
		}()
		prometheus.MustNewConstMetric(d, prometheus.GaugeValue, 1, utf8Label("\x80$"))
	}()
	if s := truncate("ab\xc3\xa9cd", 3); !utf8.ValidString(s) {
		t.Fatalf("truncate split rune: %q", s)
	}
}
