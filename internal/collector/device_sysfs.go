package collector

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// DeviceIdentity is PCI/platform identity from sysfs device links.
type DeviceIdentity struct {
	DevicePath      string
	Driver          string
	Vendor          string
	Device          string
	SubsystemVendor string
	SubsystemDevice string
	NUMANode        string
	IRQ             string
	LocalCPUs       string
	SRIOVNumVFs     int64
	HasSRIOV        bool
}

// QueueInfo summarizes netdev queue topology.
type QueueInfo struct {
	RXQueues int
	TXQueues int
}

// BondInfo is bonding slave/master state.
type BondInfo struct {
	IsMaster    bool
	IsSlave     bool
	Mode        string
	ActiveSlave string
	MIIStatus   string
	SlaveState  string
	Primary     string
	LacpPartner string
}

// BridgePortInfo is bridge port sysfs state.
type BridgePortInfo struct {
	IsPort     bool
	State      string
	Priority   string
	PortNumber string
	Hairpin    string
	Cost       string
}

// VFInfo is one SR-IOV virtual function.
type VFInfo struct {
	Index string
	MAC   string
	VLAN  string
	Spoof string
	Trust string
	State string
}

func readDeviceIdentity(sysfs, iface string) (DeviceIdentity, error) {
	base := filepath.Join(sysfs, "class", "net", iface, "device")
	var id DeviceIdentity
	if _, err := os.Stat(base); err != nil {
		return id, err
	}
	if p, err := filepath.EvalSymlinks(base); err == nil {
		id.DevicePath = p
	}
	id.Driver = readSymlinkBase(filepath.Join(base, "driver"))
	id.Vendor = readStr(base, "vendor")
	id.Device = readStr(base, "device")
	id.SubsystemVendor = readStr(base, "subsystem_vendor")
	id.SubsystemDevice = readStr(base, "subsystem_device")
	id.NUMANode = readStr(base, "numa_node")
	id.IRQ = readStr(base, "irq")
	id.LocalCPUs = readStr(base, "local_cpulist")
	if s := readStr(base, "sriov_numvfs"); s != "" {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil {
			id.SRIOVNumVFs = v
			id.HasSRIOV = true
		}
	}
	return id, nil
}

func readQueueInfo(sysfs, iface string) QueueInfo {
	var q QueueInfo
	rx, _ := os.ReadDir(filepath.Join(sysfs, "class", "net", iface, "queues"))
	for _, e := range rx {
		name := e.Name()
		if strings.HasPrefix(name, "rx-") {
			q.RXQueues++
		}
		if strings.HasPrefix(name, "tx-") {
			q.TXQueues++
		}
	}
	return q
}

func readBondInfo(sysfs, iface string) BondInfo {
	var b BondInfo
	master := filepath.Join(sysfs, "class", "net", iface, "bonding")
	if st, err := os.Stat(master); err == nil && st.IsDir() {
		b.IsMaster = true
		b.Mode = readStr(master, "mode")
		b.ActiveSlave = readStr(master, "active_slave")
		b.MIIStatus = readStr(master, "mii_status")
		b.Primary = readStr(master, "primary")
	}
	slave := filepath.Join(sysfs, "class", "net", iface, "bonding_slave")
	if st, err := os.Stat(slave); err == nil && st.IsDir() {
		b.IsSlave = true
		b.SlaveState = readStr(slave, "state")
		b.MIIStatus = firstNonEmpty(b.MIIStatus, readStr(slave, "mii_status"))
		b.LacpPartner = readStr(slave, "ad_partner_mac")
	}
	return b
}

func readBridgePort(sysfs, iface string) BridgePortInfo {
	var b BridgePortInfo
	dir := filepath.Join(sysfs, "class", "net", iface, "brport")
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return b
	}
	b.IsPort = true
	b.State = readStr(dir, "state")
	b.Priority = readStr(dir, "priority")
	b.PortNumber = readStr(dir, "port_no")
	b.Hairpin = readStr(dir, "hairpin_mode")
	b.Cost = readStr(dir, "path_cost")
	return b
}

func readVFs(sysfs, iface string) []VFInfo {
	dir := filepath.Join(sysfs, "class", "net", iface, "device", "sriov")
	// Some kernels: /sys/class/net/X/device/virtfnN
	dev := filepath.Join(sysfs, "class", "net", iface, "device")
	entries, err := os.ReadDir(dev)
	if err != nil {
		_ = dir
		return nil
	}
	var out []VFInfo
	// Prefer netdev vf info under /sys/class/net/X/device/sriov_numvfs + netlink;
	// also read /sys/class/net/X/queues is separate. VF MAC via physfn/virtfn.
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "virtfn") {
			continue
		}
		vfPath := filepath.Join(dev, name)
		idx := strings.TrimPrefix(name, "virtfn")
		net := filepath.Join(vfPath, "net")
		mac := ""
		if nets, err := os.ReadDir(net); err == nil && len(nets) > 0 {
			mac = readStr(filepath.Join(net, nets[0].Name()), "address")
		}
		out = append(out, VFInfo{
			Index: idx,
			MAC:   mac,
			VLAN:  readStr(vfPath, "sriov" /* placeholder */),
		})
	}
	// Also try modern netdev sriov vf attrs if present under class/net/iface
	vfDir := filepath.Join(sysfs, "class", "net", iface)
	_ = vfDir
	return out
}

func readSymlinkBase(path string) string {
	target, err := os.Readlink(path)
	if err != nil {
		return ""
	}
	return filepath.Base(target)
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
