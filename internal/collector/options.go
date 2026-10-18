package collector

import "time"

// Features toggles expensive optional collectors.
type Features struct {
	Optics    bool // module EEPROM / DOM (cached)
	StdStats  bool // genetlink IEEE/RMON groups
	LinkModes bool // supported/advertised/partner bitsets
	Features  bool // ethtool offload feature bits
	PrivFlags bool
	RSS       bool
	PauseFEC  bool // genetlink pause/fec counters
	Addresses bool
	L3Neigh   bool // ARP/ND table
	FDB       bool
	Qdisc     bool
	Nftables  bool
	RDMA      bool
	Conntrack bool
	Multicast bool
	Softnet   bool
	StackSNMP bool
}

// DefaultFeatures enables the full set.
func DefaultFeatures() Features {
	return Features{
		Optics: true, StdStats: true, LinkModes: true, Features: true,
		PrivFlags: true, RSS: true, PauseFEC: true,
		Addresses: true, L3Neigh: true, FDB: true, Qdisc: true,
		Nftables: true, RDMA: true, Conntrack: true, Multicast: true,
		Softnet: true, StackSNMP: true,
	}
}

// PerfOptions controls scrape concurrency and caches.
type PerfOptions struct {
	Workers        int           // parallel per-device scrapes; ≤0 → 4
	OpticsCacheTTL time.Duration // default 60s
	NegCacheTTL    time.Duration // unsupported probe cache; default 5m
	NftCacheTTL    time.Duration // nftables dump cache; default 15s; ≤0 keeps default
	MaxL3Neigh     int           // max ARP/ND info series per scrape; ≤0 → 4096; <0 unlimited
	MaxFDB         int           // max FDB info series per scrape; ≤0 → 8192; <0 unlimited
}
