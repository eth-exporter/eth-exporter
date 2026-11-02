package ethtool

import (
	"encoding/hex"
	"math"
	"testing"
)

func fixture(t *testing.T, dev, part string) []byte {
	t.Helper()
	b, err := hex.DecodeString(cmisFixtures[dev][part])
	if err != nil {
		t.Fatalf("fixture %s/%s: %v", dev, part, err)
	}
	return b
}

func near(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

// Regression: CMIS modules were decoded with SFF-8472/SFF-8636 offsets
// (whatever type the driver reported), producing temperature 0, bogus vendor
// strings and no lane data on QSFP112 / QSFP-DD modules.
func TestParseCMISIdentityAndModuleMonitors(t *testing.T) {
	cases := []struct {
		dev      string
		drvType  uint32
		vendor   string
		pn       string
		sn       string
		tempC    float64
		vcc      float64
		lanes    int
		txLane1  float64 // mW
		rxLane1  float64 // mW
		biasLane float64 // mA
	}{
		{"tw-eth0", moduleSFF8472, "TensorWave", "MOPEC7512017Z", "L12617000351", 52.875, 3.2943, 4, 1.4166, 0.0001, 59.032},
		{"eno0", moduleSFF8636, "Edgecore", "ET7512-DR4", "L12529001419", 51.234, 3.3036, 4, 1.68, 1.5387, 59.984},
	}
	for _, c := range cases {
		lower := fixture(t, c.dev, "lower")
		dom := ParseModuleDOM(c.drvType, lower)
		if !dom.CMIS {
			t.Fatalf("%s: not detected as CMIS", c.dev)
		}
		if dom.Vendor != c.vendor || dom.PartNumber != c.pn || dom.Serial != c.sn {
			t.Fatalf("%s: identity %q/%q/%q", c.dev, dom.Vendor, dom.PartNumber, dom.Serial)
		}
		if !dom.HasTemperature || !near(dom.TemperatureC, c.tempC, 0.01) {
			t.Fatalf("%s: temp=%v", c.dev, dom.TemperatureC)
		}
		if !dom.HasVoltage || !near(dom.VoltageV, c.vcc, 0.0002) {
			t.Fatalf("%s: vcc=%v", c.dev, dom.VoltageV)
		}
		if dom.MediaLanes != c.lanes || dom.FlatMemory {
			t.Fatalf("%s: lanes=%d flat=%v", c.dev, dom.MediaLanes, dom.FlatMemory)
		}
		if len(dom.Channels) != 0 {
			t.Fatalf("%s: lower page alone must not invent lane data", c.dev)
		}

		ApplyCMISLaneMonitors(&dom, fixture(t, c.dev, "p01"), fixture(t, c.dev, "p11"))
		if len(dom.Channels) != c.lanes {
			t.Fatalf("%s: channels=%d", c.dev, len(dom.Channels))
		}
		l1 := dom.Channels[0]
		if !near(l1.TxPowermW, c.txLane1, 1e-4) || !near(l1.RxPowermW, c.rxLane1, 1e-4) || !near(l1.BiasmA, c.biasLane, 1e-3) {
			t.Fatalf("%s: lane1 tx=%v rx=%v bias=%v", c.dev, l1.TxPowermW, l1.RxPowermW, l1.BiasmA)
		}
		if !near(l1.TxPowerdBm, 10*math.Log10(c.txLane1), 0.01) {
			t.Fatalf("%s: lane1 tx dBm=%v", c.dev, l1.TxPowerdBm)
		}
		if !dom.HasTxPower || !dom.HasBias {
			t.Fatalf("%s: module-level tx/bias flags not set", c.dev)
		}
	}
}

func TestCMISDarkLaneIsNotZeroDBm(t *testing.T) {
	p11 := make([]byte, 128) // all-zero power registers = no light
	dom := ModuleDOM{MediaLanes: 2}
	ApplyCMISLaneMonitors(&dom, nil, p11)
	for _, ch := range dom.Channels {
		if ch.RxPowerdBm != -40 || ch.TxPowerdBm != -40 || ch.RxPowermW != 0 {
			t.Fatalf("dark lane %d: rx=%v dBm tx=%v dBm rx=%v mW", ch.Lane, ch.RxPowerdBm, ch.TxPowerdBm, ch.RxPowermW)
		}
	}
	if dom.HasRxPower {
		t.Fatal("HasRxPower must be false for a dark lane 1")
	}
}

func TestCMISBiasMultiplier(t *testing.T) {
	p01 := make([]byte, 128)
	p11 := make([]byte, 128)
	p11[170-128], p11[171-128] = 0x13, 0x88 // 5000 * 2µA = 10 mA at 1x
	dom := ModuleDOM{MediaLanes: 1}
	for bits, want := range map[byte]float64{0: 10, 1: 20, 2: 40} {
		p01[160-128] = bits << 3
		ApplyCMISLaneMonitors(&dom, p01, p11)
		if !near(dom.Channels[0].BiasmA, want, 1e-9) {
			t.Fatalf("multiplier bits %d: bias=%v want %v", bits, dom.Channels[0].BiasmA, want)
		}
	}
}

func TestNonCMISUnchanged(t *testing.T) {
	data := make([]byte, 512)
	data[0] = 0x03 // SFP
	if ParseModuleDOM(moduleSFF8472, data).CMIS {
		t.Fatal("SFP misdetected as CMIS")
	}
	q := make([]byte, 256)
	q[0] = 0x11 // QSFP28 (SFF-8636)
	if ParseModuleDOM(moduleSFF8636, q).CMIS {
		t.Fatal("QSFP28 misdetected as CMIS")
	}
}
