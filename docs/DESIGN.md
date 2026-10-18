# Design notes

## Goals

1. **Kernel-only** — no dependency on the ethtool CLI, iproute2, lldpd, or SNMP at runtime.
2. **Safe defaults** — loopback listen, one scrape in flight, soft-fail per collector, adjacency caps.
3. **Useful on real servers** — bonding, optics DOM, LLDP neighbors, qdisc, nftables, RDMA.
4. **Self-contained tree** — Go modules vendored so `git clone` + `make build` works offline.

## Architecture

```
HTTP /metrics
    └── collector.Collector
            ├── per-device workers (ioctl / genetlink / sysfs)
            ├── host extras (rtnetlink, nft, procfs, RDMA, …)
            └── neighbor.Store  ←  neighbor.Capture (AF_PACKET, adaptive)
```

- **ioctl client** — classic `SIOCETHTOOL` for driver info, stats, coalesce, rings, module EEPROM, etc.
- **genetlink client** — `ethtool` family for standard stats, pause/FEC, linkstate, linkmodes, RSS, privflags when the kernel supports them.
- **rtnetlink** — addresses, ARP/ND, FDB, qdisc, XDP attach mode; payloads copied out of reused recv buffers; dumps wait for `NLMSG_DONE`.
- **neighbor** — BPF-filtered AF_PACKET; probe window defaults above typical LLDP 30s; continuous listen after a hit; compile out with `-tags noneighbor`.

## Non-goals

- Configuring interfaces or switches
- Replacing full network inventory platforms
- Emitting zeros for unsupported features (omit instead)

## Cardinality

High-cardinality series (FDB, ARP/ND, neighbor TLVs) are optional and capped. Prefer `--collector.neighbor.redact` when shipping metrics off-box.
