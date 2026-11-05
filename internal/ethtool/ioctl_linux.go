//go:build linux

package ethtool

import (
	"encoding/binary"
	"fmt"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	siocEthtool = 0x8946

	ethtoolGSet          = 0x00000001
	ethtoolGDrvInfo      = 0x00000003
	ethtoolGWoL          = 0x00000005
	ethtoolGLink         = 0x0000000a
	ethtoolGCoalesce     = 0x0000000e
	ethtoolGRingParam    = 0x00000010
	ethtoolGPauseParam   = 0x00000012
	ethtoolGStrings      = 0x0000001b
	ethtoolGStats        = 0x0000001d
	ethtoolGSSetInfo     = 0x00000037
	ethtoolGFeatures     = 0x0000003a
	ethtoolGChannels     = 0x0000003c
	ethtoolGModuleInfo   = 0x00000042
	ethtoolGEEE          = 0x00000044
	ethtoolGPHYStats     = 0x0000004a
	ethtoolGLinkSettings = 0x0000004c
	ethtoolGFECParam     = 0x00000050

	ethSSStats    = 1
	ethSSFeatures = 4
	ethSSPHYStats = 7 // ETH_SS_PHY_STATS
	ethGStringLen = 32

	duplexHalf    = 0x00
	duplexFull    = 0x01
	duplexUnknown = 0xff

	ifnameSize = 16
)

// Client performs ethtool ioctl requests against the kernel.
type Client struct {
	fd    int
	mu    sync.Mutex
	pages ModulePageReader
}

func New() (*Client, error) {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		return nil, fmt.Errorf("ethtool socket: %w", err)
	}
	return &Client{fd: fd}, nil
}

func (c *Client) Close() error {
	if c.fd >= 0 {
		err := unix.Close(c.fd)
		c.fd = -1
		return err
	}
	return nil
}

func (c *Client) ioctl(name string, data unsafe.Pointer) error {
	if name == "" || len(name) >= ifnameSize || strings.ContainsAny(name, "/\x00") {
		return fmt.Errorf("invalid interface name %q", name)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	var ifr ifreq
	copy(ifr.Name[:], name)
	ifr.Data = uintptr(data)
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(c.fd), siocEthtool, uintptr(unsafe.Pointer(&ifr)))
	if errno != 0 {
		return errno
	}
	return nil
}

type ifreq struct {
	Name [ifnameSize]byte
	Data uintptr
}

type drvInfo struct {
	Cmd         uint32
	Driver      [32]byte
	Version     [32]byte
	FWVersion   [32]byte
	BusInfo     [32]byte
	EromVersion [32]byte
	Reserved2   [12]byte
	NPrivFlags  uint32
	NStats      uint32
	TestInfoLen uint32
	EEDumpLen   uint32
	RegDumpLen  uint32
}

func (c *Client) DriverInfo(name string) (DrvInfo, error) {
	var info drvInfo
	info.Cmd = ethtoolGDrvInfo
	if err := c.ioctl(name, unsafe.Pointer(&info)); err != nil {
		return DrvInfo{}, err
	}
	return DrvInfo{
		Driver:      cstr(info.Driver[:]),
		Version:     cstr(info.Version[:]),
		FWVersion:   cstr(info.FWVersion[:]),
		BusInfo:     cstr(info.BusInfo[:]),
		EromVersion: cstr(info.EromVersion[:]),
		NStats:      info.NStats,
	}, nil
}

func (c *Client) stringSetLen(name string, setID uint32) (uint32, error) {
	// ETHTOOL_GSSET_INFO: cmd, reserved, sset_mask, data[1]
	buf := make([]byte, 4+4+8+8)
	binary.LittleEndian.PutUint32(buf[0:4], ethtoolGSSetInfo)
	binary.LittleEndian.PutUint64(buf[8:16], 1<<setID)
	if err := c.ioctl(name, unsafe.Pointer(&buf[0])); err != nil {
		return 0, err
	}
	mask := binary.LittleEndian.Uint64(buf[8:16])
	if mask&(1<<setID) == 0 {
		return 0, nil
	}
	return binary.LittleEndian.Uint32(buf[16:20]), nil
}

