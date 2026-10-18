//go:build !linux

package netlink

import "fmt"

type Client struct{}

func New() (*Client, error) { return nil, fmt.Errorf("rtnetlink only on Linux") }
func (c *Client) Close() error { return nil }

func LinkIndex([]LinkInfo) map[int]string { return nil }

func (c *Client) Addresses(map[int]string) ([]AddrInfo, error) {
	return nil, fmt.Errorf("unsupported")
}
func (c *Client) Neighbors(map[int]string) ([]NeighInfo, error) {
	return nil, fmt.Errorf("unsupported")
}
func (c *Client) FDB(map[int]string) ([]FDBEntry, error) { return nil, fmt.Errorf("unsupported") }
func (c *Client) Links() ([]LinkInfo, error)             { return nil, fmt.Errorf("unsupported") }
func (c *Client) Qdiscs(map[int]string) ([]QdiscInfo, error) {
	return nil, fmt.Errorf("unsupported")
}

type AddrInfo struct {
	IfIndex int
	IfName  string
	Family  string
	Address string
	Prefix  int
	Scope   int
	Flags   uint32
}

type NeighInfo struct {
	IfIndex  int
	IfName   string
	Family   string
	IP       string
	LLAddr   string
	State    uint16
	NudState string
}

type FDBEntry struct {
	IfIndex int
	IfName  string
	MAC     string
	VLAN    uint16
	Master  int
	State   uint16
}

type LinkInfo struct {
	Index      int
	Name       string
	PermAddr   string
	XDPAttached bool
	XDPProgID  uint32
	OperState  uint8
	MTU        uint32
}

type QdiscInfo struct {
	IfIndex  int
	IfName   string
	Kind     string
	Handle   uint32
	Parent   uint32
	Bytes    uint64
	Packets  uint64
	Drops    uint64
	Overlimits uint64
	Qlen     uint32
	Backlog  uint32
}
