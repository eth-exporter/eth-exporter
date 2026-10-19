package collector

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/eth-exporter/eth-exporter/internal/ethtool"
	"github.com/eth-exporter/eth-exporter/internal/netfilter"
	rtn "github.com/eth-exporter/eth-exporter/internal/netlink"
)

// ExtraDescs holds optional comprehensive metrics.
type ExtraDescs struct {
	linkStateUp     *prometheus.Desc
	linkSQI         *prometheus.Desc
	linkSQIMax      *prometheus.Desc
	linkExtState    *prometheus.Desc
	linkDownCnt     *prometheus.Desc
	linkMode        *prometheus.Desc
	linkLanes       *prometheus.Desc
	linkMasterSlave *prometheus.Desc

	privFlag *prometheus.Desc
	rssInfo  *prometheus.Desc
	rssIndir *prometheus.Desc

	moduleInfoDOM *prometheus.Desc
	moduleTemp    *prometheus.Desc
	moduleVolt    *prometheus.Desc
	moduleBias    *prometheus.Desc
	moduleTxPower *prometheus.Desc
	moduleRxPower *prometheus.Desc
	moduleChan    *prometheus.Desc

	deviceInfo  *prometheus.Desc
	deviceSRIOV *prometheus.Desc
	queues      *prometheus.Desc
	bondInfo    *prometheus.Desc
	bondUp      *prometheus.Desc
	bridgePort  *prometheus.Desc
	vfInfo      *prometheus.Desc

	addrInfo    *prometheus.Desc
	permAddr    *prometheus.Desc
	xdpAttached *prometheus.Desc
	l3Neigh     *prometheus.Desc
	fdbEntry    *prometheus.Desc
	qdiscStat   *prometheus.Desc
	qdiscQueue  *prometheus.Desc
	qdiscInfo   *prometheus.Desc

	softnet   *prometheus.Desc
	stackStat *prometheus.Desc

	rdmaInfo    *prometheus.Desc
	rdmaPort    *prometheus.Desc
	rdmaCounter *prometheus.Desc

	ctEntries *prometheus.Desc
	ctMax     *prometheus.Desc
	ctStat    *prometheus.Desc

	nftTable   *prometheus.Desc
	nftChain   *prometheus.Desc
	nftCounter *prometheus.Desc

	igmpGroup *prometheus.Desc
	mldGroup  *prometheus.Desc
	devMcast  *prometheus.Desc
}