func (c *Client) strings(name string, setID uint32, n uint32) ([]string, error) {
	if n == 0 {
		return nil, nil
	}
	hdrSize := 12 // cmd + string_set + len
	buf := make([]byte, hdrSize+int(n)*ethGStringLen)
	binary.LittleEndian.PutUint32(buf[0:4], ethtoolGStrings)
	binary.LittleEndian.PutUint32(buf[4:8], setID)
	binary.LittleEndian.PutUint32(buf[8:12], n)
	if err := c.ioctl(name, unsafe.Pointer(&buf[0])); err != nil {
		return nil, err
	}
	out := make([]string, 0, n)
	for i := 0; i < int(n); i++ {
		off := hdrSize + i*ethGStringLen
		out = append(out, cstr(buf[off:off+ethGStringLen]))
	}
	return out, nil
}

func (c *Client) Stats(name string) (map[string]uint64, error) {
	return c.stats(name, ethSSStats, ethtoolGStats, true)
}

// PhyStats returns PHY-specific statistics (ETHTOOL_GPHYSTATS).
func (c *Client) PhyStats(name string) (map[string]uint64, error) {
	return c.stats(name, ethSSPHYStats, ethtoolGPHYStats, false)
}

func (c *Client) stats(name string, ssID, cmd uint32, fallbackDrvinfo bool) (map[string]uint64, error) {
	n, err := c.stringSetLen(name, ssID)
	if err != nil {
		if !fallbackDrvinfo {
			return nil, err
		}
		// Fall back to drvinfo.n_stats on older kernels (NIC stats only).
		info, ierr := c.DriverInfo(name)
		if ierr != nil {
			return nil, err
		}
		n = info.NStats
	}
	if n == 0 {
		return map[string]uint64{}, nil
	}
	names, err := c.strings(name, ssID, n)
	if err != nil {
		return nil, err
	}
	hdrSize := 8 // cmd + n_stats
	buf := make([]byte, hdrSize+int(n)*8)
	binary.LittleEndian.PutUint32(buf[0:4], cmd)
	binary.LittleEndian.PutUint32(buf[4:8], n)
	if err := c.ioctl(name, unsafe.Pointer(&buf[0])); err != nil {
		return nil, err
	}
	out := make(map[string]uint64, n)
	for i, s := range names {
		if s == "" {
			continue
		}
		off := hdrSize + i*8
		out[s] = binary.LittleEndian.Uint64(buf[off : off+8])
	}
	return out, nil
}

type value32 struct {
	Cmd  uint32
	Data uint32
}

func (c *Client) linkUp(name string) (bool, error) {
	v := value32{Cmd: ethtoolGLink}
	if err := c.ioctl(name, unsafe.Pointer(&v)); err != nil {
		return false, err
	}
	return v.Data != 0, nil
}

type ethtoolCmd struct {
	Cmd           uint32
	Supported     uint32
	Advertising   uint32
	Speed         uint16
	Duplex        uint8
	Port          uint8
	PhyAddress    uint8
	Transceiver   uint8
	Autoneg       uint8
	MDIOSupport   uint8
	MaxTxPkt      uint32
	MaxRxPkt      uint32
	SpeedHi       uint16
	EthTPMdix     uint8
	EthTPMdixCtrl uint8
	LPAdvertising uint32
	Reserved      [2]uint32
}

type linkSettings struct {
	Cmd                 uint32
	Speed               uint32
	Duplex              uint8
	Port                uint8
	PhyAddress          uint8
	Autoneg             uint8
	MDIOSupport         uint8
	EthTPMdix           uint8
	EthTPMdixCtrl       uint8
	LinkModeMasksNWords int8
	Transceiver         uint8
	MasterSlaveCfg      uint8
	MasterSlaveState    uint8
	RateMatching        uint8
	Reserved            [7]uint8
	// link mode masks follow as __u32[3 * nwords]
}

