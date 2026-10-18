package ethtool

// DrvInfo mirrors ETHTOOL_GDRVINFO.
type DrvInfo struct {
	Driver      string
	Version     string
	FWVersion   string
	BusInfo     string
	EromVersion string
	NStats      uint32
}

// LinkSettings is a subset of ETHTOOL_GLINKSETTINGS / ETHTOOL_GSET.
type LinkSettings struct {
	Speed        uint32 // Mbps; 0 or MaxUint32 if unknown
	Duplex       string // half, full, unknown
	Port         string
	Autoneg      bool
	LinkDetected bool
	MDIOSupport  uint8
	Transceiver  uint8
	PHYAddress   uint8
	TPMdix       string
}

// Coalesce mirrors ETHTOOL_GCOALESCE.
type Coalesce struct {
	RxUsecs          uint32
	RxMaxFrames      uint32
	TxUsecs          uint32
	TxMaxFrames      uint32
	RxUsecsIRQ       uint32
	RxMaxFramesIRQ   uint32
	TxUsecsIRQ       uint32
	TxMaxFramesIRQ   uint32
	StatsBlockUsecs  uint32
	UseAdaptiveRX    bool
	UseAdaptiveTX    bool
	PktRateLow       uint32
	RxUsecsLow       uint32
	RxMaxFramesLow   uint32
	TxUsecsLow       uint32
	TxMaxFramesLow   uint32
	PktRateHigh      uint32
	RxUsecsHigh      uint32
	RxMaxFramesHigh  uint32
	TxUsecsHigh      uint32
	TxMaxFramesHigh  uint32
	RateSampleInterval uint32
}

// RingParam mirrors ETHTOOL_GRINGPARAM.
type RingParam struct {
	RXMaxPending      uint32
	RXMiniMaxPending  uint32
	RXJumboMaxPending uint32
	TXMaxPending      uint32
	RXPending         uint32
	RXMiniPending     uint32
	RXJumboPending    uint32
	TXPending         uint32
}

// PauseParam mirrors ETHTOOL_GPAUSEPARAM.
type PauseParam struct {
	Autoneg bool
	RX      bool
	TX      bool
}

// Channels mirrors ETHTOOL_GCHANNELS.
type Channels struct {
	MaxRX           uint32
	MaxTX           uint32
	MaxOther        uint32
	MaxCombined     uint32
	RXCount         uint32
	TXCount         uint32
	OtherCount      uint32
	CombinedCount   uint32
}

// EEE mirrors ETHTOOL_GEEE.
type EEE struct {
	Supported       uint32
	Advertised      uint32
	LPAdvertised    uint32
	EEEActive       bool
	EEEEnabled      bool
	TxLpiEnabled    bool
	TxLpiTimer      uint32
}

// FEC mirrors ETHTOOL_GFECPARAM.
type FEC struct {
	Active    uint32
	Configured uint32
	Supported uint32
}

// FeatureState describes one offload/feature bit.
type FeatureState struct {
	Available bool
	Requested bool
	Active    bool
	NeverChanged bool
}

// WoL mirrors ETHTOOL_GWOL.
type WoL struct {
	Supported uint32
	Opts      uint32
}

// ModuleInfo mirrors ETHTOOL_GMODULEINFO.
type ModuleInfo struct {
	Type   uint32
	EEPROM uint32
}
