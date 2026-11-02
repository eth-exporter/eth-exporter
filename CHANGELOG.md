# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
once tagged releases begin.

## [Unreleased]

### Added

- Initial public repository layout as **eth-exporter**
- Kernel-only Prometheus exporter (`ethtool_exporter` binary)
- Collectors: sysfs/procfs, ethtool ioctl + genetlink, rtnetlink, nftables, RDMA, conntrack, multicast, LLDP/CDP neighbors
- Adaptive neighbor capture with BPF filter, caching, and `-tags noneighbor` build
- systemd unit, Alloy example, web-config example
- Vendored Go module dependencies under `vendor/`
- Project docs: README, CONTRIBUTING, CODE_OF_CONDUCT, SECURITY, MIT-0 LICENSE
- `--version` flag and `ethtool_exporter_build_info` metric
- Prometheus example config with file SD, recording rules, alerting rules, and `promtool` rule tests
- Alloy fleet config (`alloy/fleet.alloy`) for a central scraper over TLS/basic auth
- Grafana dashboards: Overview, Physical layer & optics, Topology, Host network stack; provisioning files
- Dockerfile and single-host compose demo stack
- CI (Go tests, vet, gofmt, vendor check, monitoring asset validation, Docker build) and tag-driven release workflow (`make dist`)

### Changed

- `alloy/config.alloy` uses `constants.hostname` for `instance` and sets `job_name = "ethtool"`

### Fixed

- Grafana *Physical layer & optics* → **Driver statistics (ethtool -S)** failed on many-queue NICs (e.g. Broadcom BCM57608 / `bnxt_en` with 64 queues, AMD Pollara / `ionic` with 16) with backend limit errors (`query processing would load too many samples into memory`, `err-mimir-max-chunks-per-query`). The default error regex matched ~2,300 per-queue counters per host. A new **Driver stat exclude** variable (default: per-queue / per-ring names `[N]: …`, `rx_N_*`, `tx_N_*`, `*_queue_N_*`) is applied with `stat!~`, cutting the default panel to ~270 series per host; set it to `^$` to restore the old behaviour. The panel also caps `maxDataPoints` at 500.

[Unreleased]: https://github.com/eth-exporter/eth-exporter/commits/main
