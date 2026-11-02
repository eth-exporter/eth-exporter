<p align="center">
  <img src="docs/images/eth-exporter.png" alt="eth-exporter logo" width="200">
</p>

# eth-exporter

[![CI](https://github.com/eth-exporter/eth-exporter/actions/workflows/ci.yml/badge.svg)](https://github.com/eth-exporter/eth-exporter/actions/workflows/ci.yml)
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

The binary is named `ethtool_exporter` (Prometheus exporter convention). This repository is self-contained: Go module dependencies are **vendored** under [`vendor/`](vendor/), and it ships everything needed to monitor a fleet:

| Asset | Path |
|-------|------|
| Prometheus scrape config, recording rules, alerts (with unit tests) | [`prometheus/`](prometheus/) |
| Grafana Alloy configs (per-host agent and central fleet scraper) | [`alloy/`](alloy/) |
| Grafana dashboards (JSON) and provisioning | [`grafana/`](grafana/) |
| systemd unit, no-neighbor drop-in, TLS/auth example | [`systemd/`](systemd/) |
| Dockerfile and single-host demo stack | [`Dockerfile`](Dockerfile), [`compose/`](compose/) |

## Requirements

- Linux (genetlink / AF_PACKET / ethtool UAPI); stub builds compile on other GOOS but collect nothing useful
- Go **1.22+** to build from source
- Runtime privileges: `CAP_NET_ADMIN` (ethtool / nftables), `CAP_NET_RAW` (LLDP/CDP) — see [systemd unit](systemd/ethtool_exporter.service)

## Quick start

Download a release archive from [GitHub Releases](https://github.com/eth-exporter/eth-exporter/releases) (static Linux `amd64` / `arm64` binaries, with and without neighbor capture, plus all configs and dashboards), or build from source:

```bash
git clone https://github.com/eth-exporter/eth-exporter.git
cd eth-exporter
make build          # uses -mod=vendor (offline-friendly)
./ethtool_exporter  # listens on 127.0.0.1:9417 by default
curl -sS http://127.0.0.1:9417/metrics | head
./ethtool_exporter --version
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

## Monitoring setup

### Prometheus

[`prometheus/prometheus.yml`](prometheus/prometheus.yml) scrapes the local exporter, picks up additional hosts from [`prometheus/targets/`](prometheus/targets/ethtool.yml.example) (file SD), strips `:9417` from `instance`, and loads:

- [`rules/ethtool.rules.yml`](prometheus/rules/ethtool.rules.yml) — recording rules for throughput, utilization, errors, drops, FEC, softnet, conntrack, and TCP retransmits
- [`rules/ethtool.alerts.yml`](prometheus/rules/ethtool.alerts.yml) — alerts for exporter health, interface down / flapping / errors / overruns / saturation, bond members, FEC, optical RX power and temperature, conntrack, softnet drops, and TCP retransmits

```bash
promtool check config prometheus/prometheus.yml
promtool test rules prometheus/rules/tests/*.yml
```

### Grafana Alloy

- [`alloy/config.alloy`](alloy/config.alloy) — Alloy on each host scrapes `127.0.0.1:9417` and `remote_write`s to Prometheus / Mimir / Grafana Cloud (recommended; the exporter stays loopback-only).
- [`alloy/fleet.alloy`](alloy/fleet.alloy) — one central Alloy scrapes many hosts over HTTPS with basic auth, using the same file-SD targets format.

Both set `job="ethtool"` and a hostname `instance`, matching the dashboards and alerts. Load the rule files into your Mimir / Prometheus ruler (for example `mimirtool rules load prometheus/rules/*.yml`).

### Grafana dashboards

Import the JSON files from [`grafana/dashboards/`](grafana/dashboards/) (**Dashboards → New → Import**, then pick your Prometheus data source), or provision them with [`grafana/provisioning/`](grafana/provisioning/). Each dashboard has `Data source`, `Job`, `Instance`, and `Interface` variables and links to the others.

| Dashboard | Contents |
|-----------|----------|
| [Overview](grafana/dashboards/ethtool-overview.json) | Targets, interface state, throughput, packets, utilization, errors, drops, carrier changes, interface and NIC inventory, exporter health |
| [Physical layer & optics](grafana/dashboards/ethtool-physical.json) | Link settings, link-down events, transceiver DOM (power, temperature, bias, voltage, per lane), FEC, pause, IEEE/RMON counters, `ethtool -S` explorer, rings / channels / coalescing |
| [Topology](grafana/dashboards/ethtool-topology.json) | LLDP/CDP neighbors, bonds, bridge ports, SR-IOV VFs, IP addresses, multicast, ARP/ND and FDB counts |
| [Host network stack](grafana/dashboards/ethtool-host-stack.json) | Softnet per CPU, TCP/UDP/IP counters, conntrack, qdisc, nftables counters, RDMA/InfiniBand |

### Demo stack

On a Linux host, [`compose/docker-compose.yml`](compose/docker-compose.yml) runs the exporter, Prometheus, and Grafana (all on `127.0.0.1`) with the shipped configs and dashboards:

```bash
cd compose && docker compose up -d --build   # Grafana on http://127.0.0.1:3000
```

## Flags

| Flag | Default | Purpose |
|------|---------|---------|
| `--web.listen-address` | `127.0.0.1:9417` | Listen address (loopback by default) |
| `--web.telemetry-path` | `/metrics` | Metrics path |
| `--web.config.file` | _(empty)_ | TLS / basic-auth ([Prometheus web config](https://prometheus.io/docs/guides/tls-encryption/)) |
| `--web.disable-landing-page` | `false` | Serve `/metrics` only |
| `--web.max-requests-in-flight` | `1` | Concurrent scrapes (0 = unlimited) |
| `--path.sysfs` | `/sys` | sysfs root |
| `--path.procfs` | `/proc` | procfs root |
| `--collector.device-include` | _(empty)_ | Interface include regexp (exclusive with exclude) |
| `--collector.device-exclude` | `^(lo\|docker.*\|veth.*\|br-.*\|virbr.*)$` | Interface exclude regexp |
| `--collector.ethtool.metrics-include` | `.*` | Filter driver stat names |
| `--collector.neighbor` | `true` | Enable LLDP/CDP capture (no-op if `-tags noneighbor`) |
| `--collector.neighbor.redact` | `false` | Omit free-text / management identity labels |
| `--collector.neighbor.stale` | `10m` | Cache neighbors at least this long (also respects PDU TTL) |
| `--collector.neighbor.active-hold` | `3m` | Keep continuous capture after last PDU |
| `--collector.neighbor.probe-window` | `35s` | Probe listen window (exceeds typical LLDP 30s interval) |
| `--collector.neighbor.probe-min` | `1m` | Min backoff between quiet-interface probes |
| `--collector.neighbor.probe-max` | `15m` | Max backoff when no LLDP/CDP is seen |
| `--collector.neighbor.max-continuous` | `16` | Max simultaneous continuous AF_PACKET listeners |
| `--collector.workers` | `4` | Parallel per-device scrape workers |
| `--collector.optics.cache` | `60s` | Cache module DOM / EEPROM reads |
| `--collector.nftables.cache` | `15s` | Cache nftables dumps across scrapes |
| `--collector.neg-cache` | `5m` | Cache unsupported probe failures |
| `--collector.l3-neigh.max` | `4096` | Max ARP/ND series per scrape (-1 unlimited) |
| `--collector.fdb.max` | `8192` | Max FDB series per scrape (-1 unlimited) |
| `--log.level` | `info` | `debug`, `info`, `warn`, `error` |
| `--version` | | Print version and exit |

Per-subsystem toggles, all default `true`: `--collector.optics`, `.std-stats`, `.link-modes`, `.features`, `.priv-flags`, `.rss`, `.pause-fec-stats`, `.addresses`, `.l3-neigh`, `.fdb`, `.qdisc`, `.nftables`, `.rdma`, `.conntrack`, `.multicast`, `.softnet`, `.stack`.

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
- `ethtool_module_dom_info`, `ethtool_module_temperature_celsius`, `ethtool_module_voltage_volts`, `ethtool_module_bias_milliamps`, `ethtool_module_tx_power_dbm`, `ethtool_module_rx_power_dbm`
- `ethtool_module_channel{lane, metric}` — per-lane `bias_mA`, `tx_power_dBm`, `rx_power_dBm` (+ linear `tx_power_mW` / `rx_power_mW` for CMIS modules)
- Decoders: SFF-8472 (SFP), SFF-8636/8436 (QSFP/QSFP28), and **CMIS** (QSFP-DD, OSFP, QSFP112, …), selected by the SFF-8024 identifier byte rather than the driver-reported module type. CMIS lane monitors (page 11h) and the bias multiplier (page 01h) are read with `ETHTOOL_MSG_MODULE_EEPROM_GET` (kernel ≥ 5.13); without it only module temperature/voltage/identity are reported.

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

### Exporter
- `ethtool_scrape_success`, `ethtool_scrape_duration_seconds`, `ethtool_scrape_stage_duration_seconds`
- `ethtool_exporter_build_info{version, goversion}`
- Standard `go_*` and `process_*` metrics

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
prometheus/             # scrape config, file-SD targets, recording + alerting rules, rule tests
alloy/                  # Grafana Alloy: per-host agent and central fleet configs
grafana/                # dashboards (JSON) and provisioning
compose/                # single-host demo stack (exporter + Prometheus + Grafana)
docs/                   # install / design notes
```

`make check-assets` validates the Prometheus, Alloy, and Grafana files (needs `promtool`, `alloy`, and `jq`); CI runs it on every push.

## Documentation

| Doc | Topic |
|-----|--------|
| [docs/INSTALL.md](docs/INSTALL.md) | Install, containers, Prometheus, Alloy, Grafana |
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
