package neighbor

import (
	"sync"
	"time"
)

// Protocol identifies the discovery protocol.
type Protocol string

const (
	ProtoLLDP Protocol = "lldp"
	ProtoCDP  Protocol = "cdp"
)

// Neighbor is a discovered adjacent device on a local interface.
type Neighbor struct {
	Interface      string
	Protocol       Protocol
	ChassisID      string
	PortID         string
	SystemName     string
	SystemDesc     string
	PortDesc       string
	ManagementAddr string
	Platform       string
	Version        string
	Capabilities   string
	VLAN           string
	TTL            time.Duration
	LastSeen       time.Time
	RawSourceMAC   string
}

// Key uniquely identifies a neighbor on an interface.
func (n Neighbor) Key() string {
	return string(n.Protocol) + "|" + n.Interface + "|" + n.ChassisID + "|" + n.PortID
}

// Store keeps the latest neighbors with a configurable cache TTL.
type Store struct {
	mu    sync.RWMutex
	ttl   time.Duration
	items map[string]Neighbor
	// lastPDU is the most recent discovery frame time per interface.
	lastPDU map[string]time.Time
}

func NewStore(ttl time.Duration) *Store {
	if ttl <= 0 {
		ttl = DefaultConfig().Stale
	}
	return &Store{
		ttl:     ttl,
		items:   make(map[string]Neighbor),
		lastPDU: make(map[string]time.Time),
	}
}

func (s *Store) Upsert(n Neighbor) {
	if n.LastSeen.IsZero() {
		n.LastSeen = time.Now()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastPDU[n.Interface] = n.LastSeen
	// IEEE 802.1AB: TTL 0 means the neighbor information is no longer valid.
	if n.TTL == 0 {
		delete(s.items, n.Key())
		return
	}
	s.items[n.Key()] = n
}

// LastPDU returns when a discovery frame was last seen on iface.
func (s *Store) LastPDU(iface string) (time.Time, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.lastPDU[iface]
	return t, ok
}

// HasNeighbors reports whether iface currently has cached neighbors.
func (s *Store) HasNeighbors(iface string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now()
	for _, n := range s.items {
		if n.Interface != iface {
			continue
		}
		if now.Sub(n.LastSeen) <= s.expiry(n) {
			return true
		}
	}
	return false
}

func (s *Store) expiry(n Neighbor) time.Duration {
	if n.TTL == 0 {
		return 0
	}
	// Cache at least s.ttl; allow longer if the PDU advertised a larger TTL.
	expiry := s.ttl
	if n.TTL > expiry {
		expiry = n.TTL
	}
	return expiry
}

func (s *Store) List() []Neighbor {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Neighbor, 0, len(s.items))
	alive := make(map[string]struct{}, len(s.items))
	for k, n := range s.items {
		if now.Sub(n.LastSeen) > s.expiry(n) {
			delete(s.items, k)
			continue
		}
		out = append(out, n)
		alive[n.Interface] = struct{}{}
	}
	// Drop stale lastPDU entries for interfaces with no live neighbors.
	// Keep recent ones so adaptive capture can honor ActiveHold.
	pruneAfter := s.ttl * 2
	if pruneAfter < 30*time.Minute {
		pruneAfter = 30 * time.Minute
	}
	for iface, t := range s.lastPDU {
		if _, ok := alive[iface]; ok {
			continue
		}
		if now.Sub(t) > pruneAfter {
			delete(s.lastPDU, iface)
		}
	}
	return out
}
