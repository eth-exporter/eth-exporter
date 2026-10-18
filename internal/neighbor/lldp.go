package neighbor

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"time"
)

const (
	etherTypeLLDP = 0x88cc

	lldpTLVEnd            = 0
	lldpTLVChassisID      = 1
	lldpTLVPortID         = 2
	lldpTLVTTL            = 3
	lldpTLVPortDesc       = 4
	lldpTLVSystemName     = 5
	lldpTLVSystemDesc     = 6
	lldpTLVSystemCaps     = 7
	lldpTLVManagementAddr = 8
)

// ParseLLDP parses an Ethernet frame that may contain LLDP.
func ParseLLDP(iface string, frame []byte) (Neighbor, bool) {
	if len(frame) < 14 {
		return Neighbor{}, false
	}
	dst := net.HardwareAddr(frame[0:6])
	src := net.HardwareAddr(frame[6:12])
	etype := binary.BigEndian.Uint16(frame[12:14])
	payload := frame[14:]

	// Skip 802.1Q tag.
	if etype == 0x8100 && len(frame) >= 18 {
		etype = binary.BigEndian.Uint16(frame[16:18])
		payload = frame[18:]
	}
	if etype != etherTypeLLDP || !isLLDPDest(dst) {
		return Neighbor{}, false
	}

	n := Neighbor{
		Interface:    iface,
		Protocol:     ProtoLLDP,
		RawSourceMAC: src.String(),
		LastSeen:     time.Now(),
	}

	for len(payload) >= 2 {
		hdr := binary.BigEndian.Uint16(payload[0:2])
		typ := int(hdr >> 9)
		length := int(hdr & 0x1ff)
		payload = payload[2:]
		if length > len(payload) {
			break
		}
		val := payload[:length]
		payload = payload[length:]

		switch typ {
		case lldpTLVEnd:
			return n, n.ChassisID != "" || n.PortID != ""
		case lldpTLVChassisID:
			n.ChassisID = parseLLDPID(val)
		case lldpTLVPortID:
			n.PortID = parseLLDPID(val)
		case lldpTLVTTL:
			if length >= 2 {
				n.TTL = time.Duration(binary.BigEndian.Uint16(val[0:2])) * time.Second
			}
		case lldpTLVPortDesc:
			n.PortDesc = string(val)
		case lldpTLVSystemName:
			n.SystemName = string(val)
		case lldpTLVSystemDesc:
			n.SystemDesc = string(val)
		case lldpTLVSystemCaps:
			if length >= 4 {
				n.Capabilities = fmt.Sprintf("0x%04x", binary.BigEndian.Uint16(val[0:2]))
			}
		case lldpTLVManagementAddr:
			n.ManagementAddr = parseLLDPMgmt(val)
		}
	}
	return n, n.ChassisID != "" || n.PortID != ""
}

func isLLDPDest(dst net.HardwareAddr) bool {
	if len(dst) != 6 {
		return false
	}
	return dst[0] == 0x01 && dst[1] == 0x80 && dst[2] == 0xc2 &&
		dst[3] == 0x00 && dst[4] == 0x00 &&
		(dst[5] == 0x00 || dst[5] == 0x03 || dst[5] == 0x0e)
}

func parseLLDPID(val []byte) string {
	if len(val) < 1 {
		return ""
	}
	subtype := val[0]
	data := val[1:]
	switch subtype {
	case 4: // MAC address
		if len(data) == 6 {
			return net.HardwareAddr(data).String()
		}
	case 5: // network address
		if len(data) >= 2 {
			family := data[0]
			addr := data[1:]
			if family == 1 && len(addr) >= 4 { // IPv4
				return net.IP(addr[:4]).String()
			}
			if family == 2 && len(addr) >= 16 { // IPv6
				return net.IP(addr[:16]).String()
			}
		}
	}
	return strings.TrimSpace(string(data))
}

func parseLLDPMgmt(val []byte) string {
	if len(val) < 2 {
		return ""
	}
	addrLen := int(val[0])
	if addrLen < 1 || len(val) < 1+addrLen {
		return ""
	}
	family := val[1]
	addr := val[2 : 1+addrLen]
	switch family {
	case 1:
		if len(addr) >= 4 {
			return net.IP(addr[:4]).String()
		}
	case 2:
		if len(addr) >= 16 {
			return net.IP(addr[:16]).String()
		}
	}
	return fmt.Sprintf("%d:%x", family, addr)
}
