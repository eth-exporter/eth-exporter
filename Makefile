GO      ?= go
BIN     ?= ethtool_exporter
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
TAGS    ?=
# Prefer vendored modules so builds work offline from this tree alone.
MODFLAGS ?= -mod=vendor

.PHONY: build build-noneighbor test cross tidy vendor clean

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

test:
	$(GO) test $(MODFLAGS) ./...
	$(GO) test $(MODFLAGS) -tags noneighbor ./...

tidy:
	$(GO) mod tidy

vendor: tidy
	$(GO) mod vendor

clean:
	rm -f $(BIN) $(BIN)-noneighbor $(BIN)-linux-amd64 $(BIN)-linux-arm64 $(BIN)-linux-amd64-noneighbor
