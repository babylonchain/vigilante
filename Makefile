all: build

build:
	go build ./cmd/main.go

reporter-build:
	$(MAKE) -C contrib/images reporter

submitter-build:
	$(MAKE) -C contrib/images submitter
