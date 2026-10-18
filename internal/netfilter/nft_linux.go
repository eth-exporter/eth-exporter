//go:build linux

package netfilter

import (
	"encoding/binary"
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// NETLINK_NETFILTER + nf_tables subsystem dumps for tables, chains, counters.
const (
	netlinkNetfilter = 12

	nfSubsysNFT = 10

	nftMsgGetTable = 1
	nftMsgGetChain = 4
	nftMsgGetObj   = 19

	nftATableName  = 1
	nftATableFlags = 2
	nftATableUse   = 3

	nftAChainTable = 1
	nftAChainName  = 2

	nftAObjTable = 1
	nftAObjName  = 2
	nftAObjType  = 3
	nftAObjData  = 4

	nftObjectCounter = 1

	nftACounterBytes   = 1
	nftACounterPackets = 2

	nlaFNested = 1 << 15
)

type Client struct {
	mu  sync.Mutex
	fd  int
	seq uint32
}

func New() (*Client, error) {
	fd, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_RAW, netlinkNetfilter)
	if err != nil {
		return nil, err
	}
	if err := unix.Bind(fd, &unix.SockaddrNetlink{Family: unix.AF_NETLINK}); err != nil {
		_ = unix.Close(fd)
		return nil, err
	}
	_ = unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &unix.Timeval{Sec: 2})
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

type Snapshot struct {
	Tables   []Table
	Chains   []Chain
	Counters []Counter
}

type Table struct {
	Family string
	Name   string
	Use    uint32
}

type Chain struct {
	Family string
	Table  string
	Name   string
}

type Counter struct {
	Family  string
	Table   string
	Name    string
	Bytes   uint64
	Packets uint64
}

func (c *Client) Dump() (Snapshot, error) {
	var snap Snapshot
	// Dump across common nfproto families.
	for _, fam := range []uint8{2 /*IPv4*/, 10 /*IPv6*/, 1 /*INET*/, 7 /*BRIDGE*/, 3 /*ARP*/, 5 /*NETDEV*/} {
		tables, err := c.dumpTables(fam)
		if err == nil {
			snap.Tables = append(snap.Tables, tables...)
		}
		chains, err := c.dumpChains(fam)
		if err == nil {
			snap.Chains = append(snap.Chains, chains...)
		}
		counters, err := c.dumpCounters(fam)
		if err == nil {
			snap.Counters = append(snap.Counters, counters...)
		}
	}
	return snap, nil
}

func (c *Client) dumpTables(family uint8) ([]Table, error) {
	msgs, err := c.dump(nftMsgGetTable, family, nil)
	if err != nil {
		return nil, err
	}
	var out []Table
	for _, m := range msgs {
		if len(m) < 4 {
			continue
		}
		fam := familyName(m[0])
		attrs := parseAttrs(m[4:])
		name, _ := attrs.string(nftATableName)
		use, _ := attrs.uint32(nftATableUse)
		if name == "" {
			continue
		}
		out = append(out, Table{Family: fam, Name: name, Use: use})
	}
	return out, nil
}

func (c *Client) dumpChains(family uint8) ([]Chain, error) {
	msgs, err := c.dump(nftMsgGetChain, family, nil)
	if err != nil {
		return nil, err
	}
	var out []Chain
	for _, m := range msgs {
		if len(m) < 4 {
			continue
		}
		fam := familyName(m[0])
		attrs := parseAttrs(m[4:])
		table, _ := attrs.string(nftAChainTable)
		name, _ := attrs.string(nftAChainName)
		if name == "" {
			continue
		}
		out = append(out, Chain{Family: fam, Table: table, Name: name})
	}
	return out, nil
}

func (c *Client) dumpCounters(family uint8) ([]Counter, error) {
	// Request objects of type counter.
	var body nlBuilder
	body.putUint32(nftAObjType, nftObjectCounter)
	msgs, err := c.dump(nftMsgGetObj, family, body.bytes())
	if err != nil {
		return nil, err
	}
	var out []Counter
	for _, m := range msgs {
		if len(m) < 4 {
			continue
		}
		fam := familyName(m[0])
		attrs := parseAttrs(m[4:])
		table, _ := attrs.string(nftAObjTable)
		name, _ := attrs.string(nftAObjName)
		data, ok := attrs.bytes(nftAObjData)
		if !ok || name == "" {
			continue
		}
		dattrs := parseAttrs(data)
		bytes, _ := dattrs.uint64(nftACounterBytes)
		pkts, _ := dattrs.uint64(nftACounterPackets)
		out = append(out, Counter{Family: fam, Table: table, Name: name, Bytes: bytes, Packets: pkts})
	}
	return out, nil
}

