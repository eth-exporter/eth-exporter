# Security policy

## Supported versions

Security fixes are applied to the latest release on `main`. If you are running an older build, please upgrade before reporting unless the issue is specifically about upgrade safety.

## What this software does

`ethtool_exporter` is a **privileged, read-oriented** Prometheus exporter. It opens netlink / packet sockets and reads sysfs/procfs. Misconfiguration (binding to a public interface without auth, excessive adjacency cardinality, or exposing neighbor TLVs) can leak network topology or create scrape load.

## Reporting a vulnerability

**Please do not open a public GitHub issue for security vulnerabilities.**

Prefer one of:

1. **GitHub Security Advisories** — use *Report a vulnerability* on the repository’s Security tab (private to maintainers).
2. **Private contact** — email the maintainer via the address listed on the GitHub profile associated with this repository, with subject prefix `[SECURITY] eth-exporter`.

Include as much of the following as you can:

- Description of the issue and impact
- Affected commit or release tag (`ethtool_exporter --version`)
- Proof of concept or reproduction steps (non-destructive preferred)
- Whether the issue is already public elsewhere
- Your preferred credit name (or anonymous)

You should receive an acknowledgement within **7 days**. We will work with you on a fix timeline and disclosure date.

## Scope

In scope examples:

- Remote code execution, privilege escalation, or sandbox escapes via the exporter
- Authentication / TLS bypass when `--web.config.file` is configured
- Unexpected packet injection or kernel write paths (the exporter should be read-only)
- Sensitive data disclosure beyond intended metrics (e.g. unbounded dumps, secrets in logs)
- Shipped deployment assets (systemd unit, Dockerfile, compose stack, Alloy/Prometheus/Grafana examples) that weaken a deployment when used as documented

Out of scope examples:

- Issues that require already-local root or equivalent capability outside the exporter
- Denial of service by scrapes when operators intentionally expose `/metrics` without rate limits or auth
- Missing features or collector accuracy bugs without security impact (use normal issues)
- Vulnerabilities only in third-party dependencies that do not affect this project’s usage — still appreciated; we will upgrade when practical

## Hardening guidance for operators

- Keep the default listen address `127.0.0.1:9417` or protect with TLS/auth via `--web.config.file`
- Run under the provided systemd unit (dedicated user, capability bounding set, resource limits)
- Disable neighbor capture (`--collector.neighbor=false` or `-tags noneighbor`) if LLDP/CDP is not needed
- Use `--collector.neighbor.redact` when neighbor identity labels must not leave the host
- Cap ARP/FDB series with `--collector.l3-neigh.max` / `--collector.fdb.max`
- Do not publish `/metrics` to untrusted networks

## Disclosure policy

We prefer coordinated disclosure: fix first, then public advisory or release notes. We will credit reporters who wish to be named unless they request otherwise.

## Safe testing

When testing on production hosts:

- Prefer scrape from localhost
- Avoid changing interface admin state, bonding, VLANs, or switch config
- Prefer read-only diagnostics (`ethtool --show-priv-flags`, `tcpdump` with filters) over write paths
