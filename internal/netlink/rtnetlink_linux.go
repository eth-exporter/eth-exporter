//go:build linux

package netlink

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

type Client struct {
	mu  sync.Mutex
	fd  int
	seq uint32
}

func New() (*Client, error) {
	fd, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_RAW, unix.NETLINK_ROUTE)
	if err != nil {
		return nil, err
	}
	if err := unix.Bind(fd, &unix.SockaddrNetlink{Family: unix.AF_NETLINK}); err != nil {
		_ = unix.Close(fd)
		return nil, err
	}
	_ = unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &unix.Timeval{Sec: 2})
	// Large dumps (FDB/qdisc/addr) need headroom to avoid silent truncation.
	_ = unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_RCVBUF, 4<<20)
	return &Client{fd: fd}, nil
}

func (c *Client) Close() error {
	if c.fd >= 0 {
		err := unix.Close(c.fd)
		c.fd = -1
		return err
	}
	return nil
}

func (c *Client) nextSeq() uint32 {
	c.seq++
	if c.seq == 0 {
		c.seq = 1
	}
	return c.seq
}

type AddrInfo struct {
	IfIndex int
	IfName  string
	Family  string
	Address string
	Prefix  int
	Scope   int
	Flags   uint32
}

type NeighInfo struct {
	IfIndex  int
	IfName   string
	Family   string
	IP       string
	LLAddr   string
	State    uint16
	NudState string
}

type FDBEntry struct {
	IfIndex int
	IfName  string
	MAC     string
	VLAN    uint16
	Master  int
	State   uint16
}

type LinkInfo struct {
	Index       int
	Name        string
	PermAddr    string
	XDPAttached bool
	XDPProgID   uint32
	OperState   uint8
	MTU         uint32
}

type QdiscInfo struct {
	IfIndex    int
	IfName     string
	Kind       string
	Handle     uint32
	Parent     uint32
	Bytes      uint64
	Packets    uint64
	Drops      uint64
	Overlimits uint64
	Qlen       uint32
	Backlog    uint32
}

func (c *Client) dump(msgType uint16, family uint8, payload []byte) ([][]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	const nlHdr = 16
	total := nlHdr + len(payload)
	req := make([]byte, total)
	binary.LittleEndian.PutUint32(req[0:], uint32(total))
	binary.LittleEndian.PutUint16(req[4:], msgType)
	binary.LittleEndian.PutUint16(req[6:], unix.NLM_F_REQUEST|unix.NLM_F_DUMP)
	seq := c.nextSeq()
	binary.LittleEndian.PutUint32(req[8:], seq)
	binary.LittleEndian.PutUint32(req[12:], uint32(os.Getpid()))
	copy(req[16:], payload)
	_ = family
	if _, err := unix.Write(c.fd, req); err != nil {
		return nil, err
	}
	var out [][]byte
	buf := make([]byte, 1<<16)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		nr, err := unix.Read(c.fd, buf)
		if err != nil {
			// Do not treat EAGAIN/timeout as end-of-dump: multi-message dumps
			// often pause between recv calls before NLMSG_DONE.
			if err == syscall.EAGAIN || err == unix.EAGAIN || err == unix.EINTR || os.IsTimeout(err) {
				continue
			}
			return out, err
		}
		msgs, done, err := parseNL(buf[:nr])
		if err != nil {
			return out, err
		}
		out = append(out, msgs...)
		if done {
			return out, nil
		}
	}
	return out, nil
}

func parseNL(b []byte) (payloads [][]byte, done bool, err error) {
	for len(b) >= 16 {
		l := int(binary.LittleEndian.Uint32(b[0:4]))
		if l < 16 || l > len(b) {
			return payloads, done, fmt.Errorf("bad nlmsg")
		}
		typ := binary.LittleEndian.Uint16(b[4:6])
		msg := b[:l]
		aligned := (l + 3) & ^3
		if aligned > len(b) {
			aligned = len(b)
		}
		b = b[aligned:]
		switch typ {
		case unix.NLMSG_DONE:
			return payloads, true, nil
		case unix.NLMSG_ERROR:
			if l >= 20 {
				code := int32(binary.LittleEndian.Uint32(msg[16:20]))
				if code == 0 {
					return payloads, true, nil
				}
				return payloads, true, syscall.Errno(-code)
			}
			return payloads, true, fmt.Errorf("nl error")
		default:
			// Copy out of the receive buffer; the caller reuses it on the next Read.
			payload := make([]byte, l-16)
			copy(payload, msg[16:l])
			payloads = append(payloads, payload)
		}
	}
	return payloads, done, nil
}

