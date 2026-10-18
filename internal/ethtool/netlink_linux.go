//go:build linux

package ethtool

import (
	"encoding/binary"
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

const (
	genlIDCtrl = 0x10

	ctrlCmdGetFamily   = 3
	ctrlAttrFamilyID   = 1
	ctrlAttrFamilyName = 2

	ethtoolGenlName    = "ethtool"
	ethtoolGenlVersion = 1

	ethMsgStatsGet = 32 // ETHTOOL_MSG_STATS_GET
	ethMsgPauseGet = 21 // ETHTOOL_MSG_PAUSE_GET
	ethMsgFECGet   = 29 // ETHTOOL_MSG_FEC_GET

	ethFlagStats = 4

	ethAHeaderDevName = 2
	ethAHeaderFlags   = 3

	ethAStatsHeader = 2
	ethAStatsGroups = 3
	ethAStatsGrp    = 4

	ethAStatsGrpID      = 2
	ethAStatsGrpSSID    = 3
	ethAStatsGrpStat    = 4
	ethAStatsGrpHistRX  = 5
	ethAStatsGrpHistTX  = 6
	ethAStatsGrpHistLo  = 7
	ethAStatsGrpHistHi  = 8
	ethAStatsGrpHistVal = 9

	ethABitsetNomask = 1
	ethABitsetSize   = 2
	ethABitsetValue  = 4

	ethAPauseHeader    = 1
	ethAPauseStats     = 5
	ethAPauseStatTX    = 2
	ethAPauseStatRX    = 3
	ethAPauseStatStorm = 4

	ethAFECHeaderAttr    = 1
	ethAFECStatsAttr     = 5
	ethAFECStatCorrected = 2
	ethAFECStatUncorr    = 3
	ethAFECStatCorrBits  = 4

	nlaFNested = 1 << 15
	nlaAlign   = 4
)

// Netlink talks to the ethtool genetlink family.
type Netlink struct {
	fd       int
	familyID uint16
	seq      uint32
	mu       sync.Mutex
}

func NewNetlink() (*Netlink, error) {
	fd, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_RAW, unix.NETLINK_GENERIC)
	if err != nil {
		return nil, err
	}
	if err := unix.Bind(fd, &unix.SockaddrNetlink{Family: unix.AF_NETLINK}); err != nil {
		_ = unix.Close(fd)
		return nil, err
	}
	_ = unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &unix.Timeval{Sec: 2})
	n := &Netlink{fd: fd}
	id, err := n.resolveFamily(ethtoolGenlName)
	if err != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("resolve ethtool genetlink: %w", err)
	}
	n.familyID = id
	return n, nil
}

func (n *Netlink) Close() error {
	if n.fd >= 0 {
		err := unix.Close(n.fd)
		n.fd = -1
		return err
	}
	return nil
}

func (n *Netlink) nextSeqLocked() uint32 {
	n.seq++
	if n.seq == 0 {
		n.seq = 1
	}
	return n.seq
}

func (n *Netlink) resolveFamily(name string) (uint16, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	var b nlBuilder
	b.putString(ctrlAttrFamilyName, name)
	msg := b.finish(genlIDCtrl, ctrlCmdGetFamily, 1, n.nextSeqLocked())
	replies, err := n.transactLocked(msg)
	if err != nil {
		return 0, err
	}
	for _, r := range replies {
		attrs, err := parseAttrs(r)
		if err != nil {
			continue
		}
		if id, ok := attrs.uint16(ctrlAttrFamilyID); ok {
			return id, nil
		}
	}
	return 0, fmt.Errorf("family %q not found", name)
}

