package neighbor

import (
	"encoding/binary"
	"testing"
)

func TestParseCDP(t *testing.T) {
	frame := make([]byte, 0, 256)
	frame = append(frame, []byte{0x01, 0x00, 0x0c, 0xcc, 0xcc, 0xcc}...)
	frame = append(frame, []byte{0xde, 0xad, 0xbe, 0xef, 0x00, 0x01}...)
	// 802.3 length placeholder; set after payload built
	frame = append(frame, []byte{0x00, 0x00}...)

	payload := []byte{
		0xaa, 0xaa, 0x03, // LLC
		0x00, 0x00, 0x0c, // Cisco OUI
		0x20, 0x00, // CDP
		0x02, 180, 0x00, 0x00, // ver, ttl, checksum
	}
	payload = append(payload, cdpTLV(0x0001, []byte("SWITCH1"))...)
	payload = append(payload, cdpTLV(0x0003, []byte("Ethernet1/1"))...)
	payload = append(payload, cdpTLV(0x0006, []byte("cisco WS-C2960"))...)

	binary.BigEndian.PutUint16(frame[12:14], uint16(len(payload)))
	frame = append(frame, payload...)

	n, ok := ParseCDP("eth0", frame)
	if !ok {
		t.Fatal("expected CDP parse success")
	}
	if n.ChassisID != "SWITCH1" {
		t.Fatalf("device=%q", n.ChassisID)
	}
	if n.PortID != "Ethernet1/1" {
		t.Fatalf("port=%q", n.PortID)
	}
	if n.Platform != "cisco WS-C2960" {
		t.Fatalf("platform=%q", n.Platform)
	}
	if n.TTL.Seconds() != 180 {
		t.Fatalf("ttl=%v", n.TTL)
	}
}

func cdpTLV(typ uint16, val []byte) []byte {
	out := make([]byte, 4+len(val))
	binary.BigEndian.PutUint16(out[0:2], typ)
	binary.BigEndian.PutUint16(out[2:4], uint16(4+len(val)))
	copy(out[4:], val)
	return out
}
