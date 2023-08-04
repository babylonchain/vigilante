DOCKER = $(shell which docker)
MOCKS_DIR=$(CURDIR)/testutil/mocks
MOCKGEN_REPO=github.com/golang/mock/mockgen
MOCKGEN_VERSION=v1.6.0
MOCKGEN_CMD=go run ${MOCKGEN_REPO}@${MOCKGEN_VERSION}
BUILDDIR ?= $(CURDIR)/build
TOOLS_DIR := tools

BTCD_PKG := github.com/btcsuite/btcd
BTCDW_PKG := github.com/btcsuite/btcwallet

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

all: build install

build: BUILD_ARGS := $(build_args) -o $(BUILDDIR)

$(BUILD_TARGETS): go.sum $(BUILDDIR)/
	go $@ -mod=readonly $(BUILD_FLAGS) $(BUILD_ARGS) ./...

$(BUILDDIR)/:
	mkdir -p $(BUILDDIR)/

test:
	go test ./...

test-e2e:
	cd $(TOOLS_DIR); go install -trimpath $(BTCD_PKG); go install -trimpath $(BTCDW_PKG)
	go test -mod=readonly -timeout=25m -v $(PACKAGES_E2E) -count=1 --tags=e2e

build-docker:
	$(DOCKER) build --secret id=sshKey,src=${BBN_PRIV_DEPLOY_KEY} --tag babylonchain/vigilante -f Dockerfile \
		$(shell git rev-parse --show-toplevel)

rm-docker:
	$(DOCKER) rmi babylonchain/vigilante 2>/dev/null; true

mock-gen:
	mkdir -p $(MOCKS_DIR)
	$(MOCKGEN_CMD) -source=btcclient/interface.go -package mocks -destination $(MOCKS_DIR)/btcclient.go

.PHONY: build test test-e2e build-docker rm-docker mock-gen