func newExtraDescs() ExtraDescs {
	return ExtraDescs{
		linkStateUp: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "linkstate", "up"),
			"Link up from ETHTOOL_MSG_LINKSTATE_GET.",
			[]string{"device"}, nil,
		),
		linkSQI: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "linkstate", "sqi"),
			"Signal Quality Index.",
			[]string{"device"}, nil,
		),
		linkSQIMax: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "linkstate", "sqi_max"),
			"Maximum Signal Quality Index.",
			[]string{"device"}, nil,
		),
		linkExtState: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "linkstate", "ext_state_info"),
			"Extended link down state/substate (value always 1).",
			[]string{"device", "ext_state", "ext_substate"}, nil,
		),
		linkDownCnt: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "linkstate", "down_events_total"),
			"Extended link-down event count.",
			[]string{"device"}, nil,
		),
		linkMode: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "linkmode", "info"),
			"Link mode bit from LINKMODES_GET (1=set).",
			[]string{"device", "mode", "set"}, nil,
		),
		linkLanes: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "linkmode", "lanes"),
			"Negotiated lane count.",
			[]string{"device"}, nil,
		),
		linkMasterSlave: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "linkmode", "master_slave_info"),
			"Master/slave cfg and state (value always 1).",
			[]string{"device", "cfg", "state", "rate_matching"}, nil,
		),

		privFlag: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "privflag", "active"),
			"Private flag bit is active (1).",
			[]string{"device", "bit"}, nil,
		),
		rssInfo: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "rss", "info"),
			"RSS configuration (value always 1).",
			[]string{"device", "hfunc", "input_xfrm"}, nil,
		),
		rssIndir: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "rss", "indir_size"),
			"RSS indirection table length.",
			[]string{"device"}, nil,
		),

		moduleInfoDOM: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "module", "dom_info"),
			"Module identity from EEPROM (value always 1).",
			[]string{"device", "vendor", "part_number", "serial", "type"}, nil,
		),
		moduleTemp: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "module", "temperature_celsius"),
			"Module temperature from DOM.",
			[]string{"device"}, nil,
		),
		moduleVolt: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "module", "voltage_volts"),
			"Module supply voltage from DOM.",
			[]string{"device"}, nil,
		),
		moduleBias: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "module", "bias_milliamps"),
			"Laser bias current from DOM.",
			[]string{"device"}, nil,
		),
		moduleTxPower: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "module", "tx_power_dbm"),
			"TX optical power from DOM.",
			[]string{"device"}, nil,
		),
		moduleRxPower: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "module", "rx_power_dbm"),
			"RX optical power from DOM.",
			[]string{"device"}, nil,
		),
		moduleChan: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "module", "channel"),
			"Per-lane QSFP DOM reading.",
			[]string{"device", "lane", "metric"}, nil,
		),

		deviceInfo: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "device", "info"),
			"PCI/platform device identity (value always 1).",
			[]string{"device", "driver", "vendor", "device_id", "subsystem_vendor", "subsystem_device", "numa_node", "irq", "path"}, nil,
		),
		deviceSRIOV: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "device", "sriov_numvfs"),
			"Configured SR-IOV virtual functions.",
			[]string{"device"}, nil,
		),
		queues: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "queue", "count"),
			"Number of RX/TX queues.",
			[]string{"device", "direction"}, nil,
		),
		bondInfo: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "bond", "info"),
			"Bonding master/slave identity (value always 1).",
			[]string{"device", "role", "mode", "active_slave", "mii_status", "slave_state", "primary", "lacp_partner"}, nil,
		),
		bondUp: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "bond", "mii_up"),
			"1 if bonding MII status is up.",
			[]string{"device"}, nil,
		),
		bridgePort: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "bridge_port", "info"),
			"Bridge port state (value always 1).",
			[]string{"device", "state", "priority", "port_no", "hairpin", "path_cost"}, nil,
		),
		vfInfo: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "vf", "info"),
			"SR-IOV VF identity (value always 1).",
			[]string{"device", "vf", "mac"}, nil,
		),

		addrInfo: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "address", "info"),
			"Interface IP address (value always 1).",
			[]string{"device", "family", "address", "prefix", "scope"}, nil,
		),
		permAddr: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "interface", "perm_address_info"),
			"Permanent hardware address (value always 1).",
			[]string{"device", "address"}, nil,
		),
		xdpAttached: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "xdp", "attached"),
			"1 if an XDP program is attached.",
			[]string{"device", "prog_id"}, nil,
		),
		l3Neigh: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "l3_neighbor", "info"),
			"ARP/ND neighbor table entry (value always 1).",
			[]string{"device", "family", "ip", "lladdr", "state"}, nil,
		),
		fdbEntry: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "fdb", "info"),
			"Bridge FDB entry (value always 1).",
			[]string{"device", "mac", "vlan", "state"}, nil,
		),
		qdiscInfo: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "qdisc", "info"),
			"Traffic control qdisc (value always 1).",
			[]string{"device", "kind", "handle", "parent"}, nil,
		),
		qdiscStat: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "qdisc", "stat"),
			"Qdisc counters (bytes/packets/drops).",
			[]string{"device", "kind", "handle", "parent", "stat"}, nil,
		),
		qdiscQueue: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "qdisc", "queue"),
			"Qdisc queue depth gauges (qlen/backlog).",
			[]string{"device", "kind", "handle", "parent", "stat"}, nil,
		),

		softnet: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "softnet", "stat"),
			"Per-CPU softnet statistics from /proc/net/softnet_stat.",
			[]string{"cpu", "stat"}, nil,
		),
		stackStat: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "stack", "stat"),
			"Host network stack counters from /proc/net/snmp and netstat.",
			[]string{"protocol", "stat"}, nil,
		),

		rdmaInfo: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "rdma", "device_info"),
			"RDMA/InfiniBand HCA identity from /sys/class/infiniband (value always 1).",
			[]string{"device", "node_type", "fw_ver", "hca_type", "node_guid", "board_id"}, nil,
		),
		rdmaPort: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "rdma", "port_info"),
			"RDMA port state from sysfs (value always 1).",
			[]string{"device", "port", "state", "phys_state", "rate", "link_layer", "lid", "sm_lid"}, nil,
		),
		rdmaCounter: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "rdma", "port_stat"),
			"RDMA port counters (counters/ or hw_counters/).",
			[]string{"device", "port", "source", "stat"}, nil,
		),

		ctEntries: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "conntrack", "entries"),
			"Current nf_conntrack entry count.",
			nil, nil,
		),
		ctMax: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "conntrack", "max"),
			"Configured nf_conntrack_max.",
			nil, nil,
		),
		ctStat: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "conntrack", "stat"),
			"Per-CPU nf_conntrack stats from /proc/net/stat/nf_conntrack.",
			[]string{"cpu", "stat"}, nil,
		),

		nftTable: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "nftables", "table_info"),
			"nftables table (value always 1).",
			[]string{"family", "table"}, nil,
		),
		nftChain: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "nftables", "chain_info"),
			"nftables chain (value always 1).",
			[]string{"family", "table", "chain"}, nil,
		),
		nftCounter: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "nftables", "counter"),
			"Named nftables counter object.",
			[]string{"family", "table", "name", "stat"}, nil,
		),

		igmpGroup: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "igmp", "group_info"),
			"IPv4 IGMP multicast group membership (value=users).",
			[]string{"device", "group", "querier"}, nil,
		),
		mldGroup: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "mld", "group_info"),
			"IPv6 MLD multicast group membership (value=users).",
			[]string{"device", "group", "flags"}, nil,
		),
		devMcast: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "device_mcast", "info"),
			"Link-layer multicast address membership (value=refcount).",
			[]string{"device", "address", "bound"}, nil,
		),
	}
}

