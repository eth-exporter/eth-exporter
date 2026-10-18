//go:build !linux

package netfilter

import "fmt"

type Client struct{}

func New() (*Client, error) { return nil, fmt.Errorf("nftables netlink only on Linux") }
func (c *Client) Close() error { return nil }

func (c *Client) Dump() (Snapshot, error) { return Snapshot{}, fmt.Errorf("unsupported") }

type Snapshot struct {
	Tables   []Table
	Chains   []Chain
	Counters []Counter
}

type Table struct {
	Family string
	Name   string
	Use    uint32
}

type Chain struct {
	Family string
	Table  string
	Name   string
}

type Counter struct {
	Family  string
	Table   string
	Name    string
	Bytes   uint64
	Packets uint64
}
