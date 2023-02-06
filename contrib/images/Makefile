DOCKER = $(shell which docker)

all: reporter submitter monitor

reporter: reporter-rmi
	$(DOCKER) build --tag babylonchain/vigilante-reporter -f reporter/Dockerfile \
		$(shell git rev-parse --show-toplevel)

reporter-rmi:
	$(DOCKER) rmi babylonchain/vigilante-reporter 2>/dev/null; true

submitter: submitter-rmi
	$(DOCKER) build --tag babylonchain/vigilante-submitter -f submitter/Dockerfile \
		$(shell git rev-parse --show-toplevel)

submitter-rmi:
	$(DOCKER) rmi babylonchain/vigilante-submitter 2>/dev/null; true

monitor: monitor-rmi
	$(DOCKER) build --tag babylonchain/vigilante-monitor -f monitor/Dockerfile \
		$(shell git rev-parse --show-toplevel)

monitor-rmi:
	$(DOCKER) rmi babylonchain/vigilante-monitor 2>/dev/null; true