func (e ExtraDescs) describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{
		e.linkStateUp, e.linkSQI, e.linkSQIMax, e.linkExtState, e.linkDownCnt,
		e.linkMode, e.linkLanes, e.linkMasterSlave,
		e.privFlag, e.rssInfo, e.rssIndir,
		e.moduleInfoDOM, e.moduleTemp, e.moduleVolt, e.moduleBias, e.moduleTxPower, e.moduleRxPower, e.moduleChan,
		e.deviceInfo, e.deviceSRIOV, e.queues, e.bondInfo, e.bondUp, e.bridgePort, e.vfInfo,
		e.addrInfo, e.permAddr, e.xdpAttached, e.l3Neigh, e.fdbEntry, e.qdiscInfo, e.qdiscStat, e.qdiscQueue,
		e.softnet, e.stackStat,
		e.rdmaInfo, e.rdmaPort, e.rdmaCounter,
		e.ctEntries, e.ctMax, e.ctStat,
		e.nftTable, e.nftChain, e.nftCounter,
		e.igmpGroup, e.mldGroup, e.devMcast,
	} {
		ch <- d
	}
}

func (c *Collector) collectDeviceExtra(ch chan<- prometheus.Metric, name string) {
	if c.nl != nil {
		if ls, err := c.nl.LinkState(name); err == nil {
			up := 0.0
			if ls.Link {
				up = 1
			}
			ch <- prometheus.MustNewConstMetric(c.extra.linkStateUp, prometheus.GaugeValue, up, name)
			if ls.HasSQI {
				ch <- prometheus.MustNewConstMetric(c.extra.linkSQI, prometheus.GaugeValue, float64(ls.SQI), name)
				ch <- prometheus.MustNewConstMetric(c.extra.linkSQIMax, prometheus.GaugeValue, float64(ls.SQIMax), name)
			}
			if ls.HasExtState {
				ch <- prometheus.MustNewConstMetric(c.extra.linkExtState, prometheus.GaugeValue, 1,
					name, strconv.Itoa(int(ls.ExtState)), strconv.Itoa(int(ls.ExtSubstate)))
			}
			if ls.HasDownCnt {
				ch <- prometheus.MustNewConstMetric(c.extra.linkDownCnt, prometheus.CounterValue, float64(ls.ExtDownCnt), name)
			}
		}
		if c.feat.LinkModes {
			if lm, err := c.nl.LinkModes(name); err == nil {
				for _, m := range lm.Supported {
					ch <- prometheus.MustNewConstMetric(c.extra.linkMode, prometheus.GaugeValue, 1, name, m, "supported")
				}
				for _, m := range lm.Advertised {
					ch <- prometheus.MustNewConstMetric(c.extra.linkMode, prometheus.GaugeValue, 1, name, m, "advertised")
				}
				for _, m := range lm.Partner {
					ch <- prometheus.MustNewConstMetric(c.extra.linkMode, prometheus.GaugeValue, 1, name, m, "partner")
				}
				if lm.HasLanes {
					ch <- prometheus.MustNewConstMetric(c.extra.linkLanes, prometheus.GaugeValue, float64(lm.Lanes), name)
				}
				ch <- prometheus.MustNewConstMetric(c.extra.linkMasterSlave, prometheus.GaugeValue, 1,
					name, strconv.Itoa(int(lm.MasterSlaveCfg)), strconv.Itoa(int(lm.MasterSlaveState)), strconv.Itoa(int(lm.RateMatching)))
			}
		}
		if c.feat.PrivFlags {
			if pf, err := c.nl.PrivFlags(name); err == nil {
				for _, bit := range pf.ActiveBits {
					ch <- prometheus.MustNewConstMetric(c.extra.privFlag, prometheus.GaugeValue, 1, name, strconv.FormatUint(uint64(bit), 10))
				}
			}
		}
		if c.feat.RSS {
			if rss, err := c.nl.RSS(name); err == nil {
				ch <- prometheus.MustNewConstMetric(c.extra.rssInfo, prometheus.GaugeValue, 1,
					name, strconv.FormatUint(uint64(rss.HFunc), 10), strconv.FormatUint(uint64(rss.InputXfrm), 10))
				ch <- prometheus.MustNewConstMetric(c.extra.rssIndir, prometheus.GaugeValue, float64(rss.IndirLen), name)
			}
		}
	}

	if c.feat.Optics && c.eth != nil && c.domCache != nil {
		dom, err, _ := c.domCache.getOr(name, func() (ethtool.ModuleDOM, error) {
			return c.eth.ModuleDOM(name)
		})
		if err == nil {
			ch <- prometheus.MustNewConstMetric(c.extra.moduleInfoDOM, prometheus.GaugeValue, 1,
				name, dom.Vendor, dom.PartNumber, dom.Serial, strconv.FormatUint(uint64(dom.Type), 10))
			if dom.HasTemperature {
				ch <- prometheus.MustNewConstMetric(c.extra.moduleTemp, prometheus.GaugeValue, dom.TemperatureC, name)
			}
			if dom.HasVoltage {
				ch <- prometheus.MustNewConstMetric(c.extra.moduleVolt, prometheus.GaugeValue, dom.VoltageV, name)
			}
			if dom.HasBias {
				ch <- prometheus.MustNewConstMetric(c.extra.moduleBias, prometheus.GaugeValue, dom.BiasmA, name)
			}
			if dom.HasTxPower {
				ch <- prometheus.MustNewConstMetric(c.extra.moduleTxPower, prometheus.GaugeValue, dom.TxPowerdBm, name)
			}
			if dom.HasRxPower {
				ch <- prometheus.MustNewConstMetric(c.extra.moduleRxPower, prometheus.GaugeValue, dom.RxPowerdBm, name)
			}
			for _, chn := range dom.Channels {
				lane := strconv.Itoa(chn.Lane)
				ch <- prometheus.MustNewConstMetric(c.extra.moduleChan, prometheus.GaugeValue, chn.BiasmA, name, lane, "bias_mA")
				ch <- prometheus.MustNewConstMetric(c.extra.moduleChan, prometheus.GaugeValue, chn.TxPowerdBm, name, lane, "tx_power_dBm")
				ch <- prometheus.MustNewConstMetric(c.extra.moduleChan, prometheus.GaugeValue, chn.RxPowerdBm, name, lane, "rx_power_dBm")
			}
		}
	}

	if id, err := readDeviceIdentity(c.sysfs, name); err == nil {
		ch <- prometheus.MustNewConstMetric(c.extra.deviceInfo, prometheus.GaugeValue, 1,
			name, id.Driver, id.Vendor, id.Device, id.SubsystemVendor, id.SubsystemDevice, id.NUMANode, id.IRQ, id.DevicePath)
		if id.HasSRIOV {
			ch <- prometheus.MustNewConstMetric(c.extra.deviceSRIOV, prometheus.GaugeValue, float64(id.SRIOVNumVFs), name)
		}
	}
	q := readQueueInfo(c.sysfs, name)
	if q.RXQueues > 0 || q.TXQueues > 0 {
		ch <- prometheus.MustNewConstMetric(c.extra.queues, prometheus.GaugeValue, float64(q.RXQueues), name, "rx")
		ch <- prometheus.MustNewConstMetric(c.extra.queues, prometheus.GaugeValue, float64(q.TXQueues), name, "tx")
	}
	bond := readBondInfo(c.sysfs, name)
	if bond.IsMaster || bond.IsSlave {
		role := "slave"
		if bond.IsMaster {
			role = "master"
		}
		ch <- prometheus.MustNewConstMetric(c.extra.bondInfo, prometheus.GaugeValue, 1,
			name, role, bond.Mode, bond.ActiveSlave, bond.MIIStatus, bond.SlaveState, bond.Primary, bond.LacpPartner)
		up := 0.0
		if bond.MIIStatus == "up" {
			up = 1
		}
		ch <- prometheus.MustNewConstMetric(c.extra.bondUp, prometheus.GaugeValue, up, name)
	}
	if br := readBridgePort(c.sysfs, name); br.IsPort {
		ch <- prometheus.MustNewConstMetric(c.extra.bridgePort, prometheus.GaugeValue, 1,
			name, br.State, br.Priority, br.PortNumber, br.Hairpin, br.Cost)
	}
	for _, vf := range readVFs(c.sysfs, name) {
		ch <- prometheus.MustNewConstMetric(c.extra.vfInfo, prometheus.GaugeValue, 1, name, vf.Index, vf.MAC)
	}
}