func (n *Netlink) StandardStats(dev string) (StdStats, error) {
	out := StdStats{Groups: make(map[string]map[string]uint64)}
	n.mu.Lock()
	defer n.mu.Unlock()

	var body nlBuilder
	// header nest
	var hdr nlBuilder
	hdr.putString(ethAHeaderDevName, dev)
	body.putNested(ethAStatsHeader, hdr.bytes())

	// groups bitset: all __ETHTOOL_STATS_CNT bits
	const nGroups = 5
	var groups nlBuilder
	groups.putFlag(ethABitsetNomask)
	groups.putUint32(ethABitsetSize, nGroups)
	val := make([]byte, 4)
	binary.LittleEndian.PutUint32(val, (1<<nGroups)-1)
	groups.putBytes(ethABitsetValue, val)
	body.putNested(ethAStatsGroups, groups.bytes())

	msg := body.finish(n.familyID, ethMsgStatsGet, ethtoolGenlVersion, n.nextSeqLocked())
	replies, err := n.transactLocked(msg)
	if err != nil {
		return out, err
	}
	for _, r := range replies {
		attrs, err := parseAttrs(r)
		if err != nil {
			continue
		}
		for _, grp := range attrs.nestedAll(ethAStatsGrp) {
			n.parseStatsGrp(&out, grp)
		}
	}
	return out, nil
}

func (n *Netlink) parseStatsGrp(out *StdStats, raw []byte) {
	attrs, err := parseAttrs(raw)
	if err != nil {
		return
	}
	gid, ok := attrs.uint32(ethAStatsGrpID)
	if !ok {
		return
	}
	gname, ok := stdGroupNames[gid]
	if !ok {
		gname = fmt.Sprintf("group_%d", gid)
	}
	if out.Groups[gname] == nil {
		out.Groups[gname] = make(map[string]uint64)
	}
	names := stdStatNames[gid]

	for _, nest := range attrs.nestedAll(ethAStatsGrpStat) {
		// Nested nlattr: type=stat index, payload=u64
		if len(nest) < 4 {
			continue
		}
		inner, err := parseAttrs(nest)
		if err != nil || len(inner.order) == 0 {
			// nest itself may BE the typed u64 attr (single attr buffer)
			a, err := parseOneAttr(nest)
			if err != nil {
				continue
			}
			if len(a.data) < 8 {
				continue
			}
			val := binary.LittleEndian.Uint64(a.data[:8])
			name := names[a.typ]
			if name == "" {
				name = fmt.Sprintf("stat_%d", a.typ)
			}
			out.Groups[gname][name] = val
			continue
		}
		for _, a := range inner.order {
			if len(a.data) < 8 {
				continue
			}
			val := binary.LittleEndian.Uint64(a.data[:8])
			name := names[a.typ]
			if name == "" {
				name = fmt.Sprintf("stat_%d", a.typ)
			}
			out.Groups[gname][name] = val
		}
	}

	for _, h := range attrs.nestedAll(ethAStatsGrpHistRX) {
		if b, ok := parseHist(h); ok {
			out.RMONHistRX = append(out.RMONHistRX, b)
		}
	}
	for _, h := range attrs.nestedAll(ethAStatsGrpHistTX) {
		if b, ok := parseHist(h); ok {
			out.RMONHistTX = append(out.RMONHistTX, b)
		}
	}
}

func parseHist(raw []byte) (HistBucket, bool) {
	attrs, err := parseAttrs(raw)
	if err != nil {
		return HistBucket{}, false
	}
	lo, ok1 := attrs.uint32(ethAStatsGrpHistLo)
	hi, ok2 := attrs.uint32(ethAStatsGrpHistHi)
	val, ok3 := attrs.uint64(ethAStatsGrpHistVal)
	if !ok1 || !ok2 || !ok3 {
		return HistBucket{}, false
	}
	return HistBucket{Low: lo, High: hi, Val: val}, true
}

func (n *Netlink) PauseStats(dev string) (PauseStats, error) {
	var out PauseStats
	n.mu.Lock()
	defer n.mu.Unlock()
	var body nlBuilder
	var hdr nlBuilder
	hdr.putString(ethAHeaderDevName, dev)
	hdr.putUint32(ethAHeaderFlags, ethFlagStats)
	body.putNested(ethAPauseHeader, hdr.bytes())

	msg := body.finish(n.familyID, ethMsgPauseGet, ethtoolGenlVersion, n.nextSeqLocked())
	replies, err := n.transactLocked(msg)
	if err != nil {
		return out, err
	}
	for _, r := range replies {
		attrs, err := parseAttrs(r)
		if err != nil {
			continue
		}
		statsRaw, ok := attrs.bytes(ethAPauseStats)
		if !ok {
			continue
		}
		sattrs, err := parseAttrs(statsRaw)
		if err != nil {
			continue
		}
		if v, ok := sattrs.uint64(ethAPauseStatTX); ok {
			out.TXFrames, out.HasTXFrames = v, true
		}
		if v, ok := sattrs.uint64(ethAPauseStatRX); ok {
			out.RXFrames, out.HasRXFrames = v, true
		}
		if v, ok := sattrs.uint64(ethAPauseStatStorm); ok {
			out.TXPauseStorms, out.HasTXStorms = v, true
		}
	}
	return out, nil
}

