PKG := github.com/BoltzExchange/boltz-client/v2
VERSION := 2.11.3
GO_VERSION := 1.24.7-bookworm
RUST_VERSION := $(shell awk -F'"' '/^channel = / {print $$2}' rust-toolchain.toml)

PKG_BOLTZD := $(PKG)/cmd/boltzd
PKG_BOLTZ_CLI := $(PKG)/cmd/boltzcli

GO_BIN := ${GOPATH}/bin

GOTEST := CGO_ENABLED=1 GO111MODULE=on go test -v -timeout 5m
GOBUILD := CGO_ENABLED=1 GO111MODULE=on go build -v
GORUN := CGO_ENABLED=1 GO111MODULE=on go run -v
GOINSTALL := CGO_ENABLED=1 GO111MODULE=on go install -v

COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null)
LDFLAGS := -ldflags "-X $(PKG)/internal/build.Commit=$(COMMIT) -X $(PKG)/internal/build.Version=$(VERSION) -w -s"

GREEN := "\\033[0;32m"
NC := "\\033[0m"

define print
	echo $(GREEN)$1$(NC)
endef

default: build

#
# Dependencies
#

$(TOOLS_PATH):
	eval export PATH="$PATH:$(go env GOPATH)/bin"

release:
	git commit -a -m "chore: bump version to v$(VERSION)"
	git tag -s v$(VERSION) -m "v$(VERSION)"
	make changelog
	git commit -a -m "chore: update changelog"

install-tools: $(TOOLS_PATH)
	@$(call print, "Installing tools")
	cat tools/tools.go | grep _ | awk -F'"' '{print $$2}' | xargs -tI % go install %

proto: $(TOOLS_PATH)
	@$(call print, "Generating protosbufs")
	eval cd pkg/boltzrpc && ./gen_protos.sh

cln-proto: $(TOOLS_PATH)
	@$(call print, "Generating CLN protobufs")
	eval cd internal/cln/protos && ./gen_protos.sh

BINDGEN_GO_REPO := https://github.com/NordSecurity/uniffi-bindgen-go
BINDGEN_GO_TAG := v0.5.0+v0.29.5

bindgen-go:
	which uniffi-bindgen-go || cargo install uniffi-bindgen-go --git $(BINDGEN_GO_REPO) --tag $(BINDGEN_GO_TAG)

lwk-bindings: build-lwk bindgen-go
	cd lwk/lwk_bindings && uniffi-bindgen-go --out-dir ../../internal/onchain/liquid-wallet/ --library ../target/release/liblwk.a

bdk-bindings: build-bdk bindgen-go
	cd $(BDK_BINDINGS_PATH) && uniffi-bindgen-go --out-dir ../internal/onchain/bitcoin-wallet/ --library ./target/release/libbdk.a


#
# Tests
#

unit:
	@$(call print, "Running unit tests")
	$(GOTEST) ./... -v -tags unit

integration: start-regtest
	@$(call print, "Running integration tests")
	$(GOTEST) ./... -v

setup-regtest:
	@$(call print, "Preparing regtest")
	@if [ ! -d regtest/.git ]; then \
		make submodules; \
	fi
	@cd regtest && (git apply --reverse --check ../regtest.patch > /dev/null 2>&1 || git apply ../regtest.patch)

clear-wallet-data:
	rm -rf internal/onchain/liquid-wallet/test-data
	rm -rf internal/rpcserver/test/liquid-wallet

start-regtest: setup-regtest clear-wallet-data
	@$(call print, "Starting regtest")
	eval cd regtest && ./start.sh

restart-regtest: setup-regtest clear-wallet-data
	@$(call print, "Restarting regtest")
	eval cd regtest && ./restart.sh

#
# Building
#

RUST_TARGET ?=
RUST_TARGET_ARG := $(if $(RUST_TARGET),--target $(RUST_TARGET),)
RUST_RELEASE_DIR := $(if $(RUST_TARGET),$(RUST_TARGET)/,)release

build-bolt12:
	@$(call print, "Building bolt12")
	cd internal/lightning/lib/bolt12 && cargo build --release $(RUST_TARGET_ARG)
	@if [ -n "$(RUST_TARGET)" ]; then \
		mkdir -p internal/lightning/lib/bolt12/target/release && \
		cp internal/lightning/lib/bolt12/target/$(RUST_RELEASE_DIR)/libbolt12.a \
			internal/lightning/lib/bolt12/target/$(RUST_RELEASE_DIR)/libbolt12.so \
			internal/lightning/lib/bolt12/target/release/; \
	fi

