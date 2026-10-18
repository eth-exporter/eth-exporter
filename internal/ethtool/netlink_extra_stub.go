//go:build !linux

package ethtool

import "fmt"

func (n *Netlink) LinkState(string) (LinkState, error) {
	return LinkState{}, fmt.Errorf("unsupported platform")
}
func (n *Netlink) LinkModes(string) (LinkModes, error) {
	return LinkModes{}, fmt.Errorf("unsupported platform")
}
func (n *Netlink) PrivFlags(string) (PrivFlags, error) {
	return PrivFlags{}, fmt.Errorf("unsupported platform")
}
func (n *Netlink) RSS(string) (RSSInfo, error) {
	return RSSInfo{}, fmt.Errorf("unsupported platform")
}
func (n *Netlink) ModuleEEPROMPage(string, uint8, uint8, uint8, uint32, uint32) ([]byte, error) {
	return nil, fmt.Errorf("unsupported platform")
}