func (c *Client) LinkSettings(name string) (LinkSettings, error) {
	up, _ := c.linkUp(name)

	// struct ethtool_link_settings header is 48 bytes before link_mode_masks[].
	const linkSettingsHdr = 48
	const offNWords = 15
	const offTransceiver = 16

	// Handshake for GLINKSETTINGS: first call with nwords=0.
	// Kernel returns -EINVAL and a negative nwords on success of the handshake.
	handshake := make([]byte, linkSettingsHdr)
	binary.LittleEndian.PutUint32(handshake[0:4], ethtoolGLinkSettings)
	err := c.ioctl(name, unsafe.Pointer(&handshake[0]))
	nwords := int8(handshake[offNWords])
	if nwords < 0 && (err == nil || err == unix.EINVAL) {
		nwords = -nwords
		if nwords > 0 && nwords < 64 {
			buf := make([]byte, linkSettingsHdr+int(nwords)*3*4)
			copy(buf, handshake[:linkSettingsHdr])
			binary.LittleEndian.PutUint32(buf[0:4], ethtoolGLinkSettings)
			buf[offNWords] = byte(nwords)
			if err := c.ioctl(name, unsafe.Pointer(&buf[0])); err == nil {
				speed := binary.LittleEndian.Uint32(buf[4:8])
				return LinkSettings{
					Speed:        speed,
					Duplex:       duplexName(buf[8]),
					Port:         portName(buf[9]),
					Autoneg:      buf[11] != 0,
					LinkDetected: up,
					MDIOSupport:  buf[12],
					Transceiver:  buf[offTransceiver],
					PHYAddress:   buf[10],
					TPMdix:       mdixName(buf[13]),
				}, nil
			}
		}
	}

	// Legacy GSET fallback.
	var cmd ethtoolCmd
	cmd.Cmd = ethtoolGSet
	if err := c.ioctl(name, unsafe.Pointer(&cmd)); err != nil {
		return LinkSettings{LinkDetected: up}, err
	}
	speed := uint32(cmd.Speed) | uint32(cmd.SpeedHi)<<16
	return LinkSettings{
		Speed:        speed,
		Duplex:       duplexName(cmd.Duplex),
		Port:         portName(cmd.Port),
		Autoneg:      cmd.Autoneg != 0,
		LinkDetected: up,
		MDIOSupport:  cmd.MDIOSupport,
		Transceiver:  cmd.Transceiver,
		PHYAddress:   cmd.PhyAddress,
		TPMdix:       mdixName(cmd.EthTPMdix),
	}, nil
}

type coalesce struct {
	Cmd                uint32
	RxUsecs            uint32
	RxFrame            uint32
	RxUsecsIRQ         uint32
	RxFrameIRQ         uint32
	TxUsecs            uint32
	TxFrame            uint32
	TxUsecsIRQ         uint32
	TxFrameIRQ         uint32
	StatsBlockUsecs    uint32
	UseAdaptiveRX      uint32
	UseAdaptiveTX      uint32
	PktRateLow         uint32
	RxUsecsLow         uint32
	RxFrameLow         uint32
	TxUsecsLow         uint32
	TxFrameLow         uint32
	PktRateHigh        uint32
	RxUsecsHigh        uint32
	RxFrameHigh        uint32
	TxUsecsHigh        uint32
	TxFrameHigh        uint32
	RateSampleInterval uint32
}

