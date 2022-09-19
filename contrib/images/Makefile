DOCKER = $(shell which docker)
GITHUBUSER = ${GITHUBUSER:-user}
GITHUBTOKEN = ${GITHUBTOKEN:-"token"}

all: reporter submitter

reporter: reporter-rmi
	$(DOCKER) build --build-arg user=$(GITHUBUSER) --build-arg pass=$(GITHUBTOKEN) \
		--tag babylonchain/vigilante-reporter -f reporter/Dockerfile \
		$(shell git rev-parse --show-toplevel)

reporter-rmi:
	$(DOCKER) rmi babylonchain/vigilante-reporter 2>/dev/null; true

submitter: submitter-rmi
	$(DOCKER) build --build-arg user=$(GITHUBUSER) --build-arg pass=$(GITHUBTOKEN) \
		--tag babylonchain/vigilante-submitter -f submitter/Dockerfile \
		$(shell git rev-parse --show-toplevel)

submitter-rmi:
	$(DOCKER) rmi babylonchain/vigilante-submitter 2>/dev/null; true
