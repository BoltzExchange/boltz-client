PKG := github.com/BoltzExchange/boltz-lnd

PKG_BOLTZD := github.com/BoltzExchange/boltz-lnd/cmd/boltzd
PKG_BOLTZ_CLI := github.com/BoltzExchange/boltz-lnd/cmd/boltzcli

GO_BIN := ${GOPATH}/bin

GOTEST := GO111MODULE=on go test -v
GOBUILD := GO111MODULE=on go build -v
GOINSTALL := GO111MODULE=on go install -v
GOLIST := go list -deps $(PKG)/... | grep '$(PKG)'| grep -v '/vendor/'

COMMIT := $(shell git log --pretty=format:'%h' -n 1)
LDFLAGS := -ldflags "-X $(PKG)/build.Commit=$(COMMIT)"

LINT_PKG := github.com/golangci/golangci-lint/cmd/golangci-lint
LINT_BIN := $(GO_BIN)/golangci-lint
LINT = $(LINT_BIN) run -v --timeout 5m

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
	go get $(LINT_PKG)


patch-btcutil:
	@$(call print, "Patching btcutil")
	patch -u vendor/github.com/btcsuite/btcutil/address.go -i btcutil.patch --forward || true

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

#
# Utils
#

fmt:
	@$(call print, "Formatting source")
	gofmt -l -s -w .

lint: $(LINT_BIN)
	@$(call print, "Linting source")
	$(LINT)

.PHONY: build
