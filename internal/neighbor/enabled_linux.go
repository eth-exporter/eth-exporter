//go:build linux && !noneighbor

package neighbor

// NeighborEnabled reports whether AF_PACKET discovery was compiled in.
const NeighborEnabled = true
