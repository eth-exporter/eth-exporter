package collector

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// InterfaceInfo is kernel-exported state for one netdevice under sysfs.
type InterfaceInfo struct {
	Name             string
	IfIndex          int64
	MTU              int64
	OperState        string
	Carrier          int64
	Dormant          int64
	Address          string
	Broadcast        string
	Duplex           string
	Speed            int64 // Mbps; -1 if unknown
	Type             int64 // ARPHRD_*
	Flags            string
	IfAlias          string
	AddrAssign       string
	AddrLen          int64
	TxQueueLen       int64
	Group            string
	ProtoDown        int64
	PhysPortID       string
	PhysPortName     string
	PhysSwitchID     string
	DevID            string
	DevPort          string
	NameAssign       string
	Testing          int64
	CarrierChanges   int64
	CarrierUpCount   int64
	CarrierDownCount int64
}

func listInterfaces(sysfs string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(sysfs, "class", "net"))
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if !validIfaceName(name) {
			continue
		}
		names = append(names, name)
	}
	return names, nil
}

// validIfaceName rejects path components that could escape sysfs/proc joins.
func validIfaceName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	if len(name) >= 16 { // IFNAMSIZ
		return false
	}
	if strings.ContainsAny(name, "/\\\x00") {
		return false
	}
	return true
}

func readInterface(sysfs, name string) (InterfaceInfo, error) {
	if !validIfaceName(name) {
		return InterfaceInfo{}, os.ErrInvalid
	}
	base := filepath.Join(sysfs, "class", "net", name)
	info := InterfaceInfo{Name: name, Speed: -1}
	info.IfIndex = readInt(base, "ifindex")
	info.MTU = readInt(base, "mtu")
	info.OperState = readStr(base, "operstate")
	info.Carrier = readInt(base, "carrier")
	info.Dormant = readInt(base, "dormant")
	info.Address = readStr(base, "address")
	info.Broadcast = readStr(base, "broadcast")
	info.Duplex = readStr(base, "duplex")
	if s := readStr(base, "speed"); s != "" {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil {
			info.Speed = v
		}
	}
	info.Type = readInt(base, "type")
	info.Flags = readStr(base, "flags")
	info.IfAlias = readStr(base, "ifalias")
	info.AddrAssign = readStr(base, "addr_assign_type")
	info.AddrLen = readInt(base, "addr_len")
	info.TxQueueLen = readInt(base, "tx_queue_len")
	info.Group = readStr(base, "group")
	info.ProtoDown = readInt(base, "proto_down")
	info.PhysPortID = readStr(base, "phys_port_id")
	info.PhysPortName = readStr(base, "phys_port_name")
	info.PhysSwitchID = readStr(base, "phys_switch_id")
	info.DevID = readStr(base, "dev_id")
	info.DevPort = readStr(base, "dev_port")
	info.NameAssign = readStr(base, "name_assign_type")
	info.Testing = readInt(base, "testing")
	info.CarrierChanges = readInt(base, "carrier_changes")
	info.CarrierUpCount = readInt(base, "carrier_up_count")
	info.CarrierDownCount = readInt(base, "carrier_down_count")
	return info, nil
}

func readStatistics(sysfs, name string) (map[string]uint64, error) {
	dir := filepath.Join(sysfs, "class", "net", name, "statistics")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make(map[string]uint64, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		v, err := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
		if err != nil {
			continue
		}
		out[e.Name()] = v
	}
	return out, nil
}

func readStr(base, key string) string {
	b, err := os.ReadFile(filepath.Join(base, key))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func readInt(base, key string) int64 {
	s := readStr(base, key)
	if s == "" {
		return 0
	}
	v, err := strconv.ParseInt(s, 0, 64)
	if err != nil {
		return 0
	}
	return v
}
