//go:build linux && !noneighbor

package neighbor

import (
	"context"
	"log/slog"
	"net"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

// Capture adaptively listens for LLDP/CDP:
//   - interfaces that speak discovery stay in continuous capture
//   - quiet interfaces are probed briefly, then back off exponentially
type Capture struct {
	logger *slog.Logger
	filter DeviceMatcher
	store  *Store
	cfg    Config

	done chan struct{}
	wg   sync.WaitGroup

	mu     sync.Mutex
	states map[string]*ifaceState
}

type ifaceState struct {
	name       string
	index      int
	backoff    time.Duration
	nextProbe  time.Time
	continuous bool
	cancel     context.CancelFunc
	gen        uint64 // bumped when a listen session is replaced
}

func NewCapture(logger *slog.Logger, filter DeviceMatcher, store *Store, cfg Config) (*Capture, error) {
	cfg = cfg.withDefaults()
	c := &Capture{
		logger: logger,
		filter: filter,
		store:  store,
		cfg:    cfg,
		done:   make(chan struct{}),
		states: make(map[string]*ifaceState),
	}
	if err := c.rescan(); err != nil {
		return nil, err
	}
	if len(c.states) == 0 {
		return nil, unix.ENODEV
	}
	return c, nil
}

func (c *Capture) Run() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	rescan := time.NewTicker(c.cfg.RescanIFaces)
	defer rescan.Stop()

	c.scheduleAll(time.Now())
	for {
		select {
		case <-c.done:
			return
		case <-rescan.C:
			_ = c.rescan()
		case now := <-ticker.C:
			c.scheduleAll(now)
		}
	}
}

func (c *Capture) Close() {
	select {
	case <-c.done:
	default:
		close(c.done)
	}
	c.mu.Lock()
	for _, st := range c.states {
		if st.cancel != nil {
			st.cancel()
			st.cancel = nil
		}
	}
	c.mu.Unlock()
	c.wg.Wait()
}

func (c *Capture) rescan() error {
	ifaces, err := net.Interfaces()
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	seen := make(map[string]struct{}, len(ifaces))
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if c.filter != nil && !c.filter.Match(iface.Name) {
			continue
		}
		seen[iface.Name] = struct{}{}
		if st, ok := c.states[iface.Name]; ok {
			st.index = iface.Index
			continue
		}
		c.states[iface.Name] = &ifaceState{
			name:      iface.Name,
			index:     iface.Index,
			backoff:   c.cfg.ProbeMin,
			nextProbe: time.Now(), // probe soon on first sight
		}
		c.logger.Debug("neighbor: watching interface", "iface", iface.Name)
	}
	for name, st := range c.states {
		if _, ok := seen[name]; ok {
			continue
		}
		if st.cancel != nil {
			st.cancel()
		}
		delete(c.states, name)
		c.logger.Debug("neighbor: dropped interface", "iface", name)
	}
	return nil
}

func (c *Capture) scheduleAll(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, st := range c.states {
		c.scheduleLocked(st, now)
	}
}

func (c *Capture) continuousCountLocked() int {
	n := 0
	for _, st := range c.states {
		if st.continuous {
			n++
		}
	}
	return n
}

func (c *Capture) scheduleLocked(st *ifaceState, now time.Time) {
	last, saw := c.store.LastPDU(st.name)
	active := saw && now.Sub(last) < c.cfg.ActiveHold

	if active {
		if !st.continuous {
			if c.continuousCountLocked() >= c.cfg.MaxContinuous {
				// Cap raw sockets; keep probing instead of continuous listen.
				if st.cancel == nil && !now.Before(st.nextProbe) {
					c.startProbeLocked(st)
				}
				return
			}
			c.startContinuousLocked(st)
		}
		st.backoff = c.cfg.ProbeMin
		return
	}

	// No longer active: stop continuous listener if running.
	if st.continuous {
		c.stopListenLocked(st)
		st.continuous = false
		st.nextProbe = now // allow an immediate quiet-state probe schedule
		st.backoff = c.cfg.ProbeMin
	}

	if st.cancel != nil {
		// Probe window still running.
		return
	}
	if now.Before(st.nextProbe) {
		return
	}
	c.startProbeLocked(st)
}

func (c *Capture) startContinuousLocked(st *ifaceState) {
	c.stopListenLocked(st)
	ctx, cancel := context.WithCancel(context.Background())
	st.gen++
	st.cancel = cancel
	st.continuous = true
	c.wg.Add(1)
	go c.listen(ctx, st.name, st.index, 0) // 0 = until cancel
	c.logger.Debug("neighbor: continuous capture", "iface", st.name)
}

