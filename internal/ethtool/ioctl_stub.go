//go:build !linux

package ethtool

import "fmt"

// Client is a stub on non-Linux platforms.
type Client struct{}

func New() (*Client, error) {
	return nil, fmt.Errorf("ethtool ioctl is only supported on Linux")
}

func (c *Client) Close() error { return nil }

func (c *Client) DriverInfo(string) (DrvInfo, error) {
	return DrvInfo{}, fmt.Errorf("unsupported platform")
}

func (c *Client) Stats(string) (map[string]uint64, error) {
	return nil, fmt.Errorf("unsupported platform")
}

func (c *Client) PhyStats(string) (map[string]uint64, error) {
	return nil, fmt.Errorf("unsupported platform")
}

func (c *Client) LinkSettings(string) (LinkSettings, error) {
	return LinkSettings{}, fmt.Errorf("unsupported platform")
}

func (c *Client) Coalesce(string) (Coalesce, error) {
	return Coalesce{}, fmt.Errorf("unsupported platform")
}

func (c *Client) RingParam(string) (RingParam, error) {
	return RingParam{}, fmt.Errorf("unsupported platform")
}

func (c *Client) PauseParam(string) (PauseParam, error) {
	return PauseParam{}, fmt.Errorf("unsupported platform")
}

func (c *Client) Channels(string) (Channels, error) {
	return Channels{}, fmt.Errorf("unsupported platform")
}

func (c *Client) EEE(string) (EEE, error) {
	return EEE{}, fmt.Errorf("unsupported platform")
}

func (c *Client) FEC(string) (FEC, error) {
	return FEC{}, fmt.Errorf("unsupported platform")
}

func (c *Client) Features(string) (map[string]FeatureState, error) {
	return nil, fmt.Errorf("unsupported platform")
}

func (c *Client) WoL(string) (WoL, error) {
	return WoL{}, fmt.Errorf("unsupported platform")
}

func (c *Client) ModuleInfo(string) (ModuleInfo, error) {
	return ModuleInfo{}, fmt.Errorf("unsupported platform")
}

func (c *Client) ModuleEEPROM(string) (ModuleInfo, []byte, error) {
	return ModuleInfo{}, nil, fmt.Errorf("unsupported platform")
}

func (c *Client) ModuleDOM(string) (ModuleDOM, error) {
	return ModuleDOM{}, fmt.Errorf("unsupported platform")
}
