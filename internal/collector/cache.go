package collector

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/scottmo/eth-exporter/internal/ethtool"
)

// ttlCache caches expensive per-device results and unsupported probes.
type ttlCache[T any] struct {
	mu       sync.Mutex
	ttl      time.Duration
	negTTL   time.Duration
	items    map[string]cacheEntry[T]
	inflight map[string]*cacheCall[T]
}

type cacheEntry[T any] struct {
	val     T
	err     error
	expires time.Time
}

type cacheCall[T any] struct {
	done chan struct{}
	val  T
	err  error
}

func newTTLCache[T any](ttl, negTTL time.Duration) *ttlCache[T] {
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	if negTTL <= 0 {
		negTTL = 5 * time.Minute
	}
	return &ttlCache[T]{
		ttl:      ttl,
		negTTL:   negTTL,
		items:    make(map[string]cacheEntry[T]),
		inflight: make(map[string]*cacheCall[T]),
	}
}

func (c *ttlCache[T]) getOr(key string, fn func() (T, error)) (T, error, bool) {
	now := time.Now()
	c.mu.Lock()
	if e, ok := c.items[key]; ok && now.Before(e.expires) {
		c.mu.Unlock()
		return e.val, e.err, true
	}
	if call, ok := c.inflight[key]; ok {
		c.mu.Unlock()
		<-call.done
		return call.val, call.err, true
	}
	call := &cacheCall[T]{done: make(chan struct{})}
	c.inflight[key] = call
	c.mu.Unlock()

	val, err := fn()

	c.mu.Lock()
	call.val, call.err = val, err
	close(call.done)
	delete(c.inflight, key)
	if err == nil {
		c.items[key] = cacheEntry[T]{val: val, expires: now.Add(c.ttl)}
	} else if isPermanentProbeErr(err) {
		c.items[key] = cacheEntry[T]{err: err, expires: now.Add(c.negTTL)}
	} else {
		delete(c.items, key)
	}
	c.purgeExpiredLocked(now)
	c.mu.Unlock()
	return val, err, false
}

func (c *ttlCache[T]) purgeExpiredLocked(now time.Time) {
	if len(c.items) < 256 {
		return
	}
	for k, e := range c.items {
		if now.After(e.expires) {
			delete(c.items, k)
		}
	}
}

func isPermanentProbeErr(err error) bool {
	if err == nil {
		return false
	}
	var to interface{ Timeout() bool }
	if errors.As(err, &to) && to.Timeout() {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, sub := range []string{
		"resource temporarily unavailable",
		"timed out",
		"timeout",
		"device or resource busy",
		"interrupted system call",
	} {
		if strings.Contains(msg, sub) {
			return false
		}
	}
	for _, sub := range []string{
		"operation not supported",
		"not supported",
		"no such device",
		"invalid argument",
		"permission denied",
		"protocol not available",
	} {
		if strings.Contains(msg, sub) {
			return true
		}
	}
	return false
}

type domCache = ttlCache[ethtool.ModuleDOM]
