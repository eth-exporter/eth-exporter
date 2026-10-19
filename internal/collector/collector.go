package collector

import (
	"log/slog"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/eth-exporter/eth-exporter/internal/ethtool"
	"github.com/eth-exporter/eth-exporter/internal/neighbor"
	"github.com/eth-exporter/eth-exporter/internal/netfilter"
	rtn "github.com/eth-exporter/eth-exporter/internal/netlink"
)

const namespace = "ethtool"

// Options configures the collector.
type Options struct {
	SysFS          string
	ProcFS         string
	Filter         DeviceFilter
	StatsInclude   *regexp.Regexp
	NeighborStore  *neighbor.Store
	NeighborRedact bool // strip free-text / management identity from neighbor labels
	Logger         *slog.Logger
	Features       *Features // nil → DefaultFeatures()
	Perf           PerfOptions
}

// Collector implements prometheus.Collector.
type Collector struct {
	sysfs          string
	procfs         string
	filter         DeviceFilter
	statsInclude   *regexp.Regexp
	neighbors      *neighbor.Store
	neighborRedact bool
	logger         *slog.Logger
	eth            *ethtool.Client
	nl             *ethtool.Netlink
	rtn            *rtn.Client
	nft            *netfilter.Client
	extra          ExtraDescs
	feat           Features
	workers        int
	domCache       *domCache
	nftCache       *ttlCache[netfilter.Snapshot]
	maxL3Neigh     int
	maxFDB         int
	scrapeStage    *prometheus.Desc

	ifaceInfo           *prometheus.Desc
	ifaceUp             *prometheus.Desc
	ifaceCarrier        *prometheus.Desc
	ifaceMTU            *prometheus.Desc
	ifaceSpeed          *prometheus.Desc
	ifaceStat           *prometheus.Desc
	ifaceCarrierChanges *prometheus.Desc

	drvInfo     *prometheus.Desc
	linkInfo    *prometheus.Desc
	linkUp      *prometheus.Desc
	linkSpeed   *prometheus.Desc
	linkAutoneg *prometheus.Desc
	drvStat     *prometheus.Desc
	phyStat     *prometheus.Desc
	stdStat     *prometheus.Desc
	rmonHist    *prometheus.Desc
	pauseStat   *prometheus.Desc
	fecStat     *prometheus.Desc

	coalesce   *prometheus.Desc
	ring       *prometheus.Desc
	pause      *prometheus.Desc
	channels   *prometheus.Desc
	eeeInfo    *prometheus.Desc
	eeeActive  *prometheus.Desc
	fecInfo    *prometheus.Desc
	feature    *prometheus.Desc
	wolInfo    *prometheus.Desc
	moduleInfo *prometheus.Desc

	neighborInfo *prometheus.Desc
	neighborSeen *prometheus.Desc
	neighborTTL  *prometheus.Desc

	scrapeOK  *prometheus.Desc
	scrapeSec *prometheus.Desc
}