func (c *Client) Coalesce(name string) (Coalesce, error) {
	var v coalesce
	v.Cmd = ethtoolGCoalesce
	if err := c.ioctl(name, unsafe.Pointer(&v)); err != nil {
		return Coalesce{}, err
	}
	return Coalesce{
		RxUsecs:            v.RxUsecs,
		RxMaxFrames:        v.RxFrame,
		TxUsecs:            v.TxUsecs,
		TxMaxFrames:        v.TxFrame,
		RxUsecsIRQ:         v.RxUsecsIRQ,
		RxMaxFramesIRQ:     v.RxFrameIRQ,
		TxUsecsIRQ:         v.TxUsecsIRQ,
		TxMaxFramesIRQ:     v.TxFrameIRQ,
		StatsBlockUsecs:    v.StatsBlockUsecs,
		UseAdaptiveRX:      v.UseAdaptiveRX != 0,
		UseAdaptiveTX:      v.UseAdaptiveTX != 0,
		PktRateLow:         v.PktRateLow,
		RxUsecsLow:         v.RxUsecsLow,
		RxMaxFramesLow:     v.RxFrameLow,
		TxUsecsLow:         v.TxUsecsLow,
		TxMaxFramesLow:     v.TxFrameLow,
		PktRateHigh:        v.PktRateHigh,
		RxUsecsHigh:        v.RxUsecsHigh,
		RxMaxFramesHigh:    v.RxFrameHigh,
		TxUsecsHigh:        v.TxUsecsHigh,
		TxMaxFramesHigh:    v.TxFrameHigh,
		RateSampleInterval: v.RateSampleInterval,
	}, nil
}

type ringParam struct {
	Cmd               uint32
	RXMaxPending      uint32
	RXMiniMaxPending  uint32
	RXJumboMaxPending uint32
	TXMaxPending      uint32
	RXPending         uint32
	RXMiniPending     uint32
	RXJumboPending    uint32
	TXPending         uint32
}

func (c *Client) RingParam(name string) (RingParam, error) {
	var v ringParam
	v.Cmd = ethtoolGRingParam
	if err := c.ioctl(name, unsafe.Pointer(&v)); err != nil {
		return RingParam{}, err
	}
	return RingParam{
		RXMaxPending:      v.RXMaxPending,
		RXMiniMaxPending:  v.RXMiniMaxPending,
		RXJumboMaxPending: v.RXJumboMaxPending,
		TXMaxPending:      v.TXMaxPending,
		RXPending:         v.RXPending,
		RXMiniPending:     v.RXMiniPending,
		RXJumboPending:    v.RXJumboPending,
		TXPending:         v.TXPending,
	}, nil
}

type pauseParam struct {
	Cmd     uint32
	Autoneg uint32
	RX      uint32
	TX      uint32
}

func (c *Client) PauseParam(name string) (PauseParam, error) {
	var v pauseParam
	v.Cmd = ethtoolGPauseParam
	if err := c.ioctl(name, unsafe.Pointer(&v)); err != nil {
		return PauseParam{}, err
	}
	return PauseParam{Autoneg: v.Autoneg != 0, RX: v.RX != 0, TX: v.TX != 0}, nil
}

type channels struct {
	Cmd           uint32
	MaxRX         uint32
	MaxTX         uint32
	MaxOther      uint32
	MaxCombined   uint32
	RXCount       uint32
	TXCount       uint32
	OtherCount    uint32
	CombinedCount uint32
}

func (c *Client) Channels(name string) (Channels, error) {
	var v channels
	v.Cmd = ethtoolGChannels
	if err := c.ioctl(name, unsafe.Pointer(&v)); err != nil {
		return Channels{}, err
	}
	return Channels{
		MaxRX: v.MaxRX, MaxTX: v.MaxTX, MaxOther: v.MaxOther, MaxCombined: v.MaxCombined,
		RXCount: v.RXCount, TXCount: v.TXCount, OtherCount: v.OtherCount, CombinedCount: v.CombinedCount,
	}, nil
}

type eee struct {
	Cmd          uint32
	Supported    uint32
	Advertised   uint32
	LPAdvertised uint32
	EEEActive    uint32
	EEEEnabled   uint32
	TxLpiEnabled uint32
	TxLpiTimer   uint32
	Reserved     [2]uint32
}

