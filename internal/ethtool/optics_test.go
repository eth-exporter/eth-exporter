package ethtool

import "testing"

func TestParseSFF8472DOM(t *testing.T) {
	data := make([]byte, 512)
	copy(data[20:36], []byte("ACME CORP       "))
	copy(data[40:56], []byte("SFP-10G-SR      "))
	copy(data[68:84], []byte("SN1234567890    "))
	// A2 diagnostics
	a2 := data[256:]
	// 25.0 C => 25*256 = 6400 = 0x1900
	a2[96], a2[97] = 0x19, 0x00
	// 3.3V => 33000 * 100uV = 33000 = 0x80E8
	a2[98], a2[99] = 0x80, 0xE8
	// bias 10mA => 10/0.002 = 5000 = 0x1388
	a2[100], a2[101] = 0x13, 0x88
	// tx power 1.0 mW => 10000 * 0.1uW units = 10000
	a2[102], a2[103] = 0x27, 0x10
	a2[104], a2[105] = 0x27, 0x10

	dom := ParseModuleDOM(moduleSFF8472, data)
	if dom.Vendor != "ACME CORP" {
		t.Fatalf("vendor=%q", dom.Vendor)
	}
	if !dom.HasTemperature || dom.TemperatureC < 24.9 || dom.TemperatureC > 25.1 {
		t.Fatalf("temp=%v", dom.TemperatureC)
	}
	if !dom.HasTxPower {
		t.Fatal("expected tx power")
	}
}

func TestBitsToNames(t *testing.T) {
	// bit 5 = 1000baseT/Full, bit 12 = 10000baseT/Full
	bits := []byte{0x20, 0x10} // bit5 in byte0, bit12 = bit4 of byte1
	names := bitsToNames(bits, 16)
	found := map[string]bool{}
	for _, n := range names {
		found[n] = true
	}
	if !found["1000baseT/Full"] || !found["10000baseT/Full"] {
		t.Fatalf("names=%v", names)
	}
}