func (c *Capture) startProbeLocked(st *ifaceState) {
	c.stopListenLocked(st)
	ctx, cancel := context.WithTimeout(context.Background(), c.cfg.ProbeWindow)
	st.gen++
	gen := st.gen
	st.cancel = cancel
	st.continuous = false
	st.nextProbe = time.Now().Add(c.cfg.ProbeWindow + st.backoff)
	c.wg.Add(1)
	go func(name string, index int, backoff time.Duration, gen uint64) {
		heard := c.listen(ctx, name, index, c.cfg.ProbeWindow)
		c.mu.Lock()
		defer c.mu.Unlock()
		st := c.states[name]
		if st == nil || st.gen != gen {
			// Replaced by continuous capture or interface removal.
			return
		}
		st.cancel = nil
		if heard {
			st.backoff = c.cfg.ProbeMin
			st.nextProbe = time.Now() // scheduleAll promotes to continuous
			c.logger.Debug("neighbor: probe hit", "iface", name)
			return
		}
		st.backoff = backoff * 2
		if st.backoff > c.cfg.ProbeMax {
			st.backoff = c.cfg.ProbeMax
		}
		st.nextProbe = time.Now().Add(st.backoff)
		c.logger.Debug("neighbor: probe miss; backing off", "iface", name, "backoff", st.backoff)
	}(st.name, st.index, st.backoff, gen)
	c.logger.Debug("neighbor: probing", "iface", st.name, "window", c.cfg.ProbeWindow, "next_backoff", st.backoff)
}

func (c *Capture) stopListenLocked(st *ifaceState) {
	if st.cancel != nil {
		st.cancel()
		st.cancel = nil
	}
	st.continuous = false
}

// listen opens a filtered packet socket and reads until ctx done.
// Returns true if at least one discovery PDU was parsed.
func (c *Capture) listen(ctx context.Context, iface string, ifindex int, _ time.Duration) bool {
	defer c.wg.Done()
	fd, err := openPacketSocket(ifindex)
	if err != nil {
		c.logger.Debug("neighbor: open socket failed", "iface", iface, "err", err)
		return false
	}
	defer unix.Close(fd)

	_ = unix.SetNonblock(fd, true)
	buf := make([]byte, 2048)
	heard := false
	for {
		select {
		case <-ctx.Done():
			return heard
		case <-c.done:
			return heard
		default:
		}
		n, _, err := unix.Recvfrom(fd, buf, 0)
		if err != nil {
			if err == unix.EAGAIN || err == unix.EWOULDBLOCK {
				pfd := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
				_, _ = unix.Poll(pfd, 250)
				continue
			}
			select {
			case <-ctx.Done():
				return heard
			case <-c.done:
				return heard
			default:
				time.Sleep(100 * time.Millisecond)
				continue
			}
		}
		if n < 14 {
			continue
		}
		frame := buf[:n]
		if neigh, ok := ParseLLDP(iface, frame); ok {
			c.store.Upsert(neigh)
			heard = true
			continue
		}
		if neigh, ok := ParseCDP(iface, frame); ok {
			c.store.Upsert(neigh)
			heard = true
		}
	}
}

func openPacketSocket(ifindex int) (int, error) {
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW, int(htons(unix.ETH_P_ALL)))
	if err != nil {
		return -1, err
	}
	addr := unix.SockaddrLinklayer{
		Protocol: htons(unix.ETH_P_ALL),
		Ifindex:  ifindex,
	}
	if err := unix.Bind(fd, &addr); err != nil {
		_ = unix.Close(fd)
		return -1, err
	}
	for _, mac := range [][]byte{
		{0x01, 0x80, 0xc2, 0x00, 0x00, 0x0e},
		{0x01, 0x80, 0xc2, 0x00, 0x00, 0x00},
		{0x01, 0x80, 0xc2, 0x00, 0x00, 0x03},
		{0x01, 0x00, 0x0c, 0xcc, 0xcc, 0xcc},
	} {
		mreq := unix.PacketMreq{
			Ifindex: int32(ifindex),
			Type:    unix.PACKET_MR_MULTICAST,
		}
		copy(mreq.Address[:], mac)
		_ = unix.SetsockoptPacketMreq(fd, unix.SOL_PACKET, unix.PACKET_ADD_MEMBERSHIP, &mreq)
	}
	if err := attachDiscoveryFilter(fd); err != nil {
		_ = unix.Close(fd)
		return -1, err
	}
	_ = unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_RCVBUF, 64*1024)
	return fd, nil
}

func htons(v uint16) uint16 {
	return (v<<8)&0xff00 | v>>8
}