func (c *Client) Links() ([]LinkInfo, error) {
	// struct ifinfomsg
	payload := make([]byte, 16)
	msgs, err := c.dump(unix.RTM_GETLINK, 0, payload)
	if err != nil {
		return nil, err
	}
	var out []LinkInfo
	for _, m := range msgs {
		if len(m) < 16 {
			continue
		}
		idx := int(binary.LittleEndian.Uint32(m[4:8]))
		li := LinkInfo{Index: idx}
		attrs := parseRtAttrs(m[16:])
		if n, ok := attrs[unix.IFLA_IFNAME]; ok {
			li.Name = cstring(n)
		}
		if a, ok := attrs[unix.IFLA_PERM_ADDRESS]; ok {
			li.PermAddr = net.HardwareAddr(a).String()
		}
		if a, ok := attrs[unix.IFLA_MTU]; ok && len(a) >= 4 {
			li.MTU = binary.LittleEndian.Uint32(a)
		}
		if a, ok := attrs[unix.IFLA_OPERSTATE]; ok && len(a) >= 1 {
			li.OperState = a[0]
		}
		if xdp, ok := attrs[unix.IFLA_XDP]; ok {
			xattrs := parseRtAttrs(xdp)
			if a, ok := xattrs[unix.IFLA_XDP_ATTACHED]; ok && len(a) > 0 && a[0] != 0 {
				li.XDPAttached = true
			}
			if a, ok := xattrs[unix.IFLA_XDP_PROG_ID]; ok && len(a) >= 4 {
				li.XDPProgID = binary.LittleEndian.Uint32(a)
			}
		}
		out = append(out, li)
	}
	return out, nil
}

// LinkIndex maps ifindex → name.
func LinkIndex(links []LinkInfo) map[int]string {
	byIdx := make(map[int]string, len(links))
	for _, l := range links {
		byIdx[l.Index] = l.Name
	}
	return byIdx
}

func (c *Client) Addresses(byIdx map[int]string) ([]AddrInfo, error) {
	if byIdx == nil {
		links, _ := c.Links()
		byIdx = LinkIndex(links)
	}
	var out []AddrInfo
	// Dump IPv4 and IPv6 separately for more reliable complete results.
	for _, fam := range []uint8{unix.AF_INET, unix.AF_INET6} {
		payload := make([]byte, 8) // ifaddrmsg
		payload[0] = fam
		msgs, err := c.dump(unix.RTM_GETADDR, fam, payload)
		if err != nil {
			continue
		}
		for _, m := range msgs {
			if len(m) < 8 {
				continue
			}
			family := m[0]
			prefix := int(m[1])
			flags := uint32(m[2])
			scope := int(m[3])
			idx := int(binary.LittleEndian.Uint32(m[4:8]))
			attrs := parseRtAttrs(m[8:])
			// Prefer IFA_LOCAL (interface address); fall back to IFA_ADDRESS.
			var addr []byte
			if a, ok := attrs[unix.IFA_LOCAL]; ok {
				addr = a
			} else if a, ok := attrs[unix.IFA_ADDRESS]; ok {
				addr = a
			}
			name := byIdx[idx]
			if name == "" {
				if a, ok := attrs[unix.IFA_LABEL]; ok {
					name = cstring(a)
				}
			}
			ai := AddrInfo{
				IfIndex: idx,
				IfName:  name,
				Prefix:  prefix,
				Scope:   scope,
				Flags:   flags,
			}
			switch family {
			case unix.AF_INET:
				ai.Family = "inet"
				if len(addr) >= 4 {
					ai.Address = net.IP(addr[:4]).String()
				}
			case unix.AF_INET6:
				ai.Family = "inet6"
				if len(addr) >= 16 {
					ai.Address = net.IP(addr[:16]).String()
				}
			default:
				continue
			}
			if ai.Address == "" || ai.IfName == "" {
				continue
			}
			out = append(out, ai)
		}
	}
	return out, nil
}

func (c *Client) Neighbors(byIdx map[int]string) ([]NeighInfo, error) {
	payload := make([]byte, 12) // ndmsg
	msgs, err := c.dump(unix.RTM_GETNEIGH, 0, payload)
	if err != nil {
		return nil, err
	}
	if byIdx == nil {
		links, _ := c.Links()
		byIdx = LinkIndex(links)
	}
	var out []NeighInfo
	for _, m := range msgs {
		if len(m) < 12 {
			continue
		}
		family := m[0]
		idx := int(binary.LittleEndian.Uint32(m[4:8]))
		state := binary.LittleEndian.Uint16(m[8:10])
		attrs := parseRtAttrs(m[12:])
		ni := NeighInfo{
			IfIndex:  idx,
			IfName:   byIdx[idx],
			State:    state,
			NudState: nudName(state),
		}
		if a, ok := attrs[unix.NDA_LLADDR]; ok {
			ni.LLAddr = net.HardwareAddr(a).String()
		}
		dst, ok := attrs[unix.NDA_DST]
		if !ok {
			continue
		}
		switch family {
		case unix.AF_INET:
			ni.Family = "inet"
			if len(dst) >= 4 {
				ni.IP = net.IP(dst[:4]).String()
			}
		case unix.AF_INET6:
			ni.Family = "inet6"
			if len(dst) >= 16 {
				ni.IP = net.IP(dst[:16]).String()
			}
		default:
			continue
		}
		out = append(out, ni)
	}
	return out, nil
}

