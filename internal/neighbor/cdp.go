package neighbor

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"time"
)

const (
	snapDSAP         = 0xaa
	snapSSAP         = 0xaa
	snapControl      = 0x03
	cdpEtherType     = 0x2000
	cdpTLVDeviceID   = 0x0001
	cdpTLVAddress    = 0x0002
	cdpTLVPortID     = 0x0003
	cdpTLVCaps       = 0x0004
	cdpTLVVersion    = 0x0005
	cdpTLVPlatform   = 0x0006
	cdpTLVNativeVLAN = 0x000a
)

var cdpDest = net.HardwareAddr{0x01, 0x00, 0x0c, 0xcc, 0xcc, 0xcc}
var ciscoOUI = []byte{0x00, 0x00, 0x0c}

// ParseCDP parses an Ethernet frame that may contain CDP over LLC/SNAP.
func ParseCDP(iface string, frame []byte) (Neighbor, bool) {
	if len(frame) < 14+8+4 {
		return Neighbor{}, false
	}
	dst := net.HardwareAddr(frame[0:6])
	src := net.HardwareAddr(frame[6:12])
	etype := binary.BigEndian.Uint16(frame[12:14])
	payload := frame[14:]

	if etype == 0x8100 && len(frame) >= 18 {
		etype = binary.BigEndian.Uint16(frame[16:18])
		payload = frame[18:]
	}

	// CDP uses IEEE 802.3 length field (<= 1500), not EtherType.
	if etype > 1500 {
		return Neighbor{}, false
	}
	if !bytes.Equal(dst, cdpDest) {
		return Neighbor{}, false
	}
	if len(payload) < 8 {
		return Neighbor{}, false
	}
	// LLC + SNAP
	if payload[0] != snapDSAP || payload[1] != snapSSAP || payload[2] != snapControl {
		return Neighbor{}, false
	}
	if payload[3] != ciscoOUI[0] || payload[4] != ciscoOUI[1] || payload[5] != ciscoOUI[2] {
		return Neighbor{}, false
	}
	proto := binary.BigEndian.Uint16(payload[6:8])
	if proto != cdpEtherType {
		return Neighbor{}, false
	}
	cdp := payload[8:]
	if len(cdp) < 4 {
		return Neighbor{}, false
	}
	// version, ttl, checksum
	ttl := time.Duration(cdp[1]) * time.Second
	tlvs := cdp[4:]

	n := Neighbor{
		Interface:    iface,
		Protocol:     ProtoCDP,
		RawSourceMAC: src.String(),
		TTL:          ttl,
		LastSeen:     time.Now(),
	}

	for len(tlvs) >= 4 {
		typ := binary.BigEndian.Uint16(tlvs[0:2])
		length := int(binary.BigEndian.Uint16(tlvs[2:4]))
		if length < 4 || length > len(tlvs) {
			break
		}
		val := tlvs[4:length]
		tlvs = tlvs[length:]
		switch typ {
		case cdpTLVDeviceID:
			n.ChassisID = string(val)
			n.SystemName = string(val)
		case cdpTLVPortID:
			n.PortID = string(val)
		case cdpTLVCaps:
			if len(val) >= 4 {
				n.Capabilities = fmt.Sprintf("0x%08x", binary.BigEndian.Uint32(val[0:4]))
			}
		case cdpTLVVersion:
			n.Version = strings.TrimSpace(string(val))
			n.SystemDesc = n.Version
		case cdpTLVPlatform:
			n.Platform = strings.TrimSpace(string(val))
		case cdpTLVNativeVLAN:
			if len(val) >= 2 {
				n.VLAN = fmt.Sprintf("%d", binary.BigEndian.Uint16(val[0:2]))
			}
		case cdpTLVAddress:
			n.ManagementAddr = parseCDPAddresses(val)
		}
	}
	return n, n.ChassisID != "" || n.PortID != ""
}

func parseCDPAddresses(v []byte) string {
	if len(v) < 4 {
		return ""
	}
	num := int(binary.BigEndian.Uint32(v[0:4]))
	v = v[4:]
	var addrs []string
	for i := 0; i < num && len(v) >= 5; i++ {
		protoType := v[0]
		protoLen := int(v[1])
		if 2+protoLen+2 > len(v) {
			break
		}
		addrLen := int(binary.BigEndian.Uint16(v[2+protoLen : 2+protoLen+2]))
		start := 2 + protoLen + 2
		if start+addrLen > len(v) {
			break
		}
		addr := v[start : start+addrLen]
		// NLPID 0xcc = IPv4
		if protoType == 1 && protoLen == 1 && v[2] == 0xcc && addrLen == 4 {
			addrs = append(addrs, net.IP(addr).String())
		} else if addrLen == 16 {
			addrs = append(addrs, net.IP(addr).String())
		}
		v = v[start+addrLen:]
	}
	return strings.Join(addrs, ",")
}