build-lwk:
	@$(call print, "Building lwk")
	cd lwk/lwk_bindings && cargo build --release $(RUST_TARGET_ARG) --lib
	cp lwk/target/$(RUST_RELEASE_DIR)/liblwk.a lwk/target/$(RUST_RELEASE_DIR)/liblwk.so internal/onchain/liquid-wallet/lwk/

BDK_BINDINGS_PATH := ./bdk

build-bdk:
	@$(call print, "Building bdk")
	cd $(BDK_BINDINGS_PATH) && cargo build --release $(RUST_TARGET_ARG) --lib
	cp $(BDK_BINDINGS_PATH)/target/$(RUST_RELEASE_DIR)/libbdk.a $(BDK_BINDINGS_PATH)/target/$(RUST_RELEASE_DIR)/libbdk.so internal/onchain/bitcoin-wallet/bdk/

build: build-bolt12 build-lwk build-bdk
	@$(call print, "Building boltz-client")
	$(GOBUILD) $(ARGS) -o boltzd $(LDFLAGS) $(PKG_BOLTZD)
	$(GOBUILD) $(ARGS) -o boltzcli $(LDFLAGS) $(PKG_BOLTZ_CLI)

build-examples:
	@$(call print, "Building examples")
	cd examples && $(GOBUILD) $(ARGS) -o bin/submarine $(LDFLAGS) ./submarine
	cd examples && $(GOBUILD) $(ARGS) -o bin/reverse $(LDFLAGS) ./reverse

static: build-bolt12 build-lwk build-bdk
	@$(call print, "Building static boltz-client")
	$(GOBUILD) -tags static -o boltzd $(LDFLAGS) $(PKG_BOLTZD)
	$(GOBUILD) -tags static -o boltzcli $(LDFLAGS) $(PKG_BOLTZ_CLI)


daemon:
	@$(call print, "running boltzd")
	$(GORUN) $(LDFLAGS) $(PKG_BOLTZD)

cli:
	@$(call print, "running boltzcli")
	$(GORUN) $(LDFLAGS) $(PKG_BOLTZ_CLI)

install:
	@$(call print, "Installing boltz-client")
	$(GOINSTALL) $(LDFLAGS) $(PKG_BOLTZD)
	$(GOINSTALL) $(LDFLAGS) $(PKG_BOLTZ_CLI)


#
# Utils
#

submodules:
	@if git rev-parse --is-inside-work-tree > /dev/null 2>&1; then \
		$(call print, "Updating submodules"); \
		git submodule update --init --recursive; \
	else \
		$(call print, "Skipping submodule update outside a git checkout"); \
	fi

mockery:
	@$(call print, "Generating mocks")
	mockery

fmt:
	@$(call print, "Formatting source")
	gofmt -l -s -w .

lint:
	@$(call print, "Linting source")
	golangci-lint run -v

changelog:
	@$(call print, "Updating changelog")
	git-chglog --output CHANGELOG.md

PLATFORMS := linux/amd64,linux/arm64
DOCKER_CACHE := boltz/boltz-client:buildcache
DOCKER_ARGS := \
	--platform $(PLATFORMS) \
	--build-arg GO_VERSION=$(GO_VERSION) \
	--build-arg RUST_VERSION=$(RUST_VERSION)
DOCKER_CACHE_FROM_ARGS := \
	--cache-from type=registry,ref=$(DOCKER_CACHE)
DOCKER_CACHE_TO_ARGS := \
	--cache-to type=registry,ref=$(DOCKER_CACHE),mode=max
DOCKER_CACHE_ARGS := $(DOCKER_CACHE_FROM_ARGS) $(DOCKER_CACHE_TO_ARGS)
DOCKER_BINARY_CACHE_ARGS ?= $(DOCKER_CACHE_FROM_ARGS)

docker:
	@$(call print, "Building docker image")
	docker buildx build --push -t boltz/boltz-client:$(VERSION) -t boltz/boltz-client:latest $(DOCKER_ARGS) $(DOCKER_CACHE_ARGS) .

binaries:
	@$(call print, "Building binaries")
	docker buildx build --output bin --target binaries $(DOCKER_ARGS) $(DOCKER_BINARY_CACHE_ARGS) .
	tar -czvf boltz-client-linux-amd64-v$(VERSION).tar.gz bin/linux_amd64
	tar -czvf boltz-client-linux-arm64-v$(VERSION).tar.gz bin/linux_arm64
	sha256sum boltz-client-*.tar.gz bin/**/* > boltz-client-manifest-v$(VERSION).txt

.PHONY: build binaries
