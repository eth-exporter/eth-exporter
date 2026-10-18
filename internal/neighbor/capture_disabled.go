//go:build noneighbor

package neighbor

import (
	"fmt"
	"log/slog"
)

// Capture is unavailable when built with -tags noneighbor.
type Capture struct{}

func NewCapture(logger *slog.Logger, filter DeviceMatcher, store *Store, cfg Config) (*Capture, error) {
	return nil, fmt.Errorf("neighbor capture disabled at build time (-tags noneighbor)")
}

func (c *Capture) Run()   {}
func (c *Capture) Close() {}

// NeighborEnabled reports whether AF_PACKET discovery was compiled in.
const NeighborEnabled = false
