# Installation guide

## Build from source

```bash
git clone https://github.com/scottmo/eth-exporter.git
cd eth-exporter
make build                 # ./ethtool_exporter
# or offline / CI-friendly:
go build -mod=vendor -o ethtool_exporter ./cmd/ethtool_exporter
```

Cross-compile Linux binaries from another OS:

```bash
make cross
# produces:
#   ethtool_exporter-linux-amd64
#   ethtool_exporter-linux-arm64
#   ethtool_exporter-linux-amd64-noneighbor
```

## systemd (recommended)

1. Create a dedicated user: `useradd -r -s /usr/sbin/nologin -M ethtool_exporter`
2. Install the binary to `/usr/local/bin/ethtool_exporter`
3. Install [`systemd/ethtool_exporter.service`](../systemd/ethtool_exporter.service)
4. Optional: drop neighbor capability with [`systemd/ethtool_exporter-noneighbor.conf`](../systemd/ethtool_exporter-noneighbor.conf)
5. Optional TLS/basic-auth: copy [`systemd/web-config.example.yml`](../systemd/web-config.example.yml) to `/etc/ethtool_exporter/web-config.yml` (mode `640`, group `ethtool_exporter`) and pass `--web.config.file=...`

```bash
systemctl daemon-reload
systemctl enable --now ethtool_exporter
systemctl status ethtool_exporter
curl -sS http://127.0.0.1:9417/metrics | head
```

## Containers

Prefer **host network** so interface names and LLDP/CDP match the host:

```bash
docker run --rm --network host --cap-add NET_ADMIN --cap-add NET_RAW \
  -v /sys:/sys:ro -v /proc:/proc:ro \
  your-registry/eth-exporter:latest
```

Notes:

- Bind-mount host `/sys` and `/proc` (read-only is fine).
- Drop `NET_RAW` and use `-tags noneighbor` / `--collector.neighbor=false` if discovery is unused.
- Keep the process listening on localhost inside the host network namespace, and scrape from a local Alloy/Prometheus sidecar or the host agent.

## Grafana Alloy

Example config: [`alloy/config.alloy`](../alloy/config.alloy). Typical pattern: Alloy on the same host scrapes `127.0.0.1:9417` and forwards to your metrics backend.

## Intel i40e LLDP

If neighbor metrics stay empty on i40e while the switch transmits LLDP, firmware may be consuming PDUs:

```bash
ethtool --show-priv-flags nic4 | grep disable-fw-lldp
ethtool --set-priv-flags nic4 disable-fw-lldp on
tcpdump -i nic4 -c 1 ether proto 0x88cc
```

Persist the priv-flag via your interface bring-up scripts if needed across reboots. This does not disable LACP or data-plane traffic.

## Hardening checklist

- [ ] Listen on loopback or private management network only
- [ ] Enable `--web.config.file` if scrape traffic leaves the host
- [ ] Cap adjacency series (`l3-neigh.max`, `fdb.max`)
- [ ] Disable neighbor capture when unused
- [ ] Use systemd resource limits from the shipped unit
- [ ] Confirm scrapes do not change link/bond/VLAN state (exporter is read-only by design)
