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

## Running the vigilante

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
