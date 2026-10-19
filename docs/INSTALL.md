# Installation guide

## Release archives

Each [GitHub release](https://github.com/eth-exporter/eth-exporter/releases) has `ethtool_exporter-<version>.linux-<arch>[-noneighbor].tar.gz` archives with a static binary plus the `systemd/`, `prometheus/`, `alloy/`, and `grafana/` directories, and a `sha256sums.txt`:

```bash
sha256sum -c --ignore-missing sha256sums.txt
tar xzf ethtool_exporter-<version>.linux-amd64.tar.gz
install -m 755 ethtool_exporter-<version>.linux-amd64/ethtool_exporter /usr/local/bin/
```

`make dist` builds the same archives locally into `dist/`.

## Build from source

```bash
git clone https://github.com/eth-exporter/eth-exporter.git
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

Build the image from the [`Dockerfile`](../Dockerfile) (static binary on `distroless/static`):

```bash
docker build -t eth-exporter .
docker build --build-arg TAGS=noneighbor -t eth-exporter:noneighbor .
```

Run it in the **host network namespace** so interface names, `/proc/net`, `/sys/class/net`, and LLDP/CDP match the host:

```bash
docker run -d --name ethtool_exporter --restart unless-stopped \
  --network host \
  --cap-drop ALL --cap-add NET_ADMIN --cap-add NET_RAW \
  --read-only --security-opt no-new-privileges \
  eth-exporter
```

Notes:

- With `--network host`, the container's own `/proc/net` and `/sys/class/net` already reflect the host network namespace; no bind mounts are needed.
- The process runs as uid 0 inside the container because Docker only grants `--cap-add` capabilities to root; `--cap-drop ALL` limits it to the two capabilities above.
- Drop `NET_RAW` and use the `noneighbor` image or `--collector.neighbor=false` if discovery is unused.
- The exporter keeps listening on `127.0.0.1:9417` in the host namespace; scrape it from a local Alloy/Prometheus.
- [`compose/docker-compose.yml`](../compose/docker-compose.yml) runs the exporter with Prometheus and Grafana as a single-host demo.

## Prometheus

[`prometheus/prometheus.yml`](../prometheus/prometheus.yml) is a complete example:

- Scrapes `127.0.0.1:9417` as `job="ethtool"` every 15 s (10 s timeout)
- Discovers more hosts from `prometheus/targets/*.yml` (copy [`ethtool.yml.example`](../prometheus/targets/ethtool.yml.example))
- Rewrites `instance` from `host:9417` to `host`
- Loads [recording rules](../prometheus/rules/ethtool.rules.yml) and [alerts](../prometheus/rules/ethtool.alerts.yml)
- Shows (commented) how to scrape over HTTPS with basic auth and how to drop optional high-cardinality families

Copy the directory to `/etc/prometheus/` or merge the `ethtool` job and `rule_files` into your existing config, then:

```bash
promtool check config /etc/prometheus/prometheus.yml
promtool test rules prometheus/rules/tests/*.yml   # from the repo checkout
```

Alert thresholds (optical power, temperature, error ratios) are generic starting points; tune them per site and module type.

## Grafana Alloy

- [`alloy/config.alloy`](../alloy/config.alloy): Alloy on the same host scrapes `127.0.0.1:9417`, labels series with `job="ethtool"` and `instance=<hostname>`, and `remote_write`s to your backend. This is the recommended layout because the exporter never has to listen off-host.
- [`alloy/fleet.alloy`](../alloy/fleet.alloy): a central Alloy discovers hosts from file-SD target files and scrapes them over HTTPS with basic auth. Each exporter then needs `--web.listen-address=<mgmt-ip>:9417` and `--web.config.file`.

Use one or the other per Alloy instance (both define `prometheus.remote_write "default"`). Validate with `alloy validate <file>`.

Alloy does not evaluate Prometheus rules itself; load [`prometheus/rules/`](../prometheus/rules/) into your Mimir or Prometheus ruler, e.g. `mimirtool rules load prometheus/rules/*.yml`.

## Grafana dashboards

Four dashboards live in [`grafana/dashboards/`](../grafana/dashboards/): Overview, Physical layer & optics, Topology, and Host network stack. Each uses a `datasource` variable, so they work with any Prometheus-compatible data source.

**Import:** Dashboards → New → Import → upload the JSON → choose the data source.

**Provision:**

```bash
cp grafana/provisioning/datasources/prometheus.yml /etc/grafana/provisioning/datasources/eth-exporter.yml  # edit url
cp grafana/provisioning/dashboards/eth-exporter.yml /etc/grafana/provisioning/dashboards/
install -d /var/lib/grafana/dashboards/eth-exporter
cp grafana/dashboards/*.json /var/lib/grafana/dashboards/eth-exporter/
systemctl restart grafana-server
```

Set the data source `timeInterval` (scrape interval) to match Prometheus so `$__rate_interval` spans enough samples. Panels for collectors that a host does not support (for example optics without DOM-capable modules, or RDMA) show "No data" rather than zeros.

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
