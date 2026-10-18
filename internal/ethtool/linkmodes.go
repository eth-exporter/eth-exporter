package ethtool

// Common ETHTOOL_LINK_MODE_*_BIT names (subset of widely used bits).
var linkModeNames = map[uint]string{
	0:  "10baseT/Half",
	1:  "10baseT/Full",
	2:  "100baseT/Half",
	3:  "100baseT/Full",
	4:  "1000baseT/Half",
	5:  "1000baseT/Full",
	6:  "Autoneg",
	7:  "TP",
	8:  "AUI",
	9:  "MII",
	10: "FIBRE",
	11: "BNC",
	12: "10000baseT/Full",
	13: "Pause",
	14: "Asym_Pause",
	15: "2500baseX/Full",
	16: "Backplane",
	17: "1000baseKX/Full",
	18: "10000baseKX4/Full",
	19: "10000baseKR/Full",
	20: "10000baseR_FEC",
	21: "20000baseMLD2/Full",
	22: "20000baseKR2/Full",
	23: "40000baseKR4/Full",
	24: "40000baseCR4/Full",
	25: "40000baseSR4/Full",
	26: "40000baseLR4/Full",
	27: "56000baseKR4/Full",
	28: "56000baseCR4/Full",
	29: "56000baseSR4/Full",
	30: "56000baseLR4/Full",
	31: "25000baseCR/Full",
	32: "25000baseKR/Full",
	33: "25000baseSR/Full",
	34: "50000baseCR2/Full",
	35: "50000baseKR2/Full",
	36: "100000baseKR4/Full",
	37: "100000baseSR4/Full",
	38: "100000baseCR4/Full",
	39: "100000baseLR4_ER4/Full",
	40: "50000baseSR2/Full",
	41: "1000baseX/Full",
	42: "10000baseCR/Full",
	43: "10000baseSR/Full",
	44: "10000baseLR/Full",
	45: "10000baseLRM/Full",
	46: "10000baseER/Full",
	47: "2500baseT/Full",
	48: "5000baseT/Full",
	49: "FEC_NONE",
	50: "FEC_RS",
	51: "FEC_BASER",
	52: "50000baseKR/Full",
	53: "50000baseSR/Full",
	54: "50000baseCR/Full",
	55: "50000baseLR_ER_FR/Full",
	56: "50000baseDR/Full",
	57: "100000baseKR2/Full",
	58: "100000baseSR2/Full",
	59: "100000baseCR2/Full",
	60: "100000baseLR2_ER2_FR2/Full",
	61: "100000baseDR2/Full",
	62: "200000baseKR4/Full",
	63: "200000baseSR4/Full",
	64: "200000baseLR4_ER4_FR4/Full",
	65: "200000baseDR4/Full",
	66: "200000baseCR4/Full",
	67: "100baseT1/Full",
	68: "1000baseT1/Full",
	69: "400000baseKR8/Full",
	70: "400000baseSR8/Full",
	71: "400000baseLR8_ER8_FR8/Full",
	72: "400000baseDR8/Full",
	73: "400000baseCR8/Full",
	74: "FEC_LLRS",
	75: "100000baseKR/Full",
	76: "100000baseSR/Full",
	77: "100000baseLR_ER_FR/Full",
	78: "100000baseCR/Full",
	79: "100000baseDR/Full",
}

func bitsToNames(bits []byte, size uint32) []string {
	var out []string
	n := int(size)
	if n == 0 {
		n = len(bits) * 8
	}
	for i := 0; i < n; i++ {
		byteIdx := i / 8
		bitIdx := uint(i % 8)
		if byteIdx >= len(bits) {
			break
		}
		if bits[byteIdx]&(1<<bitIdx) == 0 {
			continue
		}
		if name, ok := linkModeNames[uint(i)]; ok {
			out = append(out, name)
		} else {
			out = append(out, "bit_"+itoa(i))
		}
	}
	return out
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(b[pos:])
}
