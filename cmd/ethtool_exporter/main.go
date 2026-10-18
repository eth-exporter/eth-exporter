// Command ethtool_exporter exposes Linux network interface, ethtool, and
// discovery-protocol metrics for Prometheus / Grafana Alloy.
//
// All data is read from the kernel via sysfs, ethtool ioctl, rtnetlink-style
// interface enumeration, and AF_PACKET (LLDP/CDP). No ethtool CLI, lldpd,
// iproute2, or other host tooling is required at runtime.
package main

import (
	"flag"
	"fmt"
	"html"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/exporter-toolkit/web"

	"github.com/scottmo/eth-exporter/internal/collector"
	"github.com/scottmo/eth-exporter/internal/neighbor"
)

func main() {
	var (
		listenAddr     = flag.String("web.listen-address", "127.0.0.1:9417", "Address to listen on for metrics (default loopback-only)")
		metricsPath    = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics")
		webConfigFile  = flag.String("web.config.file", "", "Prometheus exporter TLS/basic-auth config file (see systemd/web-config.example.yml)")
		disableLanding = flag.Bool("web.disable-landing-page", false, "Disable the HTML index page (metrics path only)")
		maxInFlight    = flag.Int("web.max-requests-in-flight", 1, "Max concurrent /metrics scrapes (0 = unlimited)")
		sysPath        = flag.String("path.sysfs", "/sys", "sysfs mount point (absolute)")
		procPath       = flag.String("path.procfs", "/proc", "procfs mount point (absolute)")
		deviceInclude  = flag.String("collector.device-include", "", "Regexp of interfaces to include (exclusive with exclude)")
		deviceExclude  = flag.String("collector.device-exclude", "^(lo|docker.*|veth.*|br-.*|virbr.*)$", "Regexp of interfaces to exclude")
		statsInclude   = flag.String("collector.ethtool.metrics-include", ".*", "Regexp of ethtool driver stat names to include")
		enableNeighbor = flag.Bool("collector.neighbor", true, "Enable passive LLDP/CDP neighbor discovery via AF_PACKET")
		neighborTTL    = flag.Duration("collector.neighbor.stale", 10*time.Minute, "Cache neighbors at least this long after last PDU (also respects PDU TTL)")
		neighborHold   = flag.Duration("collector.neighbor.active-hold", 3*time.Minute, "Keep continuous capture after last discovery PDU")
		neighborProbe  = flag.Duration("collector.neighbor.probe-window", 35*time.Second, "Listen window when probing quiet interfaces (must exceed typical LLDP 30s interval)")
		neighborMin    = flag.Duration("collector.neighbor.probe-min", 1*time.Minute, "Minimum backoff between probes on quiet interfaces")
		neighborMax    = flag.Duration("collector.neighbor.probe-max", 15*time.Minute, "Maximum backoff for interfaces that never advertise LLDP/CDP")
		neighborCont   = flag.Int("collector.neighbor.max-continuous", 16, "Max simultaneous continuous AF_PACKET listeners")
		neighborRedact = flag.Bool("collector.neighbor.redact", false, "Omit free-text / management identity labels from neighbor metrics")
		workers        = flag.Int("collector.workers", 4, "Parallel per-device scrape workers")
		opticsCache    = flag.Duration("collector.optics.cache", 60*time.Second, "Cache module DOM / EEPROM reads")
		negCache       = flag.Duration("collector.neg-cache", 5*time.Minute, "Cache unsupported probe failures")
		nftCache       = flag.Duration("collector.nftables.cache", 15*time.Second, "Cache nftables dumps across scrapes")
		maxL3Neigh     = flag.Int("collector.l3-neigh.max", 4096, "Max ARP/ND info series per scrape (-1 unlimited)")
		maxFDB         = flag.Int("collector.fdb.max", 8192, "Max FDB info series per scrape (-1 unlimited)")

		// Expensive optional collectors (default on; disable to reduce scrape cost).
		enOptics    = flag.Bool("collector.optics", true, "Collect module EEPROM / DOM (cached)")
		enStdStats  = flag.Bool("collector.std-stats", true, "Collect genetlink IEEE/RMON standard stats")
		enLinkModes = flag.Bool("collector.link-modes", true, "Collect supported/advertised/partner link modes")
		enFeatures  = flag.Bool("collector.features", true, "Collect ethtool offload feature bits")
		enPrivFlags = flag.Bool("collector.priv-flags", true, "Collect private flags")
		enRSS       = flag.Bool("collector.rss", true, "Collect RSS info")
		enPauseFEC  = flag.Bool("collector.pause-fec-stats", true, "Collect genetlink pause/FEC counters")
		enAddrs     = flag.Bool("collector.addresses", true, "Collect interface IP addresses")
		enL3Neigh   = flag.Bool("collector.l3-neigh", true, "Collect ARP/ND neighbor table")
		enFDB       = flag.Bool("collector.fdb", true, "Collect bridge FDB")
		enQdisc     = flag.Bool("collector.qdisc", true, "Collect TC qdisc stats")
		enNft       = flag.Bool("collector.nftables", true, "Collect nftables tables/chains/counters")
		enRDMA      = flag.Bool("collector.rdma", true, "Collect RDMA/IB sysfs counters")
		enConntrack = flag.Bool("collector.conntrack", true, "Collect nf_conntrack stats")
		enMcast     = flag.Bool("collector.multicast", true, "Collect IGMP/MLD/dev_mcast")
		enSoftnet   = flag.Bool("collector.softnet", true, "Collect softnet stats")
		enStack     = flag.Bool("collector.stack", true, "Collect /proc/net/snmp+netstat")

		logLevel = flag.String("log.level", "info", "Log level: debug, info, warn, error")
	)
	flag.Parse()

	level := new(slog.LevelVar)
	switch *logLevel {
	case "debug":
		level.Set(slog.LevelDebug)
	case "warn":
		level.Set(slog.LevelWarn)
	case "error":
		level.Set(slog.LevelError)
	default:
		level.Set(slog.LevelInfo)
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	sysFS, err := absRoot(*sysPath)
	if err != nil {
		logger.Error("invalid path.sysfs", "err", err)
		os.Exit(2)
	}
	procFS, err := absRoot(*procPath)
	if err != nil {
		logger.Error("invalid path.procfs", "err", err)
		os.Exit(2)
	}
	if !strings.HasPrefix(*metricsPath, "/") || strings.Contains(*metricsPath, "..") {
		logger.Error("invalid web.telemetry-path", "path", *metricsPath)
		os.Exit(2)
	}

	includeRE, err := compileOpt(*deviceInclude)
	if err != nil {
		logger.Error("invalid device-include", "err", err)
		os.Exit(2)
	}
	excludeRE, err := compileOpt(*deviceExclude)
	if err != nil {
		logger.Error("invalid device-exclude", "err", err)
		os.Exit(2)
	}
	if includeRE != nil && excludeRE != nil {
		logger.Error("device-include and device-exclude are mutually exclusive")
		os.Exit(2)
	}
	statsRE, err := regexp.Compile(*statsInclude)
	if err != nil {
		logger.Error("invalid ethtool.metrics-include", "err", err)
		os.Exit(2)
	}

	filter := collector.DeviceFilter{Include: includeRE, Exclude: excludeRE}

	var neighStore *neighbor.Store
	if *enableNeighbor {
		if !neighbor.NeighborEnabled {
			logger.Warn("neighbor capture unavailable (built with -tags noneighbor)")
		} else {
			neighCfg := neighbor.Config{
				Stale:         *neighborTTL,
				ActiveHold:    *neighborHold,
				ProbeWindow:   *neighborProbe,
				ProbeMin:      *neighborMin,
				ProbeMax:      *neighborMax,
				MaxContinuous: *neighborCont,
			}
			neighStore = neighbor.NewStore(neighCfg.Stale)
			cap, err := neighbor.NewCapture(logger, filter, neighStore, neighCfg)
			if err != nil {
				logger.Warn("neighbor capture disabled", "err", err)
				neighStore = nil
			} else {
				go cap.Run()
				defer cap.Close()
				logger.Info("neighbor capture started",
					"stale", neighCfg.Stale,
					"active_hold", neighCfg.ActiveHold,
					"probe_window", neighCfg.ProbeWindow,
					"probe_min", neighCfg.ProbeMin,
					"probe_max", neighCfg.ProbeMax,
					"max_continuous", neighCfg.MaxContinuous,
					"filter", "bpf-dst-mac")
			}
		}
	}

	feat := collector.Features{
		Optics: *enOptics, StdStats: *enStdStats, LinkModes: *enLinkModes, Features: *enFeatures,
		PrivFlags: *enPrivFlags, RSS: *enRSS, PauseFEC: *enPauseFEC,
		Addresses: *enAddrs, L3Neigh: *enL3Neigh, FDB: *enFDB, Qdisc: *enQdisc,
		Nftables: *enNft, RDMA: *enRDMA, Conntrack: *enConntrack, Multicast: *enMcast,
		Softnet: *enSoftnet, StackSNMP: *enStack,
	}

	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewBuildInfoCollector(),
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	c, err := collector.New(collector.Options{
		SysFS:          sysFS,
		ProcFS:         procFS,
		Filter:         filter,
		StatsInclude:   statsRE,
		NeighborStore:  neighStore,
		NeighborRedact: *neighborRedact,
		Logger:         logger,
		Features:       &feat,
		Perf: collector.PerfOptions{
			Workers:        *workers,
			OpticsCacheTTL: *opticsCache,
			NegCacheTTL:    *negCache,
			NftCacheTTL:    *nftCache,
			MaxL3Neigh:     *maxL3Neigh,
			MaxFDB:         *maxFDB,
		},
	})
	if err != nil {
		logger.Error("failed to create collector", "err", err)
		os.Exit(1)
	}
	reg.MustRegister(c)

	mux := http.NewServeMux()
	mux.Handle(*metricsPath, promhttp.HandlerFor(reg, promhttp.HandlerOpts{
		EnableOpenMetrics:   true,
		ErrorLog:            slog.NewLogLogger(logger.Handler(), slog.LevelError),
		MaxRequestsInFlight: *maxInFlight,
		Timeout:             90 * time.Second,
	}))
	if !*disableLanding {
		safePath := html.EscapeString(*metricsPath)
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/" {
				http.NotFound(w, r)
				return
			}
			_, _ = fmt.Fprintf(w, `<html><head><title>ethtool exporter</title></head>
<body><h1>ethtool exporter</h1><p><a href="%s">Metrics</a></p></body></html>`, safePath)
		})
	}

	if unboundListen(*listenAddr) && *webConfigFile == "" {
		logger.Warn("metrics listen address is not loopback-only and web.config.file is unset; prefer 127.0.0.1 or enable TLS/basic auth",
			"listen", *listenAddr)
	}

	listenAddrs := []string{*listenAddr}
	systemdSocket := false
	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		// Scrapes can include DOM / many devices; keep generous but bounded.
		WriteTimeout:   2 * time.Minute,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 8 << 10,
	}

	logger.Info("starting ethtool exporter",
		"listen", *listenAddr,
		"path", *metricsPath,
		"workers", *workers,
		"max_in_flight", *maxInFlight,
		"web_config", *webConfigFile != "")
	if err := web.ListenAndServe(server, &web.FlagConfig{
		WebListenAddresses: &listenAddrs,
		WebSystemdSocket:   &systemdSocket,
		WebConfigFile:      webConfigFile,
	}, logger); err != nil {
		logger.Error("http server failed", "err", err)
		os.Exit(1)
	}
}

func compileOpt(s string) (*regexp.Regexp, error) {
	if s == "" {
		return nil, nil
	}
	return regexp.Compile(s)
}

func absRoot(p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("empty path")
	}
	clean := filepath.Clean(p)
	if !filepath.IsAbs(clean) {
		return "", fmt.Errorf("must be absolute: %q", p)
	}
	return clean, nil
}

// unboundListen reports whether addr likely accepts remote connections.
func unboundListen(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// Bare ":9417" form.
		return strings.HasPrefix(addr, ":") || addr == ""
	}
	switch strings.ToLower(host) {
	case "", "0.0.0.0", "::", "[::]":
		return true
	case "localhost", "127.0.0.1", "::1", "[::1]":
		return false
	default:
		ip := net.ParseIP(strings.Trim(host, "[]"))
		if ip != nil {
			return !ip.IsLoopback()
		}
		return true
	}
}
