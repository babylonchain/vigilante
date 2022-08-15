# vigilante

Vigilante program for Babylon

## Requirements

- Go 1.18

## Development requirements

- Go 1.18

## Building

```bash
$ go build ./cmd/main.go
```

## Running the vigilante locally

1. Launch a Bitcoin node

```bash
$ btcd --simnet --rpclisten 127.0.0.1:18554 --rpcuser user --rpcpass pass
```

2. Launch a Babylon node following Babylon's documentation

```bash
$ babylond testnet \
    --v                     1 \
    --output-dir            ./.testnet \
    --starting-ip-address   192.168.10.2 \
    --keyring-backend       test \
    --chain-id              chain-test
$ babylond start --home ./.testnet/node0/babylond
```

1. Launch a vigilante

```bash
$ go run ./cmd/main.go reporter # vigilant reporter
$ go run ./cmd/main.go submitter # vigilant submitter
```

## Running the vigilante in Docker

One can use the `Dockerfile` to build a Docker image for the vigilant, by using the following CLI:

```bash
$ docker build -t babylonchain/vigilante:latest --build-arg user=<your_Github_username> --build-arg pass=<your_Github_access_token> .
```

where <your_Github_access_token> can be generated at [github.com/settings/tokens](https://github.com/settings/tokens).

This `Dockerfile` is also compatible with Docker's [buildx feature](https://www.docker.com/blog/multi-arch-build-and-images-the-simple-way/) that allows multi-architectural builds. To have a multi-architectural build,

```bash
$ docker buildx create --use
$ docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 -t babylonchain/vigilante:latest --build-arg user=<your_Github_username> --build-arg pass=<your_Github_access_token> .
```