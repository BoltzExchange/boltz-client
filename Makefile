PKG := github.com/BoltzExchange/boltz-client

PKG_BOLTZD := github.com/BoltzExchange/boltz-client/cmd/boltzd
PKG_BOLTZ_CLI := github.com/BoltzExchange/boltz-client/cmd/boltzcli

GO_BIN := ${GOPATH}/bin

GOTEST := CGO_ENABLED=1 GO111MODULE=on go test -v
GOBUILD := CGO_ENABLED=1 GO111MODULE=on go build -v
GORUN := CGO_ENABLED=1 GO111MODULE=on go run -v
GOINSTALL := CGO_ENABLED=1 GO111MODULE=on go install -v
GOLIST := go list -deps $(PKG)/... | grep '$(PKG)'| grep -v '/vendor/' | grep -v '/cmd/'

COMMIT := $(shell git log --pretty=format:'%h' -n 1)
LDFLAGS := -ldflags "-X $(PKG)/build.Commit=$(COMMIT) -w -s"

LINT_PKG := github.com/golangci/golangci-lint/cmd/golangci-lint
LINT_VERSION := v1.55.0

LINT_BIN := golangci-lint
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
	go install $(LINT_PKG)@$(LINT_VERSION)

$(CHANGELOG_BIN):
	@$(call print, "Fetching git-chglog")
	go get -u $(CHANGELOG_PKG)

$(TOOLS_PATH):
	eval export PATH="$PATH:$(go env GOPATH)/bin"


tools: $(TOOLS_PATH)
	@$(call print, "Installing tools")
	go install \
		"github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway" \
		"github.com/pseudomuto/protoc-gen-doc/cmd/protoc-gen-doc" \
		"google.golang.org/grpc/cmd/protoc-gen-go-grpc" \
		"google.golang.org/protobuf/cmd/protoc-gen-go" \

proto: $(TOOLS_PATH)
	@$(call print, "Generating protosbufs")
	eval cd boltzrpc && ./gen_protos.sh && cd ..

#
# Tests
#

unit:
	@$(call print, "Running unit tests")
	$(GOLIST) | $(XARGS) env $(GOTEST)

integration:
	@$(call print, "Running integration tests")
	$(GOTEST) -v $(PKG)/cmd/boltzd -skip TestAutoSwap/BTC/PerChannel


#
# Building
#

build: 
	@$(call print, "Building boltz-client")
	$(GOBUILD) $(ARGS) -o boltzd $(LDFLAGS) $(PKG_BOLTZD)
	$(GOBUILD) $(ARGS) -o boltzcli $(LDFLAGS) $(PKG_BOLTZ_CLI)

static:
	@$(call print, "Building static boltz-client")
	$(GOBUILD) -tags static -o boltzd $(LDFLAGS) $(PKG_BOLTZD)
	$(GOBUILD) -o boltzcli $(LDFLAGS) $(PKG_BOLTZ_CLI)


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

deps:
	go mod vendor
	cp -r ./go-secp256k1-zkp/secp256k1-zkp ./vendor/github.com/vulpemventures/go-secp256k1-zkp
	# exclude the package and any lines including a # (#cgo, #include, etc.)
	cd ./vendor/github.com/vulpemventures/go-secp256k1-zkp && \
		sed -i '/#\|package/!s/secp256k1/go_secp256k1/g' *.go && \
		find secp256k1-zkp -type f -name "*.c" -print0 | xargs -0 sed -i '/include/!s/secp256k1/go_secp256k1/g' && \
		find secp256k1-zkp -type f -name "*.h" -print0 | xargs -0 sed -i '/include/!s/secp256k1/go_secp256k1/g'
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

PLATFORMS := linux/amd64

docker:
	@$(call print, "Building docker image")
	docker buildx build --push --platform $(PLATFORMS) -t boltz/boltz-client:$(version) -t boltz/boltz-client:latest .

binaries:
	@$(call print, "Building binaries")
	docker buildx build --output bin --platform $(PLATFORMS) --target binaries .

.PHONY: build binaries
