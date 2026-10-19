# syntax=docker/dockerfile:1
#
#   docker build -t eth-exporter .
#   docker build --build-arg TAGS=noneighbor -t eth-exporter:noneighbor .
#
# Run with the host network namespace so interface names, /proc/net, and
# LLDP/CDP match the host (see docs/INSTALL.md):
#
#   docker run -d --name ethtool_exporter --network host \
#     --cap-drop ALL --cap-add NET_ADMIN --cap-add NET_RAW \
#     --read-only --security-opt no-new-privileges eth-exporter

ARG GO_VERSION=1

FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src
COPY . .
ARG VERSION=dev
ARG TAGS=
RUN CGO_ENABLED=0 go build -mod=vendor -trimpath -tags "${TAGS}" \
      -ldflags "-s -w -X main.version=${VERSION}" \
      -o /out/ethtool_exporter ./cmd/ethtool_exporter

# Runs as root inside the container: Docker only grants --cap-add capabilities
# to uid 0. Drop everything else with --cap-drop ALL as shown above.
FROM gcr.io/distroless/static-debian12
LABEL org.opencontainers.image.title="eth-exporter" \
      org.opencontainers.image.description="Kernel-only Prometheus exporter for Linux network interfaces" \
      org.opencontainers.image.source="https://github.com/eth-exporter/eth-exporter" \
      org.opencontainers.image.licenses="MIT-0"
COPY --from=build /out/ethtool_exporter /usr/local/bin/ethtool_exporter
EXPOSE 9417
ENTRYPOINT ["/usr/local/bin/ethtool_exporter"]
