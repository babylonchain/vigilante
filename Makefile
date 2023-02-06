MOCKS_DIR=$(CURDIR)/testutil/mocks
MOCKGEN_REPO=github.com/golang/mock/mockgen
MOCKGEN_VERSION=v1.6.0
MOCKGEN_CMD=go run ${MOCKGEN_REPO}@${MOCKGEN_VERSION}

all: build

build:
	go build ./cmd/main.go

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
