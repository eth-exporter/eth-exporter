package ethtool

// LinkState is ETHTOOL_MSG_LINKSTATE_GET.
type LinkState struct {
	Link        bool
	SQI         uint32
	SQIMax      uint32
	ExtState    uint8
	ExtSubstate uint8
	ExtDownCnt  uint32
	HasSQI      bool
	HasExtState bool
	HasDownCnt  bool
}

// LinkModes is ETHTOOL_MSG_LINKMODES_GET.
type LinkModes struct {
	Autoneg          bool
	Speed            uint32
	Duplex           uint8
	Lanes            uint32
	MasterSlaveCfg   uint8
	MasterSlaveState uint8
	RateMatching     uint8
	Supported        []string // bit names set in supported mask
	Advertised       []string
	Partner          []string
	HasLanes         bool
}

// RSSInfo is ETHTOOL_MSG_RSS_GET (context 0).
type RSSInfo struct {
	Context   uint32
	HFunc     uint32
	IndirLen  int
	HKeyLen   int
	InputXfrm uint32
}

// PrivFlags is a set of active private-flag bit indices (names unknown without stringset).
type PrivFlags struct {
	ActiveBits []uint
	Size       uint32
}

// ModuleDOM is decoded digital optical monitoring from module EEPROM.
type ModuleDOM struct {
	Type           uint32
	Vendor         string
	PartNumber     string
	Serial         string
	TemperatureC   float64
	VoltageV       float64
	BiasmA         float64
	TxPowerdBm     float64
	RxPowerdBm     float64
	HasTemperature bool
	HasVoltage     bool
	HasBias        bool
	HasTxPower     bool
	HasRxPower     bool
	Channels       []ChannelDOM
	// CMIS is true when byte 0 carries a CMIS SFF-8024 identifier
	// (QSFP-DD, OSFP, QSFP112...). MediaLanes is the media lane count of the
	// first advertised application; FlatMemory means no paged memory (no
	// page 11h lane monitors).
	CMIS       bool
	MediaLanes int
	FlatMemory bool
}

// ChannelDOM is per-lane QSFP diagnostics.
type ChannelDOM struct {
	Lane       int
	BiasmA     float64
	TxPowerdBm float64
	RxPowerdBm float64
	// TxPowermW/RxPowermW are the linear readings (set by the CMIS decoder);
	// 0 means unknown for non-CMIS decoders.
	TxPowermW float64
	RxPowermW float64
}
