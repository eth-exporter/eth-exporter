package collector

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ConntrackSummary is global nf_conntrack pressure + per-CPU stats.
type ConntrackSummary struct {
	Count    uint64
	Max      uint64
	HasCount bool
	HasMax   bool
	// StatNames are column headers from /proc/net/stat/nf_conntrack.
	StatNames []string
	// PerCPU rows aligned with StatNames.
	PerCPU [][]uint64
}

func readConntrack(procfs string) (ConntrackSummary, error) {
	var s ConntrackSummary
	if v, err := readProcUint(filepath.Join(procfs, "sys", "net", "netfilter", "nf_conntrack_count")); err == nil {
		s.Count, s.HasCount = v, true
	}
	if v, err := readProcUint(filepath.Join(procfs, "sys", "net", "netfilter", "nf_conntrack_max")); err == nil {
		s.Max, s.HasMax = v, true
	}
	f, err := os.Open(filepath.Join(procfs, "net", "stat", "nf_conntrack"))
	if err != nil {
		if s.HasCount || s.HasMax {
			return s, nil
		}
		return s, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		return s, sc.Err()
	}
	s.StatNames = strings.Fields(sc.Text())
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		row := make([]uint64, len(s.StatNames))
		for i := 0; i < len(fields) && i < len(s.StatNames); i++ {
			row[i], _ = strconv.ParseUint(fields[i], 16, 64)
		}
		s.PerCPU = append(s.PerCPU, row)
	}
	return s, sc.Err()
}

func readProcUint(path string) (uint64, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
}
