package ethtool

// StdStats holds standardized IEEE / RMON / PHY counters from ETHTOOL_MSG_STATS_GET.
type StdStats struct {
	// Counters keyed by group name then stat name.
	Groups map[string]map[string]uint64
	// RMON packet-size histograms.
	RMONHistRX []HistBucket
	RMONHistTX []HistBucket
}

// HistBucket is one RMON size histogram bin.
type HistBucket struct {
	Low  uint32
	High uint32
	Val  uint64
}

// PauseStats are pause-frame counters from ETHTOOL_MSG_PAUSE_GET + FLAG_STATS.
type PauseStats struct {
	TXFrames      uint64
	RXFrames      uint64
	TXPauseStorms uint64
	HasTXFrames   bool
	HasRXFrames   bool
	HasTXStorms   bool
}

// FECStats are FEC counters from ETHTOOL_MSG_FEC_GET + FLAG_STATS.
type FECStats struct {
	CorrectedBlocks     uint64
	UncorrectableBlocks uint64
	CorrectedBits       uint64
	HasCorrected        bool
	HasUncorrectable    bool
	HasCorrectedBits    bool
}

// Stable names for ETHTOOL_STATS_* groups (ETH_SS_STATS_STD).
var stdGroupNames = map[uint32]string{
	0: "eth-phy",
	1: "eth-mac",
	2: "eth-ctrl",
	3: "rmon",
	4: "phy",
}

// Stable counter names keyed by group then nested NLA type (0-based enum values
// from uapi/linux/ethtool_netlink.h — IEEE numbers in the constant names are
// clause references, not enum ordinals).
var stdStatNames = map[uint32]map[uint16]string{
	0: { // eth-phy
		0: "SymbolErrorDuringCarrier",
	},
	1: { // eth-mac
		0:  "FramesTransmittedOK",
		1:  "SingleCollisionFrames",
		2:  "MultipleCollisionFrames",
		3:  "FramesReceivedOK",
		4:  "FrameCheckSequenceErrors",
		5:  "AlignmentErrors",
		6:  "OctetsTransmittedOK",
		7:  "FramesWithDeferredXmissions",
		8:  "LateCollisions",
		9:  "FramesAbortedDueToXSColls",
		10: "FramesLostDueToIntMACXmitError",
		11: "CarrierSenseErrors",
		12: "OctetsReceivedOK",
		13: "FramesLostDueToIntMACRcvError",
		14: "MulticastFramesXmittedOK",
		15: "BroadcastFramesXmittedOK",
		16: "FramesWithExcessiveDeferral",
		17: "MulticastFramesReceivedOK",
		18: "BroadcastFramesReceivedOK",
		19: "InRangeLengthErrors",
		20: "OutOfRangeLengthField",
		21: "FrameTooLongErrors",
	},
	2: { // eth-ctrl
		0: "MACControlFramesTransmitted",
		1: "MACControlFramesReceived",
		2: "UnsupportedOpcodesReceived",
	},
	3: { // rmon
		0: "etherStatsUndersizePkts",
		1: "etherStatsOversizePkts",
		2: "etherStatsFragments",
		3: "etherStatsJabbers",
	},
	4: { // phy
		0: "rx_packets",
		1: "rx_bytes",
		2: "rx_errors",
		3: "tx_packets",
		4: "tx_bytes",
		5: "tx_errors",
	},
}
