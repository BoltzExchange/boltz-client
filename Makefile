PKG := github.com/BoltzExchange/boltz-client/v2
VERSION := 2.5.1
GDK_VERSION = 0.75.1
GO_VERSION := 1.24.2
RUST_VERSION := 1.82.0

PKG_BOLTZD := $(PKG)/cmd/boltzd
PKG_BOLTZ_CLI := $(PKG)/cmd/boltzcli

GO_BIN := ${GOPATH}/bin

GOTEST := CGO_ENABLED=1 GO111MODULE=on go test -v -timeout 5m
GOBUILD := CGO_ENABLED=1 GO111MODULE=on go build -v
GORUN := CGO_ENABLED=1 GO111MODULE=on go run -v
GOINSTALL := CGO_ENABLED=1 GO111MODULE=on go install -v

COMMIT := $(shell git log --pretty=format:'%h' -n 1)
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

#
# Tests
#

unit:
	@$(call print, "Running unit tests")
	$(GOTEST) ./... -v -tags unit

integration: start-regtest
	@$(call print, "Running integration tests")
	$(GOTEST) ./... -v

download-regtest:
ifeq ("$(wildcard regtest/start.sh)","")
	@$(call print, "Downloading regtest")
	make submodules
	cp regtest.override.yml regtest/docker-compose.override.yml
	cd regtest && git apply ../regtest.patch
endif

start-regtest: download-regtest
	@$(call print, "Starting regtest")
	eval cd regtest && ./start.sh

restart-regtest: download-regtest
	@$(call print, "Restarting regtest")
	eval cd regtest && ./restart.sh

#
# Building
#

build-bolt12:
	@$(call print, "Building bolt12")
	cd internal/lightning/lib/bolt12 && cargo build --release

build: download-gdk build-bolt12
	@$(call print, "Building boltz-client")
	$(GOBUILD) $(ARGS) -o boltzd $(LDFLAGS) $(PKG_BOLTZD)
	$(GOBUILD) $(ARGS) -o boltzcli $(LDFLAGS) $(PKG_BOLTZ_CLI)

static: download-gdk build-bolt12
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

deps: submodules
	go mod vendor
	cp -r ./go-secp256k1-zkp/secp256k1-zkp ./vendor/github.com/vulpemventures/go-secp256k1-zkp
	# exclude the package and any lines including a # (#cgo, #include, etc.)
	cd ./vendor/github.com/vulpemventures/go-secp256k1-zkp && \
		sed -i '/#\|package/!s/secp256k1/go_secp256k1/g' *.go && \
		find secp256k1-zkp -type f -name "*.c" -print0 | xargs -0 sed -i '/include/!s/secp256k1/go_secp256k1/g' && \
		find secp256k1-zkp -type f -name "*.h" -print0 | xargs -0 sed -i '/include/!s/secp256k1/go_secp256k1/g'

download-gdk:
ifeq ("$(wildcard internal/onchain/wallet/lib/libgreen_gdk.so)","")
	@$(call print, "Downloading gdk library")
	@container_id=$$(docker create "boltz/gdk-ubuntu:$(GDK_VERSION)" true); \
	docker cp "$$container_id:/" internal/onchain/wallet/lib/ && \
	docker rm "$$container_id";
endif


#
# Utils
#

submodules:
	@$(call print, "Updating submodules")
	git submodule update --init --recursive

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
DOCKER_ARGS := --platform $(PLATFORMS) --build-arg GO_VERSION=$(GO_VERSION) --build-arg GDK_VERSION=$(GDK_VERSION) --build-arg RUST_VERSION=$(RUST_VERSION)
DOCKER_CACHE_ARGS := --cache-from type=registry,ref=$(DOCKER_CACHE) --cache-to type=registry,ref=$(DOCKER_CACHE),mode=max

docker:
	@$(call print, "Building docker image")
	docker buildx build --push -t boltz/boltz-client:$(VERSION) -t boltz/boltz-client:latest $(DOCKER_ARGS) $(DOCKER_CACHE_ARGS) .

binaries:
	@$(call print, "Building binaries")
	docker buildx build --output bin --target binaries $(DOCKER_ARGS) $(DOCKER_CACHE_ARGS) .
	tar -czvf boltz-client-linux-amd64-v$(VERSION).tar.gz bin/linux_amd64
	tar -czvf boltz-client-linux-arm64-v$(VERSION).tar.gz bin/linux_arm64
	sha256sum boltz-client-*.tar.gz bin/**/* > boltz-client-manifest-v$(VERSION).txt

gdk-source: submodules
	cd gdk && git checkout release_$(GDK_VERSION)
	cp ./gdk/include/gdk.h ./internal/onchain/wallet/include/gdk.h

GDK_AMD64_BUILDER := blockstream/gdk-ubuntu-builder@sha256:0faa0e15127f3a2a025c2c7e92764c617c24417c40a91f950e777fb99620aa9a
GDK_ARM64_BUILDER := blockstream/gdk-ubuntu-builder@sha256:66a546eff8c28be6af96a26791bf34306710be30c20ff1d7447d66521a5defcd

build-gdk:
	docker buildx build --push -t boltz/gdk-ubuntu:latest -t boltz/gdk-ubuntu:$(GDK_VERSION) -f gdk.Dockerfile $(DOCKER_ARGS) \
		--build-arg GDK_AMD64_BUILDER=$(GDK_AMD64_BUILDER) \
		--build-arg GDK_ARM64_BUILDER=$(GDK_ARM64_BUILDER) .

.PHONY: build binaries