func (c *Client) EEE(name string) (EEE, error) {
	var v eee
	v.Cmd = ethtoolGEEE
	if err := c.ioctl(name, unsafe.Pointer(&v)); err != nil {
		return EEE{}, err
	}
	return EEE{
		Supported: v.Supported, Advertised: v.Advertised, LPAdvertised: v.LPAdvertised,
		EEEActive: v.EEEActive != 0, EEEEnabled: v.EEEEnabled != 0,
		TxLpiEnabled: v.TxLpiEnabled != 0, TxLpiTimer: v.TxLpiTimer,
	}, nil
}

type fecParam struct {
	Cmd       uint32
	ActiveFEC uint32
	FEC       uint32
	Reserved  uint32
}

func (c *Client) FEC(name string) (FEC, error) {
	var v fecParam
	v.Cmd = ethtoolGFECParam
	if err := c.ioctl(name, unsafe.Pointer(&v)); err != nil {
		return FEC{}, err
	}
	return FEC{Active: v.ActiveFEC, Configured: v.FEC}, nil
}

type wolInfo struct {
	Cmd       uint32
	Supported uint32
	Opts      uint32
	Sopass    [6]byte
}

func (c *Client) WoL(name string) (WoL, error) {
	var v wolInfo
	v.Cmd = ethtoolGWoL
	if err := c.ioctl(name, unsafe.Pointer(&v)); err != nil {
		return WoL{}, err
	}
	return WoL{Supported: v.Supported, Opts: v.Opts}, nil
}

type moduleInfo struct {
	Cmd       uint32
	Type      uint32
	EepromLen uint32
}

func (c *Client) ModuleInfo(name string) (ModuleInfo, error) {
	var v moduleInfo
	v.Cmd = ethtoolGModuleInfo
	if err := c.ioctl(name, unsafe.Pointer(&v)); err != nil {
		return ModuleInfo{}, err
	}
	return ModuleInfo{Type: v.Type, EEPROM: v.EepromLen}, nil
}

const ethtoolGModuleEEPROM = 0x00000043

// maxModuleEEPROM caps DOM/EEPROM reads (QSFP-DD pages stay well under this).
const maxModuleEEPROM = 16384

// ModuleEEPROM reads the full plug-in module EEPROM via ETHTOOL_GMODULEEEPROM.
func (c *Client) ModuleEEPROM(name string) (ModuleInfo, []byte, error) {
	info, err := c.ModuleInfo(name)
	if err != nil {
		return ModuleInfo{}, nil, err
	}
	if info.EEPROM == 0 {
		return info, nil, nil
	}
	n := int(info.EEPROM)
	if n > maxModuleEEPROM {
		return info, nil, fmt.Errorf("module EEPROM length %d exceeds cap %d", n, maxModuleEEPROM)
	}
	hdr := 16 // cmd, magic, offset, len
	buf := make([]byte, hdr+n)
	binary.LittleEndian.PutUint32(buf[0:4], ethtoolGModuleEEPROM)
	binary.LittleEndian.PutUint32(buf[4:8], 0) // magic
	binary.LittleEndian.PutUint32(buf[8:12], 0)
	binary.LittleEndian.PutUint32(buf[12:16], uint32(n))
	if err := c.ioctl(name, unsafe.Pointer(&buf[0])); err != nil {
		return info, nil, err
	}
	return info, buf[hdr : hdr+n], nil
}

