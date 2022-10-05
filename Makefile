MOCKS_DIR = ./testutil/mocks/
MOCKGEN_CMD = go run github.com/golang/mock/mockgen@v1.6.0

all: build

build:
	go build ./cmd/main.go

test:
	go test ./...

reporter-build:
	$(MAKE) -C contrib/images reporter

submitter-build:
	$(MAKE) -C contrib/images submitter

mock-gen: 
	mkdir -p $(MOCKS_DIR)
	$(MOCKGEN_CMD) -source=btcclient/interface.go -package mocks -destination testutil/mocks/btcclient.go
	$(MOCKGEN_CMD) -source=babylonclient/interface.go -package mocks -destination testutil/mocks/babylonclient.go
