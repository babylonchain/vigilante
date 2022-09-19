all: build

build:
	go build ./cmd/main.go

test:
	go test ./...

reporter-build:
	$(MAKE) -C contrib/images reporter

submitter-build:
	$(MAKE) -C contrib/images submitter