func (n *Netlink) FECStats(dev string) (FECStats, error) {
	var out FECStats
	n.mu.Lock()
	defer n.mu.Unlock()
	var body nlBuilder
	var hdr nlBuilder
	hdr.putString(ethAHeaderDevName, dev)
	hdr.putUint32(ethAHeaderFlags, ethFlagStats)
	body.putNested(ethAFECHeaderAttr, hdr.bytes())

	msg := body.finish(n.familyID, ethMsgFECGet, ethtoolGenlVersion, n.nextSeqLocked())
	replies, err := n.transactLocked(msg)
	if err != nil {
		return out, err
	}
	for _, r := range replies {
		attrs, err := parseAttrs(r)
		if err != nil {
			continue
		}
		statsRaw, ok := attrs.bytes(ethAFECStatsAttr)
		if !ok {
			continue
		}
		sattrs, err := parseAttrs(statsRaw)
		if err != nil {
			continue
		}
		// FEC stats may be arrays of u64; first element is the aggregate.
		if v, ok := sattrs.firstUint64(ethAFECStatCorrected); ok {
			out.CorrectedBlocks, out.HasCorrected = v, true
		}
		if v, ok := sattrs.firstUint64(ethAFECStatUncorr); ok {
			out.UncorrectableBlocks, out.HasUncorrectable = v, true
		}
		if v, ok := sattrs.firstUint64(ethAFECStatCorrBits); ok {
			out.CorrectedBits, out.HasCorrectedBits = v, true
		}
	}
	return out, nil
}

// transactLocked requires n.mu held.
func (n *Netlink) transactLocked(req []byte) ([][]byte, error) {
	if _, err := unix.Write(n.fd, req); err != nil {
		return nil, err
	}
	var out [][]byte
	buf := make([]byte, 1<<16)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		nr, err := unix.Read(n.fd, buf)
		if err != nil {
			if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK || err == unix.EAGAIN {
				continue
			}
			// Keep waiting for NLMSG_DONE; do not truncate on timeout mid-dump.
			if os.IsTimeout(err) || err == unix.EINTR {
				continue
			}
			return out, err
		}
		msgs, done, err := parseNetlink(buf[:nr])
		if err != nil {
			return out, err
		}
		out = append(out, msgs...)
		if done {
			return out, nil
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("netlink timeout")
	}
	return out, nil
}

// --- attribute codec ---

type nlBuilder struct {
	buf []byte
}

func (b *nlBuilder) bytes() []byte { return b.buf }

func (b *nlBuilder) put(typ uint16, data []byte) {
	nlaLen := 4 + len(data)
	pad := (nlaAlign - (nlaLen % nlaAlign)) % nlaAlign
	start := len(b.buf)
	b.buf = append(b.buf, make([]byte, 4+len(data)+pad)...)
	binary.LittleEndian.PutUint16(b.buf[start:], uint16(nlaLen))
	binary.LittleEndian.PutUint16(b.buf[start+2:], typ)
	copy(b.buf[start+4:], data)
}

func (b *nlBuilder) putNested(typ uint16, data []byte) {
	b.put(typ|nlaFNested, data)
}

func (b *nlBuilder) putFlag(typ uint16) { b.put(typ, nil) }

func (b *nlBuilder) putUint32(typ uint16, v uint32) {
	var d [4]byte
	binary.LittleEndian.PutUint32(d[:], v)
	b.put(typ, d[:])
}

func (b *nlBuilder) putString(typ uint16, s string) {
	b.put(typ, append([]byte(s), 0))
}

func (b *nlBuilder) putBytes(typ uint16, d []byte) { b.put(typ, d) }

