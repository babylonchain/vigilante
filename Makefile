MOCKS_DIR=$(CURDIR)/testutil/mocks
MOCKGEN_REPO=github.com/golang/mock/mockgen
MOCKGEN_VERSION=v1.6.0
MOCKGEN_CMD=go run ${MOCKGEN_REPO}@${MOCKGEN_VERSION}
BUILDDIR ?= $(CURDIR)/build


ifeq ($(LINK_STATICALLY),true)
    ldflags += -linkmode=external -extldflags "-Wl,-z,muldefs -static"
endif
ldflags += $(LDFLAGS)
ldflags := $(strip $(ldflags))

build_tags += $(BUILD_TAGS)

BUILD_TARGETS := build install
BUILD_FLAGS := -tags "$(build_tags)" -ldflags '$(ldflags)'

all: build install

build: BUILD_ARGS=-o $(BUILDDIR)
build-linux:
	CGO_LDFLAGS="$CGO_LDFLAGS -lstdc++ -lm -lsodium" GOOS=linux GOARCH=$(if $(findstring aarch64,$(shell uname -m)) || $(findstring arm64,$(shell uname -m)),arm64,amd64) LEDGER_ENABLED=false $(MAKE) build

$(BUILD_TARGETS): go.sum $(BUILDDIR)/
	go $@ -mod=readonly $(BUILD_FLAGS) $(BUILD_ARGS) ./...

$(BUILDDIR)/:
	mkdir -p $(BUILDDIR)/

test:
	go test ./...

reporter-build:
	$(MAKE) -C contrib/images reporter

submitter-build:
	$(MAKE) -C contrib/images submitter

monitor-build:
	$(MAKE) -C contrib/images monitor

mock-gen: 
	mkdir -p $(MOCKS_DIR)
	$(MOCKGEN_CMD) -source=btcclient/interface.go -package mocks -destination $(MOCKS_DIR)/btcclient.go
