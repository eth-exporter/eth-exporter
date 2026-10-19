//go:build linux

package ethtool

import (
	"fmt"
)

const (
	ethMsgLinkModesGet    = 4
	ethMsgLinkStateGet    = 6
	ethMsgPrivFlagsGet    = 13
	ethMsgModuleEEPROMGet = 31
	ethMsgRSSGet          = 38

	ethALinkModesHeader           = 1
	ethALinkModesAutoneg          = 2
	ethALinkModesOurs             = 3
	ethALinkModesPeer             = 4
	ethALinkModesSpeed            = 5
	ethALinkModesDuplex           = 6
	ethALinkModesMasterSlaveCfg   = 7
	ethALinkModesMasterSlaveState = 8
	ethALinkModesLanes            = 9
	ethALinkModesRateMatching     = 10

	ethALinkStateHeader      = 1
	ethALinkStateLink        = 2
	ethALinkStateSQI         = 3
	ethALinkStateSQIMax      = 4
	ethALinkStateExtState    = 5
	ethALinkStateExtSubstate = 6
	ethALinkStateExtDownCnt  = 7

	ethAPrivFlagsHeader = 1
	ethAPrivFlagsFlags  = 2

	ethAModuleEEPROMHeader     = 1
	ethAModuleEEPROMOffset     = 2
	ethAModuleEEPROMLength     = 3
	ethAModuleEEPROMPage       = 4
	ethAModuleEEPROMBank       = 5
	ethAModuleEEPROMI2CAddress = 6
	ethAModuleEEPROMData       = 7

	ethARSSHeader    = 1
	ethARSSContext   = 2
	ethARSSHFunc     = 3
	ethARSSIndir     = 4
	ethARSSHKey      = 5
	ethARSSInputXfrm = 6

	ethABitsetMask = 5
)

func (n *Netlink) getHeaderOnly(cmd uint8, headerAttr uint16, dev string, flags uint32) ([][]byte, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	var body nlBuilder
	var hdr nlBuilder
	hdr.putString(ethAHeaderDevName, dev)
	if flags != 0 {
		hdr.putUint32(ethAHeaderFlags, flags)
	}
	body.putNested(headerAttr, hdr.bytes())
	msg := body.finish(n.familyID, cmd, ethtoolGenlVersion, n.nextSeqLocked())
	return n.transactLocked(msg)
}

func (n *Netlink) LinkState(dev string) (LinkState, error) {
	var out LinkState
	replies, err := n.getHeaderOnly(ethMsgLinkStateGet, ethALinkStateHeader, dev, 0)
	if err != nil {
		return out, err
	}
	for _, r := range replies {
		attrs, err := parseAttrs(r)
		if err != nil {
			continue
		}
		if v, ok := attrs.uint8(ethALinkStateLink); ok {
			out.Link = v != 0
		}
		if v, ok := attrs.uint32(ethALinkStateSQI); ok {
			out.SQI, out.HasSQI = v, true
		}
		if v, ok := attrs.uint32(ethALinkStateSQIMax); ok {
			out.SQIMax = v
		}
		if v, ok := attrs.uint8(ethALinkStateExtState); ok {
			out.ExtState, out.HasExtState = v, true
		}
		if v, ok := attrs.uint8(ethALinkStateExtSubstate); ok {
			out.ExtSubstate = v
		}
		if v, ok := attrs.uint32(ethALinkStateExtDownCnt); ok {
			out.ExtDownCnt, out.HasDownCnt = v, true
		}
	}
	return out, nil
}

func (n *Netlink) LinkModes(dev string) (LinkModes, error) {
	var out LinkModes
	replies, err := n.getHeaderOnly(ethMsgLinkModesGet, ethALinkModesHeader, dev, 0)
	if err != nil {
		return out, err
	}
	for _, r := range replies {
		attrs, err := parseAttrs(r)
		if err != nil {
			continue
		}
		if v, ok := attrs.uint8(ethALinkModesAutoneg); ok {
			out.Autoneg = v != 0
		}
		if v, ok := attrs.uint32(ethALinkModesSpeed); ok {
			out.Speed = v
		}
		if v, ok := attrs.uint8(ethALinkModesDuplex); ok {
			out.Duplex = v
		}
		if v, ok := attrs.uint32(ethALinkModesLanes); ok {
			out.Lanes, out.HasLanes = v, true
		}
		if v, ok := attrs.uint8(ethALinkModesMasterSlaveCfg); ok {
			out.MasterSlaveCfg = v
		}
		if v, ok := attrs.uint8(ethALinkModesMasterSlaveState); ok {
			out.MasterSlaveState = v
		}
		if v, ok := attrs.uint8(ethALinkModesRateMatching); ok {
			out.RateMatching = v
		}
		if raw, ok := attrs.bytes(ethALinkModesOurs); ok {
			supp, adv := parseBitsetPair(raw)
			out.Supported = bitsToNames(supp, 0)
			out.Advertised = bitsToNames(adv, 0)
		}
		if raw, ok := attrs.bytes(ethALinkModesPeer); ok {
			_, peer := parseBitsetPair(raw)
			if len(peer) == 0 {
				// NOMASK peer: value only
				peer = parseBitsetValue(raw)
			}
			out.Partner = bitsToNames(peer, 0)
		}
	}
	return out, nil
}