func (b *nlBuilder) finish(family uint16, cmd, version uint8, seq uint32) []byte {
	const nlHdr = 16
	const genlHdr = 4
	payload := b.buf
	total := nlHdr + genlHdr + len(payload)
	out := make([]byte, total)
	binary.LittleEndian.PutUint32(out[0:], uint32(total))
	binary.LittleEndian.PutUint16(out[4:], family)
	binary.LittleEndian.PutUint16(out[6:], unix.NLM_F_REQUEST|unix.NLM_F_ACK)
	binary.LittleEndian.PutUint32(out[8:], seq)
	binary.LittleEndian.PutUint32(out[12:], uint32(os.Getpid()))
	out[16] = cmd
	out[17] = version
	copy(out[20:], payload)
	return out
}

type attr struct {
	typ  uint16
	data []byte
}

type attrMap struct {
	byType map[uint16][]attr
	order  []attr
}

func parseAttrs(b []byte) (attrMap, error) {
	m := attrMap{byType: make(map[uint16][]attr)}
	for len(b) >= 4 {
		a, rest, err := takeAttr(b)
		if err != nil {
			return m, err
		}
		m.byType[a.typ] = append(m.byType[a.typ], a)
		m.order = append(m.order, a)
		b = rest
	}
	return m, nil
}

func parseOneAttr(b []byte) (attr, error) {
	a, _, err := takeAttr(b)
	return a, err
}

func takeAttr(b []byte) (attr, []byte, error) {
	if len(b) < 4 {
		return attr{}, nil, fmt.Errorf("short nla")
	}
	nlaLen := int(binary.LittleEndian.Uint16(b[0:2]))
	typ := binary.LittleEndian.Uint16(b[2:4]) & (^uint16(nlaFNested))
	if nlaLen < 4 || nlaLen > len(b) {
		return attr{}, nil, fmt.Errorf("bad nla_len %d", nlaLen)
	}
	data := b[4:nlaLen]
	aligned := (nlaLen + nlaAlign - 1) & ^(nlaAlign - 1)
	if aligned > len(b) {
		aligned = len(b)
	}
	return attr{typ: typ, data: data}, b[aligned:], nil
}

func (m attrMap) bytes(typ uint16) ([]byte, bool) {
	as := m.byType[typ]
	if len(as) == 0 {
		return nil, false
	}
	return as[0].data, true
}

func (m attrMap) nestedAll(typ uint16) [][]byte {
	as := m.byType[typ]
	out := make([][]byte, 0, len(as))
	for _, a := range as {
		out = append(out, a.data)
	}
	return out
}

func (m attrMap) uint16(typ uint16) (uint16, bool) {
	d, ok := m.bytes(typ)
	if !ok || len(d) < 2 {
		return 0, false
	}
	return binary.LittleEndian.Uint16(d), true
}

func (m attrMap) uint32(typ uint16) (uint32, bool) {
	d, ok := m.bytes(typ)
	if !ok || len(d) < 4 {
		return 0, false
	}
	return binary.LittleEndian.Uint32(d), true
}

func (m attrMap) uint64(typ uint16) (uint64, bool) {
	d, ok := m.bytes(typ)
	if !ok || len(d) < 8 {
		return 0, false
	}
	return binary.LittleEndian.Uint64(d), true
}

func (m attrMap) firstUint64(typ uint16) (uint64, bool) {
	// Some FEC stats are arrays of u64; take the first.
	d, ok := m.bytes(typ)
	if !ok || len(d) < 8 {
		return 0, false
	}
	return binary.LittleEndian.Uint64(d[:8]), true
}

func parseNetlink(b []byte) (payloads [][]byte, done bool, err error) {
	for len(b) >= 16 {
		l := int(binary.LittleEndian.Uint32(b[0:4]))
		if l < 16 || l > len(b) {
			return payloads, done, fmt.Errorf("bad nlmsg_len")
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
			done = true
			return payloads, done, nil
		case unix.NLMSG_ERROR:
			if l < 20 {
				return payloads, true, fmt.Errorf("short nlmsg error")
			}
			code := int32(binary.LittleEndian.Uint32(msg[16:20]))
			if code == 0 {
				done = true
				return payloads, done, nil
			}
			return payloads, true, syscall.Errno(-code)
		default:
			if l < 20 { // nl + genl header
				continue
			}
			payload := make([]byte, l-20)
			copy(payload, msg[20:l])
			payloads = append(payloads, payload)
		}
	}
	return payloads, done, nil
}
