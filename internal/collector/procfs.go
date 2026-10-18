package collector

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// SoftnetCPU is one /proc/net/softnet_stat row.
type SoftnetCPU struct {
	CPU            int
	Processed      uint64
	Dropped        uint64
	TimeSqueeze    uint64
	ReceivedRPS    uint64
	FlowLimitCount uint64
}

// ProcCounter is a named counter from /proc/net/snmp or netstat.
type ProcCounter struct {
	Protocol string
	Name     string
	Value    uint64
}

func readSoftnet(procfs string) ([]SoftnetCPU, error) {
	f, err := os.Open(filepath.Join(procfs, "net", "softnet_stat"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []SoftnetCPU
	sc := bufio.NewScanner(f)
	cpu := 0
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 3 {
			continue
		}
		row := SoftnetCPU{CPU: cpu}
		row.Processed, _ = strconv.ParseUint(fields[0], 16, 64)
		row.Dropped, _ = strconv.ParseUint(fields[1], 16, 64)
		row.TimeSqueeze, _ = strconv.ParseUint(fields[2], 16, 64)
		if len(fields) > 4 {
			row.ReceivedRPS, _ = strconv.ParseUint(fields[4], 16, 64)
		}
		if len(fields) > 5 {
			row.FlowLimitCount, _ = strconv.ParseUint(fields[5], 16, 64)
		}
		out = append(out, row)
		cpu++
	}
	return out, sc.Err()
}

func readProcSNMP(procfs string) ([]ProcCounter, error) {
	var out []ProcCounter
	for _, name := range []string{"snmp", "netstat"} {
		path := filepath.Join(procfs, "net", name)
		rows, err := parseSNMPFile(path)
		if err != nil {
			continue
		}
		out = append(out, rows...)
	}
	return out, nil
}

func parseSNMPFile(path string) ([]ProcCounter, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []ProcCounter
	sc := bufio.NewScanner(f)
	var names []string
	var proto string
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		p := strings.TrimSuffix(fields[0], ":")
		if names == nil || p != proto {
			proto = p
			names = fields[1:]
			continue
		}
		// values line
		for i := 1; i < len(fields) && i-1 < len(names); i++ {
			v, err := strconv.ParseUint(fields[i], 10, 64)
			if err != nil {
				continue
			}
			out = append(out, ProcCounter{Protocol: proto, Name: names[i-1], Value: v})
		}
		names = nil
	}
	return out, sc.Err()
}