func (c *Collector) collectHostExtra(ch chan<- prometheus.Metric) {
	if c.rtn != nil {
		links, err := c.rtn.Links()
		var byIdx map[int]string
		if err == nil {
			byIdx = rtn.LinkIndex(links)
			seenLink := map[string]struct{}{}
			for _, l := range links {
				if l.Name == "" || !c.filter.Match(l.Name) {
					continue
				}
				// RTM_GETLINK can yield duplicate ifinfo messages for one device.
				if _, dup := seenLink[l.Name]; dup {
					continue
				}
				seenLink[l.Name] = struct{}{}
				if l.PermAddr != "" {
					ch <- prometheus.MustNewConstMetric(c.extra.permAddr, prometheus.GaugeValue, 1, l.Name, l.PermAddr)
				}
				prog := strconv.FormatUint(uint64(l.XDPProgID), 10)
				attached := 0.0
				if l.XDPAttached {
					attached = 1
				}
				ch <- prometheus.MustNewConstMetric(c.extra.xdpAttached, prometheus.GaugeValue, attached, l.Name, prog)
			}
		}
		if c.feat.Addresses {
			if addrs, err := c.rtn.Addresses(byIdx); err == nil {
				seen := map[string]struct{}{}
				for _, a := range addrs {
					if a.IfName == "" || a.Address == "" || !c.filter.Match(a.IfName) {
						continue
					}
					key := a.IfName + "|" + a.Family + "|" + a.Address + "|" + strconv.Itoa(a.Prefix) + "|" + strconv.Itoa(a.Scope)
					if _, ok := seen[key]; ok {
						continue
					}
					seen[key] = struct{}{}
					ch <- prometheus.MustNewConstMetric(c.extra.addrInfo, prometheus.GaugeValue, 1,
						a.IfName, a.Family, a.Address, strconv.Itoa(a.Prefix), strconv.Itoa(a.Scope))
				}
			}
		}
		if c.feat.L3Neigh {
			if neighs, err := c.rtn.Neighbors(byIdx); err == nil {
				emitted := 0
				seen := map[string]struct{}{}
				for _, n := range neighs {
					if n.IfName == "" || n.IP == "" || !c.filter.Match(n.IfName) {
						continue
					}
					// Skip NUD_NONE (0) and NUD_INCOMPLETE (0x1).
					if n.State == 0 || n.State&0x1 != 0 {
						continue
					}
					key := n.IfName + "|" + n.Family + "|" + n.IP + "|" + n.LLAddr + "|" + n.NudState
					if _, ok := seen[key]; ok {
						continue
					}
					seen[key] = struct{}{}
					if c.maxL3Neigh > 0 && emitted >= c.maxL3Neigh {
						break
					}
					ch <- prometheus.MustNewConstMetric(c.extra.l3Neigh, prometheus.GaugeValue, 1,
						n.IfName, n.Family, n.IP, n.LLAddr, n.NudState)
					emitted++
				}
			}
		}
		if c.feat.FDB {
			if fdb, err := c.rtn.FDB(byIdx); err == nil {
				emitted := 0
				seen := map[string]struct{}{}
				for _, e := range fdb {
					if e.IfName == "" || e.MAC == "" || !c.filter.Match(e.IfName) {
						continue
					}
					vlan := strconv.FormatUint(uint64(e.VLAN), 10)
					state := strconv.FormatUint(uint64(e.State), 10)
					key := e.IfName + "|" + e.MAC + "|" + vlan + "|" + state
					if _, ok := seen[key]; ok {
						continue
					}
					seen[key] = struct{}{}
					if c.maxFDB > 0 && emitted >= c.maxFDB {
						break
					}
					ch <- prometheus.MustNewConstMetric(c.extra.fdbEntry, prometheus.GaugeValue, 1,
						e.IfName, e.MAC, vlan, state)
					emitted++
				}
			}
		}
		if c.feat.Qdisc {
			if qdiscs, err := c.rtn.Qdiscs(byIdx); err == nil {
				seen := map[string]struct{}{}
				for _, q := range qdiscs {
					if q.IfName == "" || !c.filter.Match(q.IfName) {
						continue
					}
					handle := formatTCHandle(q.Handle)
					parent := formatTCHandle(q.Parent)
					key := q.IfName + "|" + q.Kind + "|" + handle + "|" + parent
					if _, ok := seen[key]; ok {
						continue
					}
					seen[key] = struct{}{}
					ch <- prometheus.MustNewConstMetric(c.extra.qdiscInfo, prometheus.GaugeValue, 1,
						q.IfName, q.Kind, handle, parent)
					ch <- prometheus.MustNewConstMetric(c.extra.qdiscStat, prometheus.CounterValue, float64(q.Bytes), q.IfName, q.Kind, handle, parent, "bytes")
					ch <- prometheus.MustNewConstMetric(c.extra.qdiscStat, prometheus.CounterValue, float64(q.Packets), q.IfName, q.Kind, handle, parent, "packets")
					ch <- prometheus.MustNewConstMetric(c.extra.qdiscStat, prometheus.CounterValue, float64(q.Drops), q.IfName, q.Kind, handle, parent, "drops")
					ch <- prometheus.MustNewConstMetric(c.extra.qdiscQueue, prometheus.GaugeValue, float64(q.Qlen), q.IfName, q.Kind, handle, parent, "qlen")
					ch <- prometheus.MustNewConstMetric(c.extra.qdiscQueue, prometheus.GaugeValue, float64(q.Backlog), q.IfName, q.Kind, handle, parent, "backlog")
				}
			}
		}
	}

	if c.feat.Softnet {
		if rows, err := readSoftnet(c.procfs); err == nil {
			for _, r := range rows {
				cpu := strconv.Itoa(r.CPU)
				ch <- prometheus.MustNewConstMetric(c.extra.softnet, prometheus.CounterValue, float64(r.Processed), cpu, "processed")
				ch <- prometheus.MustNewConstMetric(c.extra.softnet, prometheus.CounterValue, float64(r.Dropped), cpu, "dropped")
				ch <- prometheus.MustNewConstMetric(c.extra.softnet, prometheus.CounterValue, float64(r.TimeSqueeze), cpu, "time_squeeze")
				ch <- prometheus.MustNewConstMetric(c.extra.softnet, prometheus.CounterValue, float64(r.ReceivedRPS), cpu, "received_rps")
				ch <- prometheus.MustNewConstMetric(c.extra.softnet, prometheus.CounterValue, float64(r.FlowLimitCount), cpu, "flow_limit_count")
			}
		}
	}
	if c.feat.StackSNMP {
		if rows, err := readProcSNMP(c.procfs); err == nil {
			for _, r := range rows {
				ch <- prometheus.MustNewConstMetric(c.extra.stackStat, prometheus.UntypedValue, float64(r.Value), r.Protocol, r.Name)
			}
		}
	}

	if c.feat.RDMA {
		if devs, err := readRDMA(c.sysfs); err == nil {
			for _, d := range devs {
				ch <- prometheus.MustNewConstMetric(c.extra.rdmaInfo, prometheus.GaugeValue, 1,
					d.Name, d.NodeType, d.FWVer, d.HCAType, d.NodeGUID, d.BoardID)
				for _, p := range d.Ports {
					ch <- prometheus.MustNewConstMetric(c.extra.rdmaPort, prometheus.GaugeValue, 1,
						d.Name, p.Port, p.State, p.PhysState, p.Rate, p.LinkLayer, p.LID, p.SMLID)
					for k, v := range p.Counters {
						ch <- prometheus.MustNewConstMetric(c.extra.rdmaCounter, prometheus.CounterValue, float64(v),
							d.Name, p.Port, "counters", k)
					}
					for k, v := range p.HWCounters {
						ch <- prometheus.MustNewConstMetric(c.extra.rdmaCounter, prometheus.CounterValue, float64(v),
							d.Name, p.Port, "hw_counters", k)
					}
				}
			}
		}
	}

	if c.feat.Conntrack {
		if ct, err := readConntrack(c.procfs); err == nil {
			if ct.HasCount {
				ch <- prometheus.MustNewConstMetric(c.extra.ctEntries, prometheus.GaugeValue, float64(ct.Count))
			}
			if ct.HasMax {
				ch <- prometheus.MustNewConstMetric(c.extra.ctMax, prometheus.GaugeValue, float64(ct.Max))
			}
			for cpu, row := range ct.PerCPU {
				for i, name := range ct.StatNames {
					if i >= len(row) {
						break
					}
					ch <- prometheus.MustNewConstMetric(c.extra.ctStat, prometheus.CounterValue, float64(row[i]),
						strconv.Itoa(cpu), name)
				}
			}
		}
	}

	if c.feat.Nftables && c.nft != nil && c.nftCache != nil {
		snap, err, _ := c.nftCache.getOr("nft", func() (netfilter.Snapshot, error) {
			return c.nft.Dump()
		})
		if err == nil {
			for _, t := range snap.Tables {
				ch <- prometheus.MustNewConstMetric(c.extra.nftTable, prometheus.GaugeValue, 1, t.Family, t.Name)
			}
			for _, chain := range snap.Chains {
				ch <- prometheus.MustNewConstMetric(c.extra.nftChain, prometheus.GaugeValue, 1, chain.Family, chain.Table, chain.Name)
			}
			for _, ctr := range snap.Counters {
				ch <- prometheus.MustNewConstMetric(c.extra.nftCounter, prometheus.CounterValue, float64(ctr.Bytes),
					ctr.Family, ctr.Table, ctr.Name, "bytes")
				ch <- prometheus.MustNewConstMetric(c.extra.nftCounter, prometheus.CounterValue, float64(ctr.Packets),
					ctr.Family, ctr.Table, ctr.Name, "packets")
			}
		} else {
			c.logger.Debug("nftables dump", "err", err)
		}
	}

	if c.feat.Multicast {
		if groups, err := readIGMP(c.procfs); err == nil {
			seen := map[string]struct{}{}
			for _, g := range groups {
				if g.Device == "" || g.Group == "" || !c.filter.Match(g.Device) {
					continue
				}
				key := g.Device + "|" + g.Group + "|" + g.Querier
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
				ch <- prometheus.MustNewConstMetric(c.extra.igmpGroup, prometheus.GaugeValue, float64(g.Users),
					g.Device, g.Group, g.Querier)
			}
		}
		if groups, err := readMLD(c.procfs); err == nil {
			seen := map[string]struct{}{}
			for _, g := range groups {
				if g.Device == "" || g.Group == "" || !c.filter.Match(g.Device) {
					continue
				}
				key := g.Device + "|" + g.Group + "|" + g.Flags
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
				ch <- prometheus.MustNewConstMetric(c.extra.mldGroup, prometheus.GaugeValue, float64(g.Users),
					g.Device, g.Group, g.Flags)
			}
		}
		if entries, err := readDevMcast(c.procfs); err == nil {
			for _, e := range entries {
				if !c.filter.Match(e.Device) {
					continue
				}
				ch <- prometheus.MustNewConstMetric(c.extra.devMcast, prometheus.GaugeValue, float64(e.RefCount),
					e.Device, e.Address, strconv.FormatUint(e.Bound, 10))
			}
		}
	}
}
