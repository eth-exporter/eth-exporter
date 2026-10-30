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

### Fixed

- Exporter no longer panics (`label value "..." is not valid UTF-8`) when a NIC, module EEPROM, or LLDP/CDP peer reports non-UTF-8 bytes (seen on AMD Pollara 400G QSFP112 modules). Free-form label values (module vendor/part/serial, driver info, interface alias/phys IDs, neighbor TLVs) are trimmed of NUL/space padding; values that are still not printable UTF-8 are emitted hex-encoded as `0x...` instead of crashing.
- Duplicate LLDP/CDP neighbor entries (same device, protocol, chassis ID and port ID after label normalisation) are emitted once instead of failing the whole scrape with "collected before with the same name and label values".

### Changed

- `alloy/config.alloy` uses `constants.hostname` for `instance` and sets `job_name = "ethtool"`

[Unreleased]: https://github.com/eth-exporter/eth-exporter/commits/main
