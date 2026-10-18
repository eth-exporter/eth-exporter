package collector

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// RDMADevice is one InfiniBand / RDMA HCA from sysfs.
type RDMADevice struct {
	Name     string
	NodeType string
	FWVer    string
	HCAType  string
	NodeGUID string
	BoardID  string
	Ports    []RDMAPort
}

// RDMAPort is one HCA port.
type RDMAPort struct {
	Port       string
	State      string
	PhysState  string
	Rate       string
	LinkLayer  string
	LID        string
	SMLID      string
	Counters   map[string]uint64 // ports/N/counters/*
	HWCounters map[string]uint64 // ports/N/hw_counters/*
}

func readRDMA(sysfs string) ([]RDMADevice, error) {
	base := filepath.Join(sysfs, "class", "infiniband")
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, err
	}
	var out []RDMADevice
	for _, e := range entries {
		name := e.Name()
		devPath := filepath.Join(base, name)
		dev := RDMADevice{
			Name:     name,
			NodeType: readStr(devPath, "node_type"),
			FWVer:    readStr(devPath, "fw_ver"),
			HCAType:  readStr(devPath, "hca_type"),
			NodeGUID: readStr(devPath, "node_guid"),
			BoardID:  readStr(devPath, "board_id"),
		}
		portsDir := filepath.Join(devPath, "ports")
		ports, err := os.ReadDir(portsDir)
		if err != nil {
			out = append(out, dev)
			continue
		}
		for _, p := range ports {
			if !p.IsDir() {
				continue
			}
			pp := filepath.Join(portsDir, p.Name())
			port := RDMAPort{
				Port:       p.Name(),
				State:      readStr(pp, "state"),
				PhysState:  readStr(pp, "phys_state"),
				Rate:       readStr(pp, "rate"),
				LinkLayer:  readStr(pp, "link_layer"),
				LID:        readStr(pp, "lid"),
				SMLID:      readStr(pp, "sm_lid"),
				Counters:   readUintMap(filepath.Join(pp, "counters")),
				HWCounters: readUintMap(filepath.Join(pp, "hw_counters")),
			}
			dev.Ports = append(dev.Ports, port)
		}
		out = append(out, dev)
	}
	return out, nil
}

func readUintMap(dir string) map[string]uint64 {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	out := make(map[string]uint64, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		s := strings.TrimSpace(string(b))
		// Some counters are hex; accept both.
		v, err := strconv.ParseUint(s, 0, 64)
		if err != nil {
			continue
		}
		out[e.Name()] = v
	}
	return out
}