func (c *Client) dump(msgType uint8, family uint8, attrs []byte) ([][]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.seq++
	if c.seq == 0 {
		c.seq = 1
	}
	// nlmsghdr + nfgenmsg(4) + attrs
	const nlHdr = 16
	const nfHdr = 4
	nlType := uint16(nfSubsysNFT<<8) | uint16(msgType)
	total := nlHdr + nfHdr + len(attrs)
	req := make([]byte, total)
	binary.LittleEndian.PutUint32(req[0:], uint32(total))
	binary.LittleEndian.PutUint16(req[4:], nlType)
	binary.LittleEndian.PutUint16(req[6:], unix.NLM_F_REQUEST|unix.NLM_F_DUMP)
	binary.LittleEndian.PutUint32(req[8:], c.seq)
	binary.LittleEndian.PutUint32(req[12:], uint32(os.Getpid()))
	req[16] = family
	req[17] = 0 // NFT_API_VERSION-ish
	binary.LittleEndian.PutUint16(req[18:], 0)
	copy(req[20:], attrs)

	if _, err := unix.Write(c.fd, req); err != nil {
		return nil, err
	}
	var out [][]byte
	buf := make([]byte, 1<<16)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		nr, err := unix.Read(c.fd, buf)
		if err != nil {
			if err == syscall.EAGAIN || err == unix.EAGAIN || err == unix.EINTR || os.IsTimeout(err) {
				continue
			}
			return out, err
		}
		msgs, done, err := parseNL(buf[:nr])
		if err != nil {
			if len(out) == 0 {
				return nil, err
			}
			return out, nil
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
			if l > 16 {
				payload := make([]byte, l-16)
				copy(payload, msg[16:l])
				payloads = append(payloads, payload)
			}
		}
	}
	return payloads, done, nil
}

type nlBuilder struct{ buf []byte }

func (b *nlBuilder) bytes() []byte { return b.buf }
func (b *nlBuilder) put(typ uint16, data []byte) {
	nlaLen := 4 + len(data)
	pad := (4 - (nlaLen % 4)) % 4
	start := len(b.buf)
	b.buf = append(b.buf, make([]byte, 4+len(data)+pad)...)
	binary.LittleEndian.PutUint16(b.buf[start:], uint16(nlaLen))
	binary.LittleEndian.PutUint16(b.buf[start+2:], typ)
	copy(b.buf[start+4:], data)
}
func (b *nlBuilder) putUint32(typ uint16, v uint32) {
	var d [4]byte
	binary.LittleEndian.PutUint32(d[:], v)
	b.put(typ, d[:])
}

type attrMap struct {
	byType map[uint16][]byte
}

func parseAttrs(b []byte) attrMap {
	m := attrMap{byType: make(map[uint16][]byte)}
	for len(b) >= 4 {
		l := int(binary.LittleEndian.Uint16(b[0:2]))
		t := binary.LittleEndian.Uint16(b[2:4]) & (^uint16(nlaFNested))
		if l < 4 || l > len(b) {
			break
		}
		m.byType[t] = b[4:l]
		aligned := (l + 3) & ^3
		if aligned > len(b) {
			break
		}
		b = b[aligned:]
	}
	return m
}

func (m attrMap) bytes(t uint16) ([]byte, bool) {
	d, ok := m.byType[t]
	return d, ok
}
func (m attrMap) string(t uint16) (string, bool) {
	d, ok := m.bytes(t)
	if !ok {
		return "", false
	}
	for i, c := range d {
		if c == 0 {
			return string(d[:i]), true
		}
	}
	return string(d), true
}
func (m attrMap) uint32(t uint16) (uint32, bool) {
	d, ok := m.bytes(t)
	if !ok || len(d) < 4 {
		return 0, false
	}
	return binary.LittleEndian.Uint32(d), true
}
func (m attrMap) uint64(t uint16) (uint64, bool) {
	d, ok := m.bytes(t)
	if !ok || len(d) < 8 {
		return 0, false
	}
	return binary.LittleEndian.Uint64(d), true
}

func familyName(f uint8) string {
	switch f {
	case 1:
		return "inet"
	case 2:
		return "ip"
	case 3:
		return "arp"
	case 5:
		return "netdev"
	case 7:
		return "bridge"
	case 10:
		return "ip6"
	default:
		return fmt.Sprintf("family_%d", f)
	}
}
