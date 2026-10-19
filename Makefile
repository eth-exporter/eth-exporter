GO      ?= go
BIN     ?= ethtool_exporter
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
TAGS    ?=
# Prefer vendored modules so builds work offline from this tree alone.
MODFLAGS ?= -mod=vendor
DIST    ?= dist
PLATFORMS ?= linux/amd64 linux/arm64
PROMTOOL ?= promtool
ALLOY    ?= alloy

.PHONY: build build-noneighbor test cross dist check-assets tidy vendor clean

build:
	CGO_ENABLED=0 $(GO) build $(MODFLAGS) -tags '$(TAGS)' -ldflags "-s -w -X main.version=$(VERSION)" -o $(BIN) ./cmd/ethtool_exporter

# Omit AF_PACKET LLDP/CDP capture (no CAP_NET_RAW needed for discovery).
build-noneighbor:
	$(MAKE) build TAGS=noneighbor BIN=$(BIN)-noneighbor

# Static Linux binaries (usable from macOS/Windows hosts).
cross:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(MODFLAGS) -ldflags "-s -w -X main.version=$(VERSION)" -o $(BIN)-linux-amd64 ./cmd/ethtool_exporter
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build $(MODFLAGS) -ldflags "-s -w -X main.version=$(VERSION)" -o $(BIN)-linux-arm64 ./cmd/ethtool_exporter
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(MODFLAGS) -tags noneighbor -ldflags "-s -w -X main.version=$(VERSION)" -o $(BIN)-linux-amd64-noneighbor ./cmd/ethtool_exporter

# Release tarballs (binary + docs + systemd/alloy/prometheus/grafana assets) and sha256sums.txt.
dist:
	rm -rf $(DIST) && mkdir -p $(DIST)
	@set -e; for p in $(PLATFORMS); do \
	  os=$${p%/*}; arch=$${p#*/}; \
	  for tags in "" noneighbor; do \
	    name=$(BIN)-$(VERSION).$$os-$$arch$${tags:+-$$tags}; \
	    echo "building $$name"; \
	    mkdir -p $(DIST)/$$name; \
	    CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch $(GO) build $(MODFLAGS) -trimpath -tags "$$tags" \
	      -ldflags "-s -w -X main.version=$(VERSION)" -o $(DIST)/$$name/$(BIN) ./cmd/ethtool_exporter; \
	    cp -R LICENSE NOTICE README.md CHANGELOG.md systemd alloy prometheus grafana $(DIST)/$$name/; \
	    tar -C $(DIST) -czf $(DIST)/$$name.tar.gz $$name; \
	    rm -rf $(DIST)/$$name; \
	  done; \
	done
	cd $(DIST) && shasum -a 256 *.tar.gz > sha256sums.txt

# Validate the monitoring examples (needs promtool, alloy, and jq on PATH).
check-assets:
	$(PROMTOOL) check config prometheus/prometheus.yml
	$(PROMTOOL) check rules prometheus/rules/*.yml
	$(PROMTOOL) test rules prometheus/rules/tests/*.yml
	$(ALLOY) validate alloy/config.alloy
	$(ALLOY) validate alloy/fleet.alloy
	@for f in alloy/*.alloy; do $(ALLOY) fmt $$f | diff -u $$f - || { echo "$$f: run 'alloy fmt -w $$f'"; exit 1; }; done
	@for f in grafana/dashboards/*.json; do jq -e '.uid and .title and (.panels | length > 0)' $$f >/dev/null || { echo "$$f: invalid dashboard"; exit 1; }; done
	@test "$$(jq -r .uid grafana/dashboards/*.json | sort | uniq -d)" = "" || { echo "duplicate dashboard uid"; exit 1; }
	@echo "monitoring assets OK"

test:
	$(GO) test $(MODFLAGS) ./...
	$(GO) test $(MODFLAGS) -tags noneighbor ./...

tidy:
	$(GO) mod tidy

vendor: tidy
	$(GO) mod vendor

clean:
	rm -rf $(DIST)
	rm -f $(BIN) $(BIN)-noneighbor $(BIN)-linux-amd64 $(BIN)-linux-arm64 $(BIN)-linux-amd64-noneighbor
