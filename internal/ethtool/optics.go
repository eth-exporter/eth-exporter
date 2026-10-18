package ethtool

import (
	"bytes"
	"math"
)

const (
	moduleSFF8079 = 0x1
	moduleSFF8472 = 0x2
	moduleSFF8636 = 0x3
	moduleSFF8436 = 0x4
)

// ParseModuleDOM extracts identity + DOM from an ethtool module EEPROM blob.
func ParseModuleDOM(modType uint32, data []byte) ModuleDOM {
	dom := ModuleDOM{Type: modType}
	switch modType {
	case moduleSFF8472:
		parseSFF8472(&dom, data)
	case moduleSFF8636, moduleSFF8436:
		parseSFF8636(&dom, data)
	case moduleSFF8079:
		parseSFF8079ID(&dom, data)
	default:
		if len(data) >= 256 {
			parseSFF8079ID(&dom, data)
		}
		if len(data) >= 512 {
			parseSFF8472DOM(&dom, data[256:])
		}
	}
	return dom
}

func parseSFF8079ID(dom *ModuleDOM, data []byte) {
	if len(data) < 96 {
		return
	}
	dom.Vendor = cstrBytes(data[20:36])
	dom.PartNumber = cstrBytes(data[40:56])
	dom.Serial = cstrBytes(data[68:84])
}

func parseSFF8472(dom *ModuleDOM, data []byte) {
	parseSFF8079ID(dom, data)
	if len(data) >= 512 {
		parseSFF8472DOM(dom, data[256:])
	}
}

func parseSFF8472DOM(dom *ModuleDOM, a2 []byte) {
	if len(a2) < 106 {
		return
	}
	// Diagnostic monitoring type at A0 byte 92; if only A2 present assume DOM.
	temp := int16(a2[96])<<8 | int16(a2[97])
	dom.TemperatureC = float64(temp) / 256.0
	dom.HasTemperature = true

	vcc := uint16(a2[98])<<8 | uint16(a2[99])
	dom.VoltageV = float64(vcc) * 100e-6
	dom.HasVoltage = true

	bias := uint16(a2[100])<<8 | uint16(a2[101])
	dom.BiasmA = float64(bias) * 2e-3
	dom.HasBias = true

	tx := uint16(a2[102])<<8 | uint16(a2[103])
	dom.TxPowerdBm = mwToDBm(float64(tx) * 0.1e-3)
	dom.HasTxPower = tx != 0

	rx := uint16(a2[104])<<8 | uint16(a2[105])
	dom.RxPowerdBm = mwToDBm(float64(rx) * 0.1e-3)
	dom.HasRxPower = rx != 0
}

func parseSFF8636(dom *ModuleDOM, data []byte) {
	if len(data) < 224 {
		return
	}
	dom.Vendor = cstrBytes(data[148:164])
	dom.PartNumber = cstrBytes(data[168:184])
	dom.Serial = cstrBytes(data[196:212])

	temp := int16(data[22])<<8 | int16(data[23])
	dom.TemperatureC = float64(temp) / 256.0
	dom.HasTemperature = true

	vcc := uint16(data[26])<<8 | uint16(data[27])
	dom.VoltageV = float64(vcc) * 100e-6
	dom.HasVoltage = true

	// Per-channel RX power 34-41, TX bias 42-49, TX power 50-57 (2 bytes each × 4)
	for ch := 0; ch < 4; ch++ {
		c := ChannelDOM{Lane: ch + 1}
		rxOff := 34 + ch*2
		biasOff := 42 + ch*2
		txOff := 50 + ch*2
		if txOff+1 >= len(data) {
			break
		}
		rx := uint16(data[rxOff])<<8 | uint16(data[rxOff+1])
		bias := uint16(data[biasOff])<<8 | uint16(data[biasOff+1])
		tx := uint16(data[txOff])<<8 | uint16(data[txOff+1])
		c.RxPowerdBm = mwToDBm(float64(rx) * 0.1e-3)
		c.BiasmA = float64(bias) * 2e-3
		c.TxPowerdBm = mwToDBm(float64(tx) * 0.1e-3)
		dom.Channels = append(dom.Channels, c)
		if ch == 0 {
			dom.RxPowerdBm, dom.HasRxPower = c.RxPowerdBm, rx != 0
			dom.BiasmA, dom.HasBias = c.BiasmA, bias != 0
			dom.TxPowerdBm, dom.HasTxPower = c.TxPowerdBm, tx != 0
		}
	}
}

func mwToDBm(mw float64) float64 {
	if mw <= 0 {
		// Callers gate on Has* flags; avoid -Inf in metrics.
		return 0
	}
	return 10 * math.Log10(mw)
}

func cstrBytes(b []byte) string {
	b = bytes.TrimSpace(b)
	if i := bytes.IndexByte(b, 0); i >= 0 {
		b = b[:i]
	}
	return string(bytes.TrimSpace(b))
}