func (c *Client) FDB(byIdx map[int]string) ([]FDBEntry, error) {
	// ndmsg with AF_BRIDGE
	payload := make([]byte, 12)
	payload[0] = unix.AF_BRIDGE
	msgs, err := c.dump(unix.RTM_GETNEIGH, unix.AF_BRIDGE, payload)
	if err != nil {
		return nil, err
	}
	if byIdx == nil {
		links, _ := c.Links()
		byIdx = LinkIndex(links)
	}
	var out []FDBEntry
	for _, m := range msgs {
		if len(m) < 12 {
			continue
		}
		if m[0] != unix.AF_BRIDGE {
			continue
		}
		idx := int(binary.LittleEndian.Uint32(m[4:8]))
		state := binary.LittleEndian.Uint16(m[8:10])
		attrs := parseRtAttrs(m[12:])
		e := FDBEntry{IfIndex: idx, IfName: byIdx[idx], State: state}
		if a, ok := attrs[unix.NDA_LLADDR]; ok {
			e.MAC = net.HardwareAddr(a).String()
		}
		if a, ok := attrs[unix.NDA_VLAN]; ok && len(a) >= 2 {
			e.VLAN = binary.LittleEndian.Uint16(a)
		}
		if a, ok := attrs[unix.NDA_MASTER]; ok && len(a) >= 4 {
			e.Master = int(binary.LittleEndian.Uint32(a))
		}
		if e.MAC == "" {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

// TCA_* from linux/pkt_cls.h / pkt_sched.h (not always in x/sys/unix).
const (
	tcaKind    = 1
	tcaOptions = 2
	tcaStats   = 3
	tcaXstats  = 4
	tcaRate    = 5
	tcaFcnt    = 6
	tcaStats2  = 7
	tcaStab    = 8

	tcaStatsBasic   = 1
	tcaStatsRateEst = 2
	tcaStatsQueue   = 3
)

func (c *Client) Qdiscs(byIdx map[int]string) ([]QdiscInfo, error) {
	payload := make([]byte, 20) // tcmsg
	msgs, err := c.dump(unix.RTM_GETQDISC, 0, payload)
	if err != nil {
		return nil, err
	}
	if byIdx == nil {
		links, _ := c.Links()
		byIdx = LinkIndex(links)
	}
	var out []QdiscInfo
	for _, m := range msgs {
		if len(m) < 20 {
			continue
		}
		idx := int(binary.LittleEndian.Uint32(m[4:8]))
		handle := binary.LittleEndian.Uint32(m[8:12])
		parent := binary.LittleEndian.Uint32(m[12:16])
		attrs := parseRtAttrs(m[20:])
		q := QdiscInfo{IfIndex: idx, IfName: byIdx[idx], Handle: handle, Parent: parent}
		if a, ok := attrs[tcaKind]; ok {
			q.Kind = cstring(a)
		}
		if a, ok := attrs[tcaStats2]; ok {
			parseTCAStats2(&q, a)
		} else if a, ok := attrs[tcaStats]; ok && len(a) >= 16 {
			q.Bytes = binary.LittleEndian.Uint64(a[0:8])
			if len(a) >= 12 {
				q.Packets = uint64(binary.LittleEndian.Uint32(a[8:12]))
			}
		}
		_ = tcaOptions
		_ = tcaXstats
		_ = tcaRate
		_ = tcaFcnt
		_ = tcaStab
		_ = tcaStatsRateEst
		out = append(out, q)
	}
	return out, nil
}

func parseTCAStats2(q *QdiscInfo, raw []byte) {
	attrs := parseRtAttrs(raw)
	if a, ok := attrs[tcaStatsBasic]; ok && len(a) >= 16 {
		q.Bytes = binary.LittleEndian.Uint64(a[0:8])
		q.Packets = binary.LittleEndian.Uint64(a[8:16])
	}
	if a, ok := attrs[tcaStatsQueue]; ok && len(a) >= 16 {
		q.Qlen = binary.LittleEndian.Uint32(a[0:4])
		q.Backlog = binary.LittleEndian.Uint32(a[4:8])
		q.Drops = uint64(binary.LittleEndian.Uint32(a[8:12]))
		q.Overlimits = uint64(binary.LittleEndian.Uint32(a[12:16]))
	}
}

func parseRtAttrs(b []byte) map[uint16][]byte {
	out := make(map[uint16][]byte)
	for len(b) >= 4 {
		l := int(binary.LittleEndian.Uint16(b[0:2]))
		t := binary.LittleEndian.Uint16(b[2:4])
		if l < 4 || l > len(b) {
			break
		}
		out[t] = b[4:l]
		aligned := (l + 3) & ^3
		if aligned > len(b) {
			break
		}
		b = b[aligned:]
	}
	return out
}

func cstring(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

func nudName(s uint16) string {
	switch {
	case s&unix.NUD_REACHABLE != 0:
		return "reachable"
	case s&unix.NUD_STALE != 0:
		return "stale"
	case s&unix.NUD_DELAY != 0:
		return "delay"
	case s&unix.NUD_PROBE != 0:
		return "probe"
	case s&unix.NUD_FAILED != 0:
		return "failed"
	case s&unix.NUD_PERMANENT != 0:
		return "permanent"
	case s&unix.NUD_NOARP != 0:
		return "noarp"
	default:
		return fmt.Sprintf("0x%x", s)
	}
}
