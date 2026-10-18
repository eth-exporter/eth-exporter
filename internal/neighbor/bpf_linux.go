//go:build linux && !noneighbor

package neighbor

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

// sockFilter is the classic BPF instruction layout (linux/filter.h).
type sockFilter struct {
	Code uint16
	Jt   uint8
	Jf   uint8
	K    uint32
}

// attachDiscoveryFilter keeps only LLDP/CDP multicast destinations:
//
//	01:80:c2:00:00:0e / 00 / 03  (LLDP nearest-bridge variants)
//	01:00:0c:cc:cc:cc           (CDP)
func attachDiscoveryFilter(fd int) error {
	prog := []sockFilter{
		{Code: 0x20, K: 0},
		{Code: 0x15, Jt: 1, Jf: 0, K: 0x0180c200},
		{Code: 0x15, Jt: 4, Jf: 6, K: 0x01000ccc},
		{Code: 0x28, K: 4},
		{Code: 0x15, Jt: 5, Jf: 0, K: 0x000e},
		{Code: 0x15, Jt: 4, Jf: 0, K: 0x0000},
		{Code: 0x15, Jt: 3, Jf: 2, K: 0x0003},
		{Code: 0x28, K: 4},
		{Code: 0x15, Jt: 1, Jf: 0, K: 0xcccc},
		{Code: 0x06, K: 0},
		{Code: 0x06, K: 0xffff},
	}

	type sockFprog struct {
		Len    uint16
		Filter *sockFilter
	}
	fprog := sockFprog{
		Len:    uint16(len(prog)),
		Filter: &prog[0],
	}
	_, _, errno := unix.Syscall6(
		unix.SYS_SETSOCKOPT,
		uintptr(fd),
		uintptr(unix.SOL_SOCKET),
		uintptr(unix.SO_ATTACH_FILTER),
		uintptr(unsafe.Pointer(&fprog)),
		unsafe.Sizeof(fprog),
		0,
	)
	if errno != 0 {
		return errno
	}
	return nil
}
