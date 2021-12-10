PKG := github.com/BoltzExchange/boltz-lnd

PKG_BOLTZD := github.com/BoltzExchange/boltz-lnd/cmd/boltzd
PKG_BOLTZ_CLI := github.com/BoltzExchange/boltz-lnd/cmd/boltzcli

GO_BIN := ${GOPATH}/bin

GOTEST := CGO_ENABLED=1 GO111MODULE=on go test -v
GOBUILD := CGO_ENABLED=1 GO111MODULE=on go build -v
GOINSTALL := CGO_ENABLED=1 GO111MODULE=on go install -v
GOLIST := go list -deps $(PKG)/... | grep '$(PKG)'| grep -v '/vendor/'

COMMIT := $(shell git log --pretty=format:'%h' -n 1)
LDFLAGS := -ldflags "-X $(PKG)/build.Commit=$(COMMIT) -w -s"

LINT_PKG := github.com/golangci/golangci-lint/cmd/golangci-lint
LINT_BIN := $(GO_BIN)/golangci-lint
LINT = $(LINT_BIN) run -v --timeout 5m

CHANGELOG_PKG := github.com/git-chglog/git-chglog/cmd/git-chglog
CHANGELOG_BIN := $(GO_BIN)/git-chglog
CHANGELOG := $(CHANGELOG_BIN) --output CHANGELOG.md

XARGS := xargs -L 1

GREEN := "\\033[0;32m"
NC := "\\033[0m"

define print
	echo $(GREEN)$1$(NC)
endef

default: build

#
# Dependencies
#

$(LINT_BIN):
	@$(call print, "Fetching linter")
	go get -u $(LINT_PKG)

$(CHANGELOG_BIN):
	@$(call print, "Fetching git-chglog")
	go get -u $(CHANGELOG_PKG)

patch-btcutil:
	@$(call print, "Patching btcutil")
	patch -u vendor/github.com/btcsuite/btcutil/address.go -i btcutil.patch --forward || true

proto:
	@$(call print, "Generating protosbufs")
	eval cd boltzrpc && ./gen_protos.sh && cd ..

#
# Tests
#

unit:
	@$(call print, "Running unit tests")
	$(GOLIST) | $(XARGS) env $(GOTEST)

#
# Building
#

build: patch-btcutil
	@$(call print, "Building boltz-lnd")
	$(GOBUILD) -o boltzd $(LDFLAGS) $(PKG_BOLTZD)
	$(GOBUILD) -o boltzcli $(LDFLAGS) $(PKG_BOLTZ_CLI)

install: patch-btcutil
	@$(call print, "Installing boltz-lnd")
	$(GOINSTALL) $(LDFLAGS) $(PKG_BOLTZD)
	$(GOINSTALL) $(LDFLAGS) $(PKG_BOLTZ_CLI)

binaries:
	@$(call print, "Compiling binaries")
	eval ./binaries.sh linux-amd64 linux-arm64 linux-arm windows-amd64 windows-386

#
# Utils
#

fmt:
	@$(call print, "Formatting source")
	gofmt -l -s -w .

lint: $(LINT_BIN)
	@$(call print, "Linting source")
	$(LINT)

changelog:
	@$(call print, "Updating changelog")
	$(CHANGELOG)

docker:
	@$(call print, "Building docker image")
	docker buildx build --push --platform linux/amd64 --platform linux/arm64 --platform linux/arm/v7 -t boltz/boltz-lnd:$(version) -t boltz/boltz-lnd:latest .

.PHONY: build binaries
