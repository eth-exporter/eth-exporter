//go:build !linux

package ethtool

import "fmt"

// Netlink is a stub on non-Linux platforms.
type Netlink struct{}

func NewNetlink() (*Netlink, error) {
	return nil, fmt.Errorf("ethtool genetlink is only supported on Linux")
}

func (n *Netlink) Close() error { return nil }

func (n *Netlink) StandardStats(string) (StdStats, error) {
	return StdStats{}, fmt.Errorf("unsupported platform")
}

func (n *Netlink) PauseStats(string) (PauseStats, error) {
	return PauseStats{}, fmt.Errorf("unsupported platform")
}

func (n *Netlink) FECStats(string) (FECStats, error) {
	return FECStats{}, fmt.Errorf("unsupported platform")
}
