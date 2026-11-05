package ethtool

// CMIS (Common Management Interface Specification, OIF CMIS 4.0/5.x) decode for
// QSFP-DD, OSFP, QSFP112 and other modules that report a CMIS SFF-8024
// identifier. Drivers commonly report a legacy ETHTOOL_GMODULEINFO type for
// these modules (e.g. ionic → SFF-8472, bnxt_en → SFF-8636), so the CMIS
// identifier in byte 0 must take precedence over the driver-reported type.
//
// Memory map used (all offsets absolute within a 256-byte page view):
//   Lower page 00h: 0 identifier, 1 revision, 2 flags (bit7 = flat memory),
//                   14-15 module temperature (s16, 1/256 °C),
//                   16-17 supply voltage (u16, 100 µV),
//                   85 media type, 86-89 first application descriptor
//                   (88 low nibble = media lane count).
//   Upper page 00h: 129-144 vendor name, 148-163 part number, 166-181 serial.
//   Page 01h:       160 bits 4:3 Tx bias current multiplier (1x/2x/4x).
//   Page 11h:       154-169 Tx optical power lanes 1-8 (u16, 0.1 µW),
//                   170-185 Tx bias lanes 1-8 (u16, 2 µA × multiplier),
//                   186-201 Rx optical power lanes 1-8 (u16, 0.1 µW).

// SFF-8024 identifiers whose management interface is CMIS.
var cmisIdentifiers = map[byte]bool{
	0x18: true, // QSFP-DD
	0x19: true, // OSFP
	0x1b: true, // DSFP
	0x1e: true, // QSFP+ or later with CMIS (QSFP112 etc.)
	0x1f: true, // SFP-DD with CMIS
	0x20: true, // SFP+ and later with CMIS
}

// IsCMIS reports whether an EEPROM blob (starting at lower page 00h byte 0)
// belongs to a CMIS-managed module.
func IsCMIS(data []byte) bool {
	return len(data) > 0 && cmisIdentifiers[data[0]]
}

// parseCMIS decodes identity, module temperature/voltage and the media lane
// count from lower page 00h + upper page 00h (the first 256 bytes, which every
// driver returns for ETHTOOL_GMODULEEEPROM / page 0).
func parseCMIS(dom *ModuleDOM, data []byte) {
	dom.CMIS = true
	if len(data) < 128 {
		return
	}
	temp := int16(uint16(data[14])<<8 | uint16(data[15]))
	dom.TemperatureC = float64(temp) / 256.0
	dom.HasTemperature = true

	vcc := uint16(data[16])<<8 | uint16(data[17])
	dom.VoltageV = float64(vcc) * 100e-6
	dom.HasVoltage = vcc != 0

	dom.FlatMemory = data[2]&0x80 != 0
	dom.MediaLanes = int(data[88] & 0x0f)
	if dom.MediaLanes == 0 || dom.MediaLanes > 8 {
		dom.MediaLanes = 8
	}
	if len(data) >= 256 {
		dom.Vendor = cstrBytes(data[129:145])
		dom.PartNumber = cstrBytes(data[148:164])
		dom.Serial = cstrBytes(data[166:182])
	}
}

// ApplyCMISLaneMonitors decodes per-lane Tx/Rx optical power and Tx bias from
// the upper halves (bytes 128-255) of pages 01h and 11h. page01 may be nil
// (bias multiplier then defaults to 1x). Lanes beyond dom.MediaLanes are
// ignored. The module-level Tx/Rx/Bias fields are set from lane 1, matching
// the SFF-8636 decoder.
func ApplyCMISLaneMonitors(dom *ModuleDOM, page01Upper, page11Upper []byte) {
	if len(page11Upper) < 128 {
		return
	}
	mult := 1.0
	if len(page01Upper) >= 128 {
		switch (page01Upper[160-128] >> 3) & 0x3 {
		case 1:
			mult = 2
		case 2:
			mult = 4
		}
	}
	lanes := dom.MediaLanes
	if lanes <= 0 || lanes > 8 {
		lanes = 8
	}
	u16 := func(abs int) uint16 {
		i := abs - 128
		return uint16(page11Upper[i])<<8 | uint16(page11Upper[i+1])
	}
	dom.Channels = dom.Channels[:0]
	for ln := 0; ln < lanes; ln++ {
		tx := u16(154 + 2*ln)
		bias := u16(170 + 2*ln)
		rx := u16(186 + 2*ln)
		c := ChannelDOM{
			Lane:       ln + 1,
			TxPowerdBm: cmisDBm(tx),
			BiasmA:     float64(bias) * 2e-3 * mult,
			RxPowerdBm: cmisDBm(rx),
			TxPowermW:  float64(tx) * 0.1e-3,
			RxPowermW:  float64(rx) * 0.1e-3,
		}
		dom.Channels = append(dom.Channels, c)
		if ln == 0 {
			dom.TxPowerdBm, dom.HasTxPower = c.TxPowerdBm, tx != 0
			dom.RxPowerdBm, dom.HasRxPower = c.RxPowerdBm, rx != 0
			dom.BiasmA, dom.HasBias = c.BiasmA, bias != 0
		}
	}
}

// cmisDBm converts a CMIS power register (0.1 µW units) to dBm. A zero
// register (no light / loss of signal) is reported as -40 dBm, the smallest
// representable non-zero reading, instead of mwToDBm's 0 dBm (= 1 mW), which
// would make a dark lane look healthy on dashboards and alerts.
func cmisDBm(raw uint16) float64 {
	if raw == 0 {
		return -40
	}
	return mwToDBm(float64(raw) * 0.1e-3)
}
