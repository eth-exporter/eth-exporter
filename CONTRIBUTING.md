# Contributing to eth-exporter

Thanks for your interest in improving eth-exporter. This document describes how to work on the project and submit changes.

## Code of conduct

Participation is governed by our [Code of Conduct](CODE_OF_CONDUCT.md). By contributing, you agree to uphold it.

## Ways to contribute

- Bug reports and reproducible test cases
- Documentation improvements
- New collectors or metrics that come from **kernel-only** sources (sysfs, procfs, ioctl, genetlink, rtnetlink, AF_PACKET)
- Performance, safety, and cardinality improvements
- Tests (unit tests preferred; no dependency on privileged live NICs in CI)

Out of scope for this project:

- Shelling out to `ethtool`, `ip`, `lldpd`, SNMP agents, or other host packages at runtime
- Features that require writing kernel config or disrupting live data-plane settings by default

## Development setup

```bash
git clone https://github.com/scottmo/eth-exporter.git
cd eth-exporter
# Dependencies are vendored; offline builds work with -mod=vendor
make test
make build
```

Requirements: Go 1.22+, a POSIX shell, and `make` (optional; plain `go` commands work).

### Updating dependencies

```bash
go get example.com/module@vX.Y.Z
make vendor   # go mod tidy && go mod vendor
git add go.mod go.sum vendor/
```

Commit `vendor/` whenever `go.mod` / `go.sum` change so the tree stays self-contained.

### Build tags

| Tag | Effect |
|-----|--------|
| _(none)_ | Full Linux collectors including AF_PACKET neighbor capture |
| `noneighbor` | Omit LLDP/CDP capture (`make build-noneighbor`) |

Always run tests for both configurations:

```bash
make test
```

## Coding guidelines

- Prefer small, focused changes with clear commit messages.
- Match existing style in `internal/` (stdlib + prometheus client; minimal new deps).
- Collectors must fail soft: unsupported ioctls/netlink ops omit metrics, they do not fail the scrape.
- Avoid high-cardinality labels (unbounded MACs, full FDB dumps without caps).
- Do not log neighbor identity or management addresses at info level by default.
- Keep the default listen address on loopback; any change to bind defaults needs strong justification.
- Add or update unit tests for parsers and pure logic (`internal/neighbor`, `internal/ethtool`, etc.).

## Pull requests

1. Fork and create a topic branch from `main`.
2. Make your change with tests where practical.
3. Ensure `make test` and `make build` succeed.
4. Open a PR with:
   - **What** changed and **why**
   - Any security / privilege / cardinality impact
   - How you tested (unit, VM, real host)

Maintainers may ask for smaller PRs if a change mixes unrelated refactors.

## Reporting bugs

Open a GitHub issue including:

- Kernel version (`uname -r`) and NIC driver (`ethtool -i <iface>` or sysfs)
- Exporter version / commit (`ethtool_exporter -version` if available, else `git describe`)
- Relevant flags
- Unexpected metrics, logs, or resource use
- Confirmation the host stayed healthy (no intentional data-plane changes)

**Do not** attach packet captures that contain credentials or private topology unless you redact them. Security-sensitive issues belong in [SECURITY.md](SECURITY.md), not public issues.

## Security

See [SECURITY.md](SECURITY.md) for private vulnerability reporting. Do not open public issues for exploitable flaws.

## License

By contributing, you agree that your contributions are licensed under the [MIT No Attribution (MIT-0)](LICENSE) license of this repository.