func parseBitsetPair(raw []byte) (mask, value []byte) {
	attrs, err := parseAttrs(raw)
	if err != nil {
		return nil, nil
	}
	value, _ = attrs.bytes(ethABitsetValue)
	mask, _ = attrs.bytes(ethABitsetMask)
	if mask == nil {
		// NOMASK: value bits are the set; treat as both supported+advertised
		mask = value
	}
	return mask, value
}

func parseBitsetValue(raw []byte) []byte {
	attrs, err := parseAttrs(raw)
	if err != nil {
		return nil
	}
	v, _ := attrs.bytes(ethABitsetValue)
	return v
}

func (n *Netlink) PrivFlags(dev string) (PrivFlags, error) {
	var out PrivFlags
	replies, err := n.getHeaderOnly(ethMsgPrivFlagsGet, ethAPrivFlagsHeader, dev, 0)
	if err != nil {
		return out, err
	}
	for _, r := range replies {
		attrs, err := parseAttrs(r)
		if err != nil {
			continue
		}
		raw, ok := attrs.bytes(ethAPrivFlagsFlags)
		if !ok {
			continue
		}
		bs, err := parseAttrs(raw)
		if err != nil {
			continue
		}
		size, _ := bs.uint32(ethABitsetSize)
		val, _ := bs.bytes(ethABitsetValue)
		out.Size = size
		nbits := int(size)
		if nbits == 0 {
			nbits = len(val) * 8
		}
		for i := 0; i < nbits; i++ {
			if i/8 >= len(val) {
				break
			}
			if val[i/8]&(1<<(i%8)) != 0 {
				out.ActiveBits = append(out.ActiveBits, uint(i))
			}
		}
	}
	return out, nil
}

func (n *Netlink) RSS(dev string) (RSSInfo, error) {
	var out RSSInfo
	n.mu.Lock()
	defer n.mu.Unlock()
	var body nlBuilder
	var hdr nlBuilder
	hdr.putString(ethAHeaderDevName, dev)
	body.putNested(ethARSSHeader, hdr.bytes())
	body.putUint32(ethARSSContext, 0)
	msg := body.finish(n.familyID, ethMsgRSSGet, ethtoolGenlVersion, n.nextSeqLocked())
	replies, err := n.transactLocked(msg)
	if err != nil {
		return out, err
	}
	for _, r := range replies {
		attrs, err := parseAttrs(r)
		if err != nil {
			continue
		}
		if v, ok := attrs.uint32(ethARSSContext); ok {
			out.Context = v
		}
		if v, ok := attrs.uint32(ethARSSHFunc); ok {
			out.HFunc = v
		}
		if v, ok := attrs.bytes(ethARSSIndir); ok {
			out.IndirLen = len(v) / 4
		}
		if v, ok := attrs.bytes(ethARSSHKey); ok {
			out.HKeyLen = len(v)
		}
		if v, ok := attrs.uint32(ethARSSInputXfrm); ok {
			out.InputXfrm = v
		}
	}
	return out, nil
}

// ModuleEEPROMPage reads a page of module EEPROM via genetlink.
func (n *Netlink) ModuleEEPROMPage(dev string, page, bank, i2c uint8, offset, length uint32) ([]byte, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	var body nlBuilder
	var hdr nlBuilder
	hdr.putString(ethAHeaderDevName, dev)
	body.putNested(ethAModuleEEPROMHeader, hdr.bytes())
	body.putUint32(ethAModuleEEPROMOffset, offset)
	body.putUint32(ethAModuleEEPROMLength, length)
	body.putUint8(ethAModuleEEPROMPage, page)
	body.putUint8(ethAModuleEEPROMBank, bank)
	body.putUint8(ethAModuleEEPROMI2CAddress, i2c)
	msg := body.finish(n.familyID, ethMsgModuleEEPROMGet, ethtoolGenlVersion, n.nextSeqLocked())
	replies, err := n.transactLocked(msg)
	if err != nil {
		return nil, err
	}
	for _, r := range replies {
		attrs, err := parseAttrs(r)
		if err != nil {
			continue
		}
		if d, ok := attrs.bytes(ethAModuleEEPROMData); ok {
			return d, nil
		}
	}
	return nil, fmt.Errorf("no EEPROM data")
}

func (m attrMap) uint8(typ uint16) (uint8, bool) {
	d, ok := m.bytes(typ)
	if !ok || len(d) < 1 {
		return 0, false
	}
	return d[0], true
}

func (b *nlBuilder) putUint8(typ uint16, v uint8) {
	b.put(typ, []byte{v})
}
