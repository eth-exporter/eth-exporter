package neighbor

import (
	"encoding/binary"
	"testing"
)

func TestParseLLDP(t *testing.T) {
	// dst(6) + src(6) + ethertype(2) + TLVs
	frame := make([]byte, 0, 128)
	frame = append(frame, []byte{0x01, 0x80, 0xc2, 0x00, 0x00, 0x0e}...)
	frame = append(frame, []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}...)
	frame = append(frame, []byte{0x88, 0xcc}...)

	// Chassis ID TLV: type=1, len=7, subtype=4 (MAC), mac
	chassis := []byte{0x04, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66}
	frame = append(frame, lldpTLV(1, chassis)...)
	// Port ID TLV: type=2, subtype=5 / ifName
	port := append([]byte{0x05}, []byte("Gi0/1")...)
	frame = append(frame, lldpTLV(2, port)...)
	// TTL = 120
	ttl := make([]byte, 2)
	binary.BigEndian.PutUint16(ttl, 120)
	frame = append(frame, lldpTLV(3, ttl)...)
	// System name
	frame = append(frame, lldpTLV(5, []byte("core-sw1"))...)
	// End
	frame = append(frame, lldpTLV(0, nil)...)

	n, ok := ParseLLDP("eth0", frame)
	if !ok {
		t.Fatal("expected LLDP parse success")
	}
	if n.ChassisID != "11:22:33:44:55:66" {
		t.Fatalf("chassis=%q", n.ChassisID)
	}
	if n.PortID != "Gi0/1" {
		t.Fatalf("port=%q", n.PortID)
	}
	if n.SystemName != "core-sw1" {
		t.Fatalf("sys=%q", n.SystemName)
	}
	if n.TTL.Seconds() != 120 {
		t.Fatalf("ttl=%v", n.TTL)
	}
}

func lldpTLV(typ int, val []byte) []byte {
	hdr := uint16(typ<<9) | uint16(len(val))
	out := make([]byte, 2+len(val))
	binary.BigEndian.PutUint16(out[0:2], hdr)
	copy(out[2:], val)
	return out
}
