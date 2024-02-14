DOCKER = $(shell which docker)
MOCKS_DIR=$(CURDIR)/testutil/mocks
MOCKGEN_REPO=github.com/golang/mock/mockgen
MOCKGEN_VERSION=v1.6.0
MOCKGEN_CMD=go run ${MOCKGEN_REPO}@${MOCKGEN_VERSION}
BUILDDIR ?= $(CURDIR)/build
TOOLS_DIR := tools

BTCD_PKG := github.com/btcsuite/btcd
BTCDW_PKG := github.com/btcsuite/btcwallet
BABYLON_PKG := github.com/babylonchain/babylon/cmd/babylond

GO_BIN := ${GOPATH}/bin
BTCD_BIN := $(GO_BIN)/btcd

ldflags := $(LDFLAGS)
build_tags := $(BUILD_TAGS)
build_args := $(BUILD_ARGS)

PACKAGES_E2E=$(shell go list ./... | grep '/e2e')

ifeq ($(LINK_STATICALLY),true)
	ldflags += -linkmode=external -extldflags "-Wl,-z,muldefs -static" -v
endif

ifeq ($(VERBOSE),true)
	build_args += -v
endif

BUILD_TARGETS := build install
BUILD_FLAGS := --tags "$(build_tags)" --ldflags '$(ldflags)'

# Update changelog vars
ifneq (,$(SINCE_TAG))
       sinceTag := --since-tag $(SINCE_TAG)
endif
ifneq (,$(UPCOMING_TAG))
       upcomingTag := --upcoming-tag $(UPCOMING_TAG)
endif

all: build install

build: BUILD_ARGS := $(build_args) -o $(BUILDDIR)

$(BUILD_TARGETS): go.sum $(BUILDDIR)/
	go $@ -mod=readonly $(BUILD_FLAGS) $(BUILD_ARGS) ./...

$(BUILDDIR)/:
	mkdir -p $(BUILDDIR)/

test:
	go test ./...

test-e2e:
	cd $(TOOLS_DIR); go install -trimpath $(BTCD_PKG); go install -trimpath $(BTCDW_PKG); go install -trimpath $(BABYLON_PKG);
	go test -mod=readonly -timeout=25m -v $(PACKAGES_E2E) -count=1 --tags=e2e

build-docker:
	$(DOCKER) build --tag babylonchain/vigilante -f Dockerfile \
		$(shell git rev-parse --show-toplevel)

rm-docker:
	$(DOCKER) rmi babylonchain/vigilante 2>/dev/null; true

mocks:
	mkdir -p $(MOCKS_DIR)
	$(MOCKGEN_CMD) -source=btcclient/interface.go -package mocks -destination $(MOCKS_DIR)/btcclient.go
	$(MOCKGEN_CMD) -source=submitter/poller/expected_babylon_client.go -package poller -destination submitter/poller/mock_babylon_client.go
	$(MOCKGEN_CMD) -source=submitter/expected_babylon_client.go -package submitter -destination submitter/mock_babylon_client.go
	$(MOCKGEN_CMD) -source=reporter/expected_babylon_client.go -package reporter -destination reporter/mock_babylon_client.go
	$(MOCKGEN_CMD) -source=monitor/expected_babylon_client.go -package monitor -destination monitor/mock_babylon_client.go
	$(MOCKGEN_CMD) -source=btcstaking-tracker/btcslasher/expected_babylon_client.go -package btcslasher -destination btcstaking-tracker/btcslasher/mock_babylon_client.go
	$(MOCKGEN_CMD) -source=btcstaking-tracker/atomicslasher/expected_babylon_client.go -package atomicslasher -destination btcstaking-tracker/atomicslasher/mock_babylon_client.go
	$(MOCKGEN_CMD) -source=btcstaking-tracker/unbondingwatcher/expected_babylon_client.go -package unbondingwatcher -destination btcstaking-tracker/unbondingwatcher/mock_babylon_client.go

update-changelog:
	@echo ./scripts/update_changelog.sh $(sinceTag) $(upcomingTag)
	./scripts/update_changelog.sh $(sinceTag) $(upcomingTag)

.PHONY: build test test-e2e build-docker rm-docker mocks update-changelog