// ModuleDOM reads and decodes optical diagnostics for a device.
//
// For CMIS modules the legacy ioctl only exposes page 00h, so per-lane
// monitors (page 11h) and the bias multiplier (page 01h) are fetched with
// ETHTOOL_MSG_MODULE_EEPROM_GET when a netlink client is supplied via
// SetModulePageReader. Failure to read the extra pages is not an error: the
// module-level temperature/voltage/identity from page 00h are still returned.
func (c *Client) ModuleDOM(name string) (ModuleDOM, error) {
	info, data, err := c.ModuleEEPROM(name)
	if err != nil {
		return ModuleDOM{}, err
	}
	if len(data) == 0 {
		return ModuleDOM{}, fmt.Errorf("empty module EEPROM")
	}
	dom := ParseModuleDOM(info.Type, data)
	if dom.CMIS && !dom.FlatMemory && c.pages != nil {
		p11, err11 := c.pages.ModuleEEPROMPage(name, 0x11, 0, 0x50, 128, 128)
		if err11 == nil && len(p11) >= 128 {
			p01, _ := c.pages.ModuleEEPROMPage(name, 0x01, 0, 0x50, 128, 128)
			ApplyCMISLaneMonitors(&dom, p01, p11)
		}
	}
	return dom, nil
}

// ModulePageReader reads one 128-byte half page of module EEPROM (netlink).
type ModulePageReader interface {
	ModuleEEPROMPage(dev string, page, bank, i2c uint8, offset, length uint32) ([]byte, error)
}

// SetModulePageReader enables paged (CMIS page 01h/11h) module reads.
func (c *Client) SetModulePageReader(r ModulePageReader) { c.pages = r }

// Features returns named feature bits via ETHTOOL_GFEATURES + string set.
func (c *Client) Features(name string) (map[string]FeatureState, error) {
	n, err := c.stringSetLen(name, ethSSFeatures)
	if err != nil || n == 0 {
		return map[string]FeatureState{}, err
	}
	names, err := c.strings(name, ethSSFeatures, n)
	if err != nil {
		return nil, err
	}
	blocks := (n + 31) / 32
	// struct ethtool_gfeatures { cmd, size, features[size]{available,requested,active,never_changed} }
	buf := make([]byte, 8+int(blocks)*16)
	binary.LittleEndian.PutUint32(buf[0:4], ethtoolGFeatures)
	binary.LittleEndian.PutUint32(buf[4:8], blocks)
	if err := c.ioctl(name, unsafe.Pointer(&buf[0])); err != nil {
		return nil, err
	}
	out := make(map[string]FeatureState, n)
	for i, fname := range names {
		if fname == "" {
			continue
		}
		block := i / 32
		bit := uint32(1) << (i % 32)
		base := 8 + block*16
		avail := binary.LittleEndian.Uint32(buf[base : base+4])
		req := binary.LittleEndian.Uint32(buf[base+4 : base+8])
		act := binary.LittleEndian.Uint32(buf[base+8 : base+12])
		never := binary.LittleEndian.Uint32(buf[base+12 : base+16])
		out[fname] = FeatureState{
			Available:    avail&bit != 0,
			Requested:    req&bit != 0,
			Active:       act&bit != 0,
			NeverChanged: never&bit != 0,
		}
	}
	return out, nil
}

func cstr(b []byte) string {
	if i := strings.IndexByte(string(b), 0); i >= 0 {
		return string(b[:i])
	}
	return string(b)
}

func duplexName(d uint8) string {
	switch d {
	case duplexHalf:
		return "half"
	case duplexFull:
		return "full"
	default:
		return "unknown"
	}
}

func portName(p uint8) string {
	switch p {
	case 0x00:
		return "tp"
	case 0x01:
		return "aui"
	case 0x02:
		return "mii"
	case 0x03:
		return "fibre"
	case 0x04:
		return "bnc"
	case 0x05:
		return "da"
	case 0xef:
		return "none"
	case 0xff:
		return "other"
	default:
		return fmt.Sprintf("port_%d", p)
	}
}

func mdixName(m uint8) string {
	switch m {
	case 0:
		return "invalid"
	case 1:
		return "mdi"
	case 2:
		return "mdi-x"
	case 3:
		return "auto"
	default:
		return fmt.Sprintf("mdix_%d", m)
	}
}