func New(opts Options) (*Collector, error) {
	sysfs := opts.SysFS
	if sysfs == "" {
		sysfs = "/sys"
	}
	procfs := opts.ProcFS
	if procfs == "" {
		procfs = "/proc"
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	eth, err := ethtool.New()
	if err != nil {
		logger.Warn("ethtool ioctl client unavailable; driver ioctls disabled", "err", err)
	}
	nl, err := ethtool.NewNetlink()
	if err != nil {
		logger.Warn("ethtool genetlink unavailable; standard/pause/fec stats disabled", "err", err)
	}
	rt, err := rtn.New()
	if err != nil {
		logger.Warn("rtnetlink unavailable; address/neighbor/fdb/qdisc metrics disabled", "err", err)
	}
	feat := DefaultFeatures()
	if opts.Features != nil {
		feat = *opts.Features
	}
	var nft *netfilter.Client
	if feat.Nftables {
		var nerr error
		nft, nerr = netfilter.New()
		if nerr != nil {
			logger.Warn("nftables netlink unavailable; nft counter metrics disabled", "err", nerr)
		}
	}
	workers := opts.Perf.Workers
	if workers <= 0 {
		workers = 4
	}
	if n := runtime.GOMAXPROCS(0); workers > n {
		workers = n
	}
	opticsTTL := opts.Perf.OpticsCacheTTL
	if opticsTTL <= 0 {
		opticsTTL = 60 * time.Second
	}
	negTTL := opts.Perf.NegCacheTTL
	if negTTL <= 0 {
		negTTL = 5 * time.Minute
	}
	nftTTL := opts.Perf.NftCacheTTL
	if nftTTL <= 0 {
		nftTTL = 15 * time.Second
	}
	maxL3 := opts.Perf.MaxL3Neigh
	if maxL3 == 0 {
		maxL3 = 4096
	}
	maxFDB := opts.Perf.MaxFDB
	if maxFDB == 0 {
		maxFDB = 8192
	}
	c := &Collector{
		sysfs:          sysfs,
		procfs:         procfs,
		filter:         opts.Filter,
		statsInclude:   opts.StatsInclude,
		neighbors:      opts.NeighborStore,
		neighborRedact: opts.NeighborRedact,
		logger:         logger,
		eth:            eth,
		nl:             nl,
		rtn:            rt,
		nft:            nft,
		extra:          newExtraDescs(),
		feat:           feat,
		workers:        workers,
		domCache:       newTTLCache[ethtool.ModuleDOM](opticsTTL, negTTL),
		nftCache:       newTTLCache[netfilter.Snapshot](nftTTL, nftTTL),
		maxL3Neigh:     maxL3,
		maxFDB:         maxFDB,
		scrapeStage: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "scrape", "stage_duration_seconds"),
			"Wall time spent in a scrape stage.",
			[]string{"stage"}, nil,
		),

		ifaceInfo: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "interface", "info"),
			"Interface identity and static attributes from sysfs (value always 1).",
			[]string{"device", "address", "broadcast", "operstate", "duplex", "flags", "ifalias", "phys_port_id", "phys_port_name", "phys_switch_id", "group", "name_assign_type"},
			nil,
		),
		ifaceUp: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "interface", "up"),
			"1 if operstate is up.",
			[]string{"device"}, nil,
		),
		ifaceCarrier: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "interface", "carrier"),
			"Carrier state from sysfs (1=up).",
			[]string{"device"}, nil,
		),
		ifaceMTU: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "interface", "mtu_bytes"),
			"Interface MTU.",
			[]string{"device"}, nil,
		),
		ifaceSpeed: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "interface", "speed_mbps"),
			"Negotiated or configured speed in Mbps from sysfs (-1 if unknown).",
			[]string{"device"}, nil,
		),
		ifaceStat: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "interface", "stat"),
			"Kernel interface counters from /sys/class/net/<dev>/statistics.",
			[]string{"device", "stat"}, nil,
		),
		ifaceCarrierChanges: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "interface", "carrier_changes_total"),
			"Carrier change counters from sysfs.",
			[]string{"device", "direction"}, nil,
		),

		drvInfo: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "driver", "info"),
			"Driver information from ETHTOOL_GDRVINFO (value always 1).",
			[]string{"device", "driver", "version", "firmware_version", "bus_info", "erom_version"},
			nil,
		),
		linkInfo: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "link", "info"),
			"Link settings from ETHTOOL_GLINKSETTINGS/GSET (value always 1).",
			[]string{"device", "duplex", "port", "tp_mdix"},
			nil,
		),
		linkUp: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "link", "up"),
			"1 if ETHTOOL_GLINK reports link detected.",
			[]string{"device"}, nil,
		),
		linkSpeed: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "link", "speed_mbps"),
			"Link speed in Mbps from ethtool.",
			[]string{"device"}, nil,
		),
		linkAutoneg: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "link", "autoneg"),
			"1 if autonegotiation is enabled.",
			[]string{"device"}, nil,
		),
		drvStat: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "nic", "statistics"),
			"Driver-specific NIC statistics from ETHTOOL_GSTATS (ethtool -S).",
			[]string{"device", "stat"}, nil,
		),
		phyStat: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "phy", "statistics"),
			"PHY-specific statistics from ETHTOOL_GPHYSTATS.",
			[]string{"device", "stat"}, nil,
		),
		stdStat: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "std", "statistics"),
			"Standardized IEEE/RMON/PHY statistics from ETHTOOL_MSG_STATS_GET.",
			[]string{"device", "group", "stat"}, nil,
		),
		rmonHist: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "rmon", "hist_packets"),
			"RMON packet-size histogram buckets from ETHTOOL_MSG_STATS_GET.",
			[]string{"device", "direction", "low", "high"}, nil,
		),
		pauseStat: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "pause", "frames_total"),
			"Pause-frame counters from ETHTOOL_MSG_PAUSE_GET with ETHTOOL_FLAG_STATS.",
			[]string{"device", "stat"}, nil,
		),
		fecStat: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "fec", "blocks_total"),
			"FEC counters from ETHTOOL_MSG_FEC_GET with ETHTOOL_FLAG_STATS.",
			[]string{"device", "stat"}, nil,
		),

		coalesce: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "coalesce", "setting"),
			"Interrupt coalescing parameters from ETHTOOL_GCOALESCE.",
			[]string{"device", "setting"}, nil,
		),
		ring: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "ring", "setting"),
			"Ring buffer parameters from ETHTOOL_GRINGPARAM.",
			[]string{"device", "setting"}, nil,
		),
		pause: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "pause", "setting"),
			"Pause frame parameters from ETHTOOL_GPAUSEPARAM (1=enabled).",
			[]string{"device", "setting"}, nil,
		),
		channels: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "channel", "setting"),
			"Channel counts from ETHTOOL_GCHANNELS.",
			[]string{"device", "setting"}, nil,
		),
		eeeInfo: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "eee", "info"),
			"Energy Efficient Ethernet flags (1=true).",
			[]string{"device", "setting"}, nil,
		),
		eeeActive: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "eee", "tx_lpi_timer"),
			"EEE TX LPI timer.",
			[]string{"device"}, nil,
		),
		fecInfo: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "fec", "mode"),
			"FEC mode bitmasks from ETHTOOL_GFECPARAM.",
			[]string{"device", "setting"}, nil,
		),
		feature: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "feature", "info"),
			"Offload/feature state from ETHTOOL_GFEATURES (1=true).",
			[]string{"device", "feature", "state"}, nil,
		),
		wolInfo: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wol", "info"),
			"Wake-on-LAN supported/opts bitmasks from ETHTOOL_GWOL.",
			[]string{"device", "setting"}, nil,
		),
		moduleInfo: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "module", "info"),
			"Plug-in module info from ETHTOOL_GMODULEINFO.",
			[]string{"device", "type"}, nil,
		),

		neighborInfo: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "neighbor", "info"),
			"LLDP/CDP neighbor identity learned passively from the wire (value always 1).",
			[]string{"device", "protocol", "chassis_id", "port_id", "system_name", "system_desc", "port_desc", "management_address", "platform", "capabilities", "vlan", "source_mac"},
			nil,
		),
		neighborSeen: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "neighbor", "last_seen_seconds"),
			"Unix timestamp when the neighbor was last heard.",
			[]string{"device", "protocol", "chassis_id", "port_id"}, nil,
		),
		neighborTTL: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "neighbor", "ttl_seconds"),
			"TTL advertised by the neighbor PDU.",
			[]string{"device", "protocol", "chassis_id", "port_id"}, nil,
		),

		scrapeOK: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "scrape", "success"),
			"1 if the last scrape of this collector completed without fatal error.",
			nil, nil,
		),
		scrapeSec: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "scrape", "duration_seconds"),
			"Duration of the scrape.",
			nil, nil,
		),
	}
	return c, nil
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.ifaceInfo
	ch <- c.ifaceUp
	ch <- c.ifaceCarrier
	ch <- c.ifaceMTU
	ch <- c.ifaceSpeed
	ch <- c.ifaceStat
	ch <- c.ifaceCarrierChanges
	ch <- c.drvInfo
	ch <- c.linkInfo
	ch <- c.linkUp
	ch <- c.linkSpeed
	ch <- c.linkAutoneg
	ch <- c.drvStat
	ch <- c.phyStat
	ch <- c.stdStat
	ch <- c.rmonHist
	ch <- c.pauseStat
	ch <- c.fecStat
	ch <- c.coalesce
	ch <- c.ring
	ch <- c.pause
	ch <- c.channels
	ch <- c.eeeInfo
	ch <- c.eeeActive
	ch <- c.fecInfo
	ch <- c.feature
	ch <- c.wolInfo
	ch <- c.moduleInfo
	ch <- c.neighborInfo
	ch <- c.neighborSeen
	ch <- c.neighborTTL
	c.extra.describe(ch)
	ch <- c.scrapeStage
	ch <- c.scrapeOK
	ch <- c.scrapeSec
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		ch <- prometheus.MustNewConstMetric(c.scrapeSec, prometheus.GaugeValue, v)
	}))
	defer timer.ObserveDuration()

	ok := 1.0
	t0 := time.Now()
	names, err := listInterfaces(c.sysfs)
	if err != nil {
		c.logger.Error("list interfaces", "err", err)
		ok = 0
		ch <- prometheus.MustNewConstMetric(c.scrapeOK, prometheus.GaugeValue, ok)
		return
	}
	var devices []string
	for _, name := range names {
		if c.filter.Match(name) {
			devices = append(devices, name)
		}
	}
	ch <- prometheus.MustNewConstMetric(c.scrapeStage, prometheus.GaugeValue, time.Since(t0).Seconds(), "list_interfaces")

	tDev := time.Now()
	c.collectDevicesParallel(ch, devices)
	ch <- prometheus.MustNewConstMetric(c.scrapeStage, prometheus.GaugeValue, time.Since(tDev).Seconds(), "devices")

	tHost := time.Now()
	c.collectHostExtra(ch)
	ch <- prometheus.MustNewConstMetric(c.scrapeStage, prometheus.GaugeValue, time.Since(tHost).Seconds(), "host")

	if c.neighbors != nil {
		for _, n := range c.neighbors.List() {
			chassis, port := truncate(n.ChassisID, 128), truncate(n.PortID, 128)
			sysName, sysDesc, portDesc := truncate(n.SystemName, 128), truncate(n.SystemDesc, 256), truncate(n.PortDesc, 128)
			mgmt, platform, caps, vlan, srcMAC := truncate(n.ManagementAddr, 64), truncate(n.Platform, 64), truncate(n.Capabilities, 64), truncate(n.VLAN, 32), truncate(n.RawSourceMAC, 32)
			if c.neighborRedact {
				sysName, sysDesc, portDesc = "", "", ""
				mgmt, platform, caps, vlan, srcMAC = "", "", "", "", ""
			}
			ch <- prometheus.MustNewConstMetric(c.neighborInfo, prometheus.GaugeValue, 1,
				n.Interface, string(n.Protocol), chassis, port, sysName, sysDesc,
				portDesc, mgmt, platform, caps, vlan, srcMAC,
			)
			ch <- prometheus.MustNewConstMetric(c.neighborSeen, prometheus.GaugeValue, float64(n.LastSeen.Unix()),
				n.Interface, string(n.Protocol), chassis, port,
			)
			ch <- prometheus.MustNewConstMetric(c.neighborTTL, prometheus.GaugeValue, n.TTL.Seconds(),
				n.Interface, string(n.Protocol), chassis, port,
			)
		}
	}

	ch <- prometheus.MustNewConstMetric(c.scrapeOK, prometheus.GaugeValue, ok)
}

