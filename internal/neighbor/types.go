package neighbor

import "time"

// DeviceMatcher selects interfaces to monitor.
type DeviceMatcher interface {
	Match(name string) bool
}

// Config controls neighbor caching and adaptive capture backoff.
type Config struct {
	// Stale is how long to retain a neighbor in the metric store after last
	// hearing it. Discovery PDUs often advertise ~120s TTLs; this floor lets
	// you cache longer since adjacencies rarely change.
	Stale time.Duration

	// ActiveHold keeps continuous capture on an interface after the last PDU.
	ActiveHold time.Duration

	// ProbeWindow is how long to listen when checking a quiet interface.
	ProbeWindow time.Duration

	// ProbeMin is the initial delay between probes on quiet interfaces.
	ProbeMin time.Duration

	// ProbeMax caps exponential backoff for interfaces that never speak
	// LLDP/CDP.
	ProbeMax time.Duration

	// RescanIFaces is how often to rediscover interfaces for capture.
	RescanIFaces time.Duration

	// MaxContinuous caps simultaneous continuous AF_PACKET listeners.
	// Additional active interfaces fall back to periodic probes.
	MaxContinuous int
}

// DefaultConfig returns sane caching / backoff defaults.
func DefaultConfig() Config {
	return Config{
		Stale:         10 * time.Minute,
		ActiveHold:    3 * time.Minute,
		ProbeWindow:   35 * time.Second,
		ProbeMin:      1 * time.Minute,
		ProbeMax:      15 * time.Minute,
		RescanIFaces:  2 * time.Minute,
		MaxContinuous: 16,
	}
}

func (c Config) withDefaults() Config {
	d := DefaultConfig()
	if c.Stale <= 0 {
		c.Stale = d.Stale
	}
	if c.ActiveHold <= 0 {
		c.ActiveHold = d.ActiveHold
	}
	if c.ProbeWindow <= 0 {
		c.ProbeWindow = d.ProbeWindow
	}
	if c.ProbeMin <= 0 {
		c.ProbeMin = d.ProbeMin
	}
	if c.ProbeMax <= 0 {
		c.ProbeMax = d.ProbeMax
	}
	if c.ProbeMax < c.ProbeMin {
		c.ProbeMax = c.ProbeMin
	}
	if c.RescanIFaces <= 0 {
		c.RescanIFaces = d.RescanIFaces
	}
	if c.MaxContinuous <= 0 {
		c.MaxContinuous = d.MaxContinuous
	}
	return c
}
