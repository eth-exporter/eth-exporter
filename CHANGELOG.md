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

[Unreleased]: https://github.com/scottmo/eth-exporter/compare/HEAD...HEAD