func (c *Collector) collectDevicesParallel(ch chan<- prometheus.Metric, devices []string) {
	if len(devices) == 0 {
		return
	}
	workers := c.workers
	if workers > len(devices) {
		workers = len(devices)
	}
	jobs := make(chan string, len(devices))
	var wg sync.WaitGroup
	worker := func() {
		defer wg.Done()
		for name := range jobs {
			c.collectDevice(ch, name)
			c.collectDeviceExtra(ch, name)
		}
	}
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go worker()
	}
	for _, d := range devices {
		jobs <- d
	}
	close(jobs)
	wg.Wait()
}

func (c *Collector) collectDevice(ch chan<- prometheus.Metric, name string) {
	info, err := readInterface(c.sysfs, name)
	if err != nil {
		c.logger.Debug("read interface", "device", name, "err", err)
		return
	}
	ch <- prometheus.MustNewConstMetric(c.ifaceInfo, prometheus.GaugeValue, 1,
		info.Name, info.Address, info.Broadcast, info.OperState, info.Duplex, info.Flags,
		info.IfAlias, info.PhysPortID, info.PhysPortName, info.PhysSwitchID, info.Group, info.NameAssign,
	)
	up := 0.0
	if info.OperState == "up" {
		up = 1
	}
	ch <- prometheus.MustNewConstMetric(c.ifaceUp, prometheus.GaugeValue, up, name)
	ch <- prometheus.MustNewConstMetric(c.ifaceCarrier, prometheus.GaugeValue, float64(info.Carrier), name)
	ch <- prometheus.MustNewConstMetric(c.ifaceMTU, prometheus.GaugeValue, float64(info.MTU), name)
	ch <- prometheus.MustNewConstMetric(c.ifaceSpeed, prometheus.GaugeValue, float64(info.Speed), name)
	ch <- prometheus.MustNewConstMetric(c.ifaceCarrierChanges, prometheus.CounterValue, float64(info.CarrierChanges), name, "total")
	ch <- prometheus.MustNewConstMetric(c.ifaceCarrierChanges, prometheus.CounterValue, float64(info.CarrierUpCount), name, "up")
	ch <- prometheus.MustNewConstMetric(c.ifaceCarrierChanges, prometheus.CounterValue, float64(info.CarrierDownCount), name, "down")

	if stats, err := readStatistics(c.sysfs, name); err == nil {
		for k, v := range stats {
			ch <- prometheus.MustNewConstMetric(c.ifaceStat, prometheus.CounterValue, float64(v), name, k)
		}
	}

	if c.nl != nil {
		if c.feat.StdStats {
			if std, err := c.nl.StandardStats(name); err == nil {
				for group, stats := range std.Groups {
					for k, v := range stats {
						ch <- prometheus.MustNewConstMetric(c.stdStat, prometheus.CounterValue, float64(v), name, group, k)
					}
				}
				for _, b := range std.RMONHistRX {
					ch <- prometheus.MustNewConstMetric(c.rmonHist, prometheus.CounterValue, float64(b.Val),
						name, "rx", strconv.FormatUint(uint64(b.Low), 10), strconv.FormatUint(uint64(b.High), 10))
				}
				for _, b := range std.RMONHistTX {
					ch <- prometheus.MustNewConstMetric(c.rmonHist, prometheus.CounterValue, float64(b.Val),
						name, "tx", strconv.FormatUint(uint64(b.Low), 10), strconv.FormatUint(uint64(b.High), 10))
				}
			} else {
				c.logger.Debug("standard stats", "device", name, "err", err)
			}
		}
		if c.feat.PauseFEC {
			if ps, err := c.nl.PauseStats(name); err == nil {
				if ps.HasTXFrames {
					ch <- prometheus.MustNewConstMetric(c.pauseStat, prometheus.CounterValue, float64(ps.TXFrames), name, "tx_frames")
				}
				if ps.HasRXFrames {
					ch <- prometheus.MustNewConstMetric(c.pauseStat, prometheus.CounterValue, float64(ps.RXFrames), name, "rx_frames")
				}
				if ps.HasTXStorms {
					ch <- prometheus.MustNewConstMetric(c.pauseStat, prometheus.CounterValue, float64(ps.TXPauseStorms), name, "tx_pause_storm_events")
				}
			}
			if fs, err := c.nl.FECStats(name); err == nil {
				if fs.HasCorrected {
					ch <- prometheus.MustNewConstMetric(c.fecStat, prometheus.CounterValue, float64(fs.CorrectedBlocks), name, "corrected_blocks")
				}
				if fs.HasUncorrectable {
					ch <- prometheus.MustNewConstMetric(c.fecStat, prometheus.CounterValue, float64(fs.UncorrectableBlocks), name, "uncorrectable_blocks")
				}
				if fs.HasCorrectedBits {
					ch <- prometheus.MustNewConstMetric(c.fecStat, prometheus.CounterValue, float64(fs.CorrectedBits), name, "corrected_bits")
				}
			}
		}
	}

	if c.eth == nil {
		return
	}

	if drv, err := c.eth.DriverInfo(name); err == nil {
		ch <- prometheus.MustNewConstMetric(c.drvInfo, prometheus.GaugeValue, 1,
			name, drv.Driver, drv.Version, drv.FWVersion, drv.BusInfo, drv.EromVersion,
		)
	}

	if link, err := c.eth.LinkSettings(name); err == nil {
		ch <- prometheus.MustNewConstMetric(c.linkInfo, prometheus.GaugeValue, 1, name, link.Duplex, link.Port, link.TPMdix)
		lu := 0.0
		if link.LinkDetected {
			lu = 1
		}
		ch <- prometheus.MustNewConstMetric(c.linkUp, prometheus.GaugeValue, lu, name)
		speed := float64(link.Speed)
		if link.Speed == 0 || link.Speed == ^uint32(0) || link.Speed == 65535 {
			speed = -1
		}
		ch <- prometheus.MustNewConstMetric(c.linkSpeed, prometheus.GaugeValue, speed, name)
		an := 0.0
		if link.Autoneg {
			an = 1
		}
		ch <- prometheus.MustNewConstMetric(c.linkAutoneg, prometheus.GaugeValue, an, name)
	}

	if stats, err := c.eth.Stats(name); err == nil {
		for k, v := range stats {
			if c.statsInclude != nil && !c.statsInclude.MatchString(k) {
				continue
			}
			ch <- prometheus.MustNewConstMetric(c.drvStat, prometheus.UntypedValue, float64(v), name, sanitizeLabel(k))
		}
	}
	if stats, err := c.eth.PhyStats(name); err == nil {
		for k, v := range stats {
			if c.statsInclude != nil && !c.statsInclude.MatchString(k) {
				continue
			}
			ch <- prometheus.MustNewConstMetric(c.phyStat, prometheus.UntypedValue, float64(v), name, sanitizeLabel(k))
		}
	}

	if coal, err := c.eth.Coalesce(name); err == nil {
		emitU32(ch, c.coalesce, name, map[string]uint32{
			"rx_usecs": coal.RxUsecs, "rx_max_frames": coal.RxMaxFrames,
			"tx_usecs": coal.TxUsecs, "tx_max_frames": coal.TxMaxFrames,
			"rx_usecs_irq": coal.RxUsecsIRQ, "tx_usecs_irq": coal.TxUsecsIRQ,
			"stats_block_usecs": coal.StatsBlockUsecs,
			"pkt_rate_low":      coal.PktRateLow, "pkt_rate_high": coal.PktRateHigh,
			"rate_sample_interval": coal.RateSampleInterval,
		})
		emitBool(ch, c.coalesce, name, "use_adaptive_rx", coal.UseAdaptiveRX)
		emitBool(ch, c.coalesce, name, "use_adaptive_tx", coal.UseAdaptiveTX)
	}

	if ring, err := c.eth.RingParam(name); err == nil {
		emitU32(ch, c.ring, name, map[string]uint32{
			"rx_max_pending": ring.RXMaxPending, "rx_pending": ring.RXPending,
			"rx_mini_max_pending": ring.RXMiniMaxPending, "rx_mini_pending": ring.RXMiniPending,
			"rx_jumbo_max_pending": ring.RXJumboMaxPending, "rx_jumbo_pending": ring.RXJumboPending,
			"tx_max_pending": ring.TXMaxPending, "tx_pending": ring.TXPending,
		})
	}

	if pause, err := c.eth.PauseParam(name); err == nil {
		emitBool(ch, c.pause, name, "autoneg", pause.Autoneg)
		emitBool(ch, c.pause, name, "rx", pause.RX)
		emitBool(ch, c.pause, name, "tx", pause.TX)
	}

	if chans, err := c.eth.Channels(name); err == nil {
		emitU32(ch, c.channels, name, map[string]uint32{
			"max_rx": chans.MaxRX, "max_tx": chans.MaxTX, "max_other": chans.MaxOther, "max_combined": chans.MaxCombined,
			"rx_count": chans.RXCount, "tx_count": chans.TXCount, "other_count": chans.OtherCount, "combined_count": chans.CombinedCount,
		})
	}

	if eee, err := c.eth.EEE(name); err == nil {
		emitBool(ch, c.eeeInfo, name, "active", eee.EEEActive)
		emitBool(ch, c.eeeInfo, name, "enabled", eee.EEEEnabled)
		emitBool(ch, c.eeeInfo, name, "tx_lpi_enabled", eee.TxLpiEnabled)
		ch <- prometheus.MustNewConstMetric(c.eeeActive, prometheus.GaugeValue, float64(eee.TxLpiTimer), name)
	}

	if fec, err := c.eth.FEC(name); err == nil {
		emitU32(ch, c.fecInfo, name, map[string]uint32{
			"active": fec.Active, "configured": fec.Configured, "supported": fec.Supported,
		})
	}

	if c.feat.Features {
		if feats, err := c.eth.Features(name); err == nil {
			for fname, st := range feats {
				label := sanitizeLabel(fname)
				ch <- prometheus.MustNewConstMetric(c.feature, prometheus.GaugeValue, boolFloat(st.Active), name, label, "active")
				ch <- prometheus.MustNewConstMetric(c.feature, prometheus.GaugeValue, boolFloat(st.Requested), name, label, "requested")
				ch <- prometheus.MustNewConstMetric(c.feature, prometheus.GaugeValue, boolFloat(st.Available), name, label, "available")
			}
		}
	}

	if wol, err := c.eth.WoL(name); err == nil {
		emitU32(ch, c.wolInfo, name, map[string]uint32{"supported": wol.Supported, "opts": wol.Opts})
	}

	if mod, err := c.eth.ModuleInfo(name); err == nil {
		ch <- prometheus.MustNewConstMetric(c.moduleInfo, prometheus.GaugeValue, float64(mod.EEPROM), name, strconv.FormatUint(uint64(mod.Type), 10))
	}
}

func emitU32(ch chan<- prometheus.Metric, desc *prometheus.Desc, device string, vals map[string]uint32) {
	for k, v := range vals {
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(v), device, k)
	}
}

func emitBool(ch chan<- prometheus.Metric, desc *prometheus.Desc, device, setting string, v bool) {
	ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, boolFloat(v), device, setting)
}

func boolFloat(v bool) float64 {
	if v {
		return 1
	}
	return 0
}

func sanitizeLabel(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, " ", "_")
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// formatTCHandle renders a tc handle as major:minor (hex), matching `tc` output.
func formatTCHandle(h uint32) string {
	return strconv.FormatUint(uint64(h>>16), 16) + ":" + strconv.FormatUint(uint64(h&0xffff), 16)
}
