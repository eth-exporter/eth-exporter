package collector

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// IGMPGroup is an IPv4 multicast group membership from /proc/net/igmp.
type IGMPGroup struct {
	Device  string
	Group   string
	Users   uint64
	Querier string
}

// MLDGroup is an IPv6 multicast group from /proc/net/igmp6.
type MLDGroup struct {
	Device string
	Group  string
	Users  uint64
	Flags  string
}

// DevMcastEntry is a link-layer multicast address from /proc/net/dev_mcast.
type DevMcastEntry struct {
	Index    int
	Device   string
	RefCount uint64
	Bound    uint64
	Address  string
}

func readIGMP(procfs string) ([]IGMPGroup, error) {
	f, err := os.Open(filepath.Join(procfs, "net", "igmp"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []IGMPGroup
	var curDev, querier string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		// Header lines like: "Idx Device        : Count Querier" or "1 eth0  : 2 V"
		if strings.Contains(line, "Device") && strings.Contains(line, "Querier") && !strings.Contains(line, ".") {
			continue
		}
		// Device line: "<idx> <dev> : <count> <querier>"
		if len(fields) >= 4 && fields[2] == ":" {
			curDev = fields[1]
			querier = fields[len(fields)-1]
			continue
		}
		// Group line (indented): "<group_hex> <users> ..."
		if curDev == "" || len(fields) < 2 {
			continue
		}
		group := formatIPv4Hex(fields[0])
		users, _ := strconv.ParseUint(fields[1], 10, 64)
		out = append(out, IGMPGroup{Device: curDev, Group: group, Users: users, Querier: querier})
	}
	return out, sc.Err()
}

func readMLD(procfs string) ([]MLDGroup, error) {
	f, err := os.Open(filepath.Join(procfs, "net", "igmp6"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []MLDGroup
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		// Format: index device address users flags ...
		if len(fields) < 4 {
			continue
		}
		if fields[0] == "Idx" || fields[0] == "index" {
			continue
		}
		dev := fields[1]
		addr := formatIPv6Hex(fields[2])
		users, _ := strconv.ParseUint(fields[3], 10, 64)
		flags := ""
		if len(fields) > 4 {
			flags = fields[4]
		}
		out = append(out, MLDGroup{Device: dev, Group: addr, Users: users, Flags: flags})
	}
	return out, sc.Err()
}

func readDevMcast(procfs string) ([]DevMcastEntry, error) {
	f, err := os.Open(filepath.Join(procfs, "net", "dev_mcast"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []DevMcastEntry
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		// idx interface_name refcount bound addr
		if len(fields) < 5 {
			continue
		}
		idx, _ := strconv.Atoi(fields[0])
		ref, _ := strconv.ParseUint(fields[2], 10, 64)
		bound, _ := strconv.ParseUint(fields[3], 10, 64)
		addr := formatMACHex(fields[4])
		out = append(out, DevMcastEntry{
			Index: idx, Device: fields[1], RefCount: ref, Bound: bound, Address: addr,
		})
	}
	return out, sc.Err()
}

func formatIPv4Hex(s string) string {
	// Often already dotted; /proc/net/igmp prints ntohl(addr) as %08X (host order).
	if strings.Contains(s, ".") {
		return s
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return s
	}
	return strconv.FormatUint(v&0xff, 10) + "." +
		strconv.FormatUint((v>>8)&0xff, 10) + "." +
		strconv.FormatUint((v>>16)&0xff, 10) + "." +
		strconv.FormatUint((v>>24)&0xff, 10)
}

func formatIPv6Hex(s string) string {
	s = strings.TrimSpace(s)
	if strings.Contains(s, ":") {
		return s
	}
	// 32 hex chars without separators
	if len(s) != 32 {
		return s
	}
	var parts []string
	for i := 0; i < 32; i += 4 {
		parts = append(parts, s[i:i+4])
	}
	return strings.Join(parts, ":")
}

func formatMACHex(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if strings.Contains(s, ":") {
		return s
	}
	if len(s) != 12 {
		return s
	}
	var b strings.Builder
	for i := 0; i < 12; i += 2 {
		if i > 0 {
			b.WriteByte(':')
		}
		b.WriteString(s[i : i+2])
	}
	return b.String()
}
