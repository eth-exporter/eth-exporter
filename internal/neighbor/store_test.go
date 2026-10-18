package neighbor

import (
	"testing"
	"time"
)

func TestStoreCachesBeyondPDUTTL(t *testing.T) {
	s := NewStore(5 * time.Minute)
	s.Upsert(Neighbor{
		Interface: "eth0",
		Protocol:  ProtoLLDP,
		ChassisID: "a",
		PortID:    "1",
		TTL:       30 * time.Second,
		LastSeen:  time.Now().Add(-2 * time.Minute),
	})
	if got := s.List(); len(got) != 1 {
		t.Fatalf("expected cached neighbor, got %d", len(got))
	}
	if !s.HasNeighbors("eth0") {
		t.Fatal("expected HasNeighbors")
	}
}

func TestStoreExpiresAfterFloor(t *testing.T) {
	s := NewStore(1 * time.Second)
	s.Upsert(Neighbor{
		Interface: "eth0",
		Protocol:  ProtoLLDP,
		ChassisID: "a",
		PortID:    "1",
		TTL:       100 * time.Millisecond,
		LastSeen:  time.Now().Add(-2 * time.Second),
	})
	if got := s.List(); len(got) != 0 {
		t.Fatalf("expected expired neighbor, got %d", len(got))
	}
	if s.HasNeighbors("eth0") {
		t.Fatal("expected no neighbors after expiry")
	}
}

func TestStoreTTLZeroWithdraws(t *testing.T) {
	s := NewStore(10 * time.Minute)
	s.Upsert(Neighbor{
		Interface: "eth0",
		Protocol:  ProtoLLDP,
		ChassisID: "a",
		PortID:    "1",
		TTL:       120 * time.Second,
		LastSeen:  time.Now(),
	})
	s.Upsert(Neighbor{
		Interface: "eth0",
		Protocol:  ProtoLLDP,
		ChassisID: "a",
		PortID:    "1",
		TTL:       0,
		LastSeen:  time.Now(),
	})
	if got := s.List(); len(got) != 0 {
		t.Fatalf("expected TTL=0 withdraw, got %d", len(got))
	}
	if _, ok := s.LastPDU("eth0"); !ok {
		t.Fatal("lastPDU should remain after withdraw")
	}
}
