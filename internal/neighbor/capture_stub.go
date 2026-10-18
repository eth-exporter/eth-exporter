//go:build !linux && !noneighbor

package neighbor

import (
	"fmt"
	"log/slog"
)

// Capture is a stub on non-Linux platforms.
type Capture struct{}

func NewCapture(logger *slog.Logger, filter DeviceMatcher, store *Store, cfg Config) (*Capture, error) {
	return nil, fmt.Errorf("AF_PACKET neighbor capture is only supported on Linux")
}

func (c *Capture) Run()   {}
func (c *Capture) Close() {}

// NeighborEnabled is true when the neighbor package was built with capture support
// for this GOOS (still requires Linux at runtime).
const NeighborEnabled = true
