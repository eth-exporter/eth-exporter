# Support

## Getting help

- **Bugs / unexpected metrics** — open a [GitHub issue](https://github.com/scottmo/eth-exporter/issues) with kernel version, driver, flags, and what you expected vs observed.
- **How do I…?** — check [README.md](README.md) and [docs/INSTALL.md](docs/INSTALL.md) first; then open a discussion or issue.
- **Security** — follow [SECURITY.md](SECURITY.md); do not file public issues for vulnerabilities.
- **Community standards** — [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).

## What we can help with

- Build / install / systemd / Alloy scrape setup
- Interpreting metric families and collector flags
- Privilege and hardening questions
- Reproducible collector bugs

## What we cannot guarantee

- Fixes for proprietary NIC firmware quirks beyond documented workarounds (e.g. i40e `disable-fw-lldp`)
- Support for kernels or drivers that omit genetlink / standard stats groups
- Production SLA or paid support (this is an open-source project under MIT-0)

## Maintainer response

Best-effort. Pull requests with tests and clear scope are the fastest path to a fix.
