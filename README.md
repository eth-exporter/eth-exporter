# eth-exporter

[![License: MIT-0](https://img.shields.io/badge/License-MIT--0-lightgrey.svg)](LICENSE)

Prometheus / Grafana Alloy exporter for Linux network interfaces.

It reads **only from the kernel** — no `ethtool` CLI, `lldpd`, `ip`, SNMP, or other host packages are required at runtime. A single static Go binary talks to:

| Source | Data |
|--------|------|
| `/sys/class/net/*` | operstate, carrier, MTU, speed, MAC, phys_port_*, queues, bonding, bridge port, VF, PCI identity |
| `/sys/class/net/*/statistics/*` | Standard interface counters |
| `/proc/net/softnet_stat`, `snmp`, `netstat` | Softnet + IP/TCP/UDP stack counters |
| `/sys/class/infiniband` | RDMA/IB HCA + port counters |
| `/proc/sys/net/netfilter`, `/proc/net/stat/nf_conntrack` | Conntrack pressure + per-CPU stats |
| **nfnetlink nftables** | Tables, chains, named counter objects |
| `/proc/net/igmp`, `igmp6`, `dev_mcast` | IGMP/MLD + L2 multicast memberships |
| ethtool **ioctl** | Driver info, link, NIC/PHY stats, coalesce/rings/pause/channels, EEE/FEC/features/WoL, module EEPROM + DOM |
| ethtool **genetlink** | Standard IEEE/RMON groups, pause/FEC counters, linkstate (SQI/down reasons), linkmodes, privflags, RSS |
| **rtnetlink** | Addresses, permanent MAC, ARP/ND, bridge FDB, XDP attach, TC qdisc stats |
| **AF_PACKET** | Passive LLDP and CDP neighbor discovery |

The binary is named `ethtool_exporter` (Prometheus exporter convention). This repository is self-contained: Go module dependencies are **vendored** under [`vendor/`](vendor/).

## Requirements

- Linux (genetlink / AF_PACKET / ethtool UAPI); stub builds compile on other GOOS but collect nothing useful
- Go **1.22+** to build from source
- Runtime privileges: `CAP_NET_ADMIN` (ethtool / nftables), `CAP_NET_RAW` (LLDP/CDP) — see [systemd unit](systemd/ethtool_exporter.service)

## Quick start

```bash
git clone https://github.com/scottmo/eth-exporter.git
cd eth-exporter
make build          # uses -mod=vendor (offline-friendly)
./ethtool_exporter  # listens on 127.0.0.1:9417 by default
curl -sS http://127.0.0.1:9417/metrics | head
```

Cross-compile for Linux from macOS/Windows:

```bash
make cross          # ethtool_exporter-linux-amd64, -arm64, -amd64-noneighbor
```

Build without neighbor capture (no `CAP_NET_RAW` for discovery):

```bash
make build-noneighbor
# or: go build -mod=vendor -tags noneighbor -o ethtool_exporter ./cmd/ethtool_exporter
```

## Install (systemd)

```bash
useradd -r -s /usr/sbin/nologin -M ethtool_exporter
install -m 755 ethtool_exporter /usr/local/bin/
cp systemd/ethtool_exporter.service /etc/systemd/system/
# optional TLS/auth:
# install -d -m 750 /etc/ethtool_exporter
# install -m 640 -o root -g ethtool_exporter systemd/web-config.example.yml /etc/ethtool_exporter/web-config.yml
systemctl daemon-reload && systemctl enable --now ethtool_exporter
```

See [docs/INSTALL.md](docs/INSTALL.md) for containers, Alloy, and hardening details.

## Flags

| Flag | Default | Purpose |
|------|---------|---------|
| `--web.listen-address` | `127.0.0.1:9417` | Listen address (loopback by default) |
| `--web.config.file` | _(empty)_ | TLS / basic-auth ([Prometheus web config](https://prometheus.io/docs/guides/tls-encryption/)) |
| `--web.disable-landing-page` | `false` | Serve `/metrics` only |
| `--path.sysfs` | `/sys` | sysfs root |
| `--path.procfs` | `/proc` | procfs root |
| `--collector.device-include` | _(empty)_ | Interface include regexp |
| `--collector.device-exclude` | `^(lo\|docker.*\|…)$` | Interface exclude regexp |
| `--collector.ethtool.metrics-include` | `.*` | Filter driver stat names |
| `--collector.neighbor` | `true` | Enable LLDP/CDP capture (no-op if `-tags noneighbor`) |
| `--collector.neighbor.redact` | `false` | Omit free-text / management identity labels |
| `--collector.neighbor.stale` | `10m` | Cache neighbors at least this long (also respects PDU TTL) |
| `--collector.neighbor.active-hold` | `3m` | Keep continuous capture after last PDU |
| `--collector.neighbor.probe-window` | `35s` | Probe listen window (exceeds typical LLDP 30s interval) |
| `--collector.neighbor.probe-min` | `1m` | Min backoff between quiet-interface probes |
| `--collector.neighbor.probe-max` | `15m` | Max backoff when no LLDP/CDP is seen |
| `--collector.workers` | `4` | Parallel per-device scrape workers |
| `--collector.optics.cache` | `60s` | Cache module DOM / EEPROM reads |
| `--collector.neg-cache` | `5m` | Cache unsupported probe failures |
| `--collector.optics` / `--collector.std-stats` / … | `true` | Per-subsystem toggles (see `--help`) |

## Resource usage (observed)

On production Proxmox hosts with neighbor capture enabled and full collectors:

| State | RSS | CPU |
|-------|-----|-----|
| Idle (LLDP listeners warm) | ~14–16 MB | ~0.2–0.5% of one core |
| After full scrape | ~26–32 MB | — |
| Scrape wall time | — | ~1.4–1.5 s (warm CPU ~0.1–0.35 s/scrape) |

At a 15 s scrape interval, amortized CPU is roughly **~2% of one core** on a busy multi-NIC host. Lean profile (neighbor/optics/nftables off) scrapes in ~0.3 s.

## Performance notes

- Neighbor capture uses a **classic BPF filter** so only LLDP/CDP multicast destinations reach userspace.
- Interfaces that advertise neighbors stay in **continuous** capture (capped by `--collector.neighbor.max-continuous`); quiet ones are probed, then **exponentially back off**.
- Module EEPROM/DOM and nftables dumps are **cached**; ARP/FDB series are capped.
- Concurrent scrapes default to **1 in flight**.
- Watch `ethtool_scrape_stage_duration_seconds{stage=...}` for scrape cost.

Lean profile:

```bash
./ethtool_exporter \
  --collector.neighbor=false \
  --collector.optics=false \
  --collector.fdb=false \
  --collector.l3-neigh=false \
  --collector.nftables=false \
  --collector.link-modes=false \
  --collector.features=false
```

## Metric families

### Interface / device
- `ethtool_interface_*` — sysfs state, counters, carrier changes
- `ethtool_interface_perm_address_info` — permanent MAC
- `ethtool_device_info` / `ethtool_device_sriov_numvfs` — PCI/platform identity
- `ethtool_queue_count`, `ethtool_bond_*`, `ethtool_bridge_port_info`, `ethtool_vf_info`
- `ethtool_address_info` — IPv4/IPv6 addresses

### Ethtool driver / link
- `ethtool_driver_info`, `ethtool_nic_statistics`, `ethtool_phy_statistics`
- `ethtool_link_*`, `ethtool_linkstate_*`, `ethtool_linkmode_*`
- coalesce / ring / pause / channel / EEE / FEC / features / WoL
- `ethtool_privflag_active`, `ethtool_rss_*`

### Optics (DOM)
- `ethtool_module_dom_*` — temperature, voltage, bias, TX/RX power, channels

### Standard / pause / FEC
- `ethtool_std_statistics`, `ethtool_rmon_hist_packets`
- `ethtool_pause_frames_total`, `ethtool_fec_blocks_total`

### L2/L3 adjacency & TC
- `ethtool_neighbor_*` — LLDP/CDP
- `ethtool_l3_neighbor_info` — ARP/ND
- `ethtool_fdb_info` — bridge FDB
- `ethtool_xdp_attached`, `ethtool_qdisc_*`

### Host stack / RDMA / firewall / multicast
- `ethtool_softnet_stat`, `ethtool_stack_stat`
- `ethtool_rdma_*`
- `ethtool_conntrack_*`, `ethtool_nftables_*`
- `ethtool_igmp_group_info`, `ethtool_mld_group_info`, `ethtool_device_mcast_info`

Unsupported features are omitted per interface (not scraped as zeros).

## Platform notes

- **Intel i40e**: firmware LLDP absorbs PDUs before the host stack. To receive neighbors: `ethtool --set-priv-flags <iface> disable-fw-lldp on` (safe for LACP/data plane; not persistent across reboot unless scripted). Confirm with `tcpdump -i <iface> ether proto 0x88cc`.
- **genetlink std/pause/FEC**: requires family `ethtool` (`genl ctrl get name ethtool`). Some drivers (e.g. i40e) return empty IEEE/RMON groups even when genetlink works; driver `-S` stats still come from ioctl/sysfs.

## Repository layout

```
cmd/ethtool_exporter/   # main binary
internal/               # collector, ethtool, neighbor, netlink, netfilter
vendor/                 # Go module dependencies (committed)
systemd/                # unit file, drop-in, web-config example
alloy/                  # Grafana Alloy scrape example
docs/                   # install / design notes
```

## Documentation

| Doc | Topic |
|-----|--------|
| [docs/INSTALL.md](docs/INSTALL.md) | Install, containers, Alloy |
| [CONTRIBUTING.md](CONTRIBUTING.md) | How to contribute |
| [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) | Community standards |
| [SECURITY.md](SECURITY.md) | Vulnerability reporting |
| [CHANGELOG.md](CHANGELOG.md) | Release history |
| [NOTICE](NOTICE) | Third-party notices |
| [LICENSE](LICENSE) | MIT-0 (MIT No Attribution) |

## Security hardening

Defaults and the unit file aim for least exposure:

| Control | Recommendation |
|---------|----------------|
| Listen address | `127.0.0.1:9417` |
| TLS / auth | `--web.config.file` ([example](systemd/web-config.example.yml)) |
| Capabilities | `CAP_NET_ADMIN` + `CAP_NET_RAW` only; drop RAW if neighbor off |
| Neighbor identity | `--collector.neighbor.redact` |
| Scrape DoS surface | HTTP timeouts; `max-requests-in-flight=1`; systemd `MemoryMax` |
| Cardinality | `--collector.l3-neigh.max` / `--collector.fdb.max` |

Do **not** expose `/metrics` on a public network without TLS and authentication.

## License

[MIT No Attribution (MIT-0)](LICENSE). Third-party Go modules retain their own licenses; see [`vendor/`](vendor/) and [NOTICE](NOTICE).

Uses Linux ethtool UAPI concepts (`uapi/linux/ethtool.h`, genetlink) without linking the upstream ethtool C binary.
