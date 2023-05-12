DOCKER = $(shell which docker)

all: vigilante

vigilante: vigilante-rmi
	$(DOCKER) build --tag babylonchain/vigilante -f vigilante/Dockerfile \
		$(shell git rev-parse --show-toplevel)

vigilante-rmi:
	$(DOCKER) rmi babylonchain/vigilante 2>/dev/null; true
