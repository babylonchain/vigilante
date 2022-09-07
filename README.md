# vigilante

Vigilante program for Babylon

## Requirements

- Go 1.18

## Building

Note that Vigilante depends on https://github.com/babylonchain/babylon, which is still a private repository.
In order to allow Go to retrieve private dependencies, one needs to enforce Git to use SSH (rather than HTTPS) for authentication, by adding the following lines to your `~/.gitconfig`:

```
[url "ssh://git@github.com/"]
	insteadOf = https://github.com/
```

In order to build the vigilante,
```bash
$ make build
```

## Running the vigilante

### Babylon configuration

Initially, create a testnet files for Babylon.
In this snippet, we create only one node, but this can work
for an arbitrary number of nodes.

```bash
$ babylond testnet \
    --v                     1 \
    --output-dir            ./.testnet \
    --starting-ip-address   192.168.10.2 \
    --keyring-backend       test \
    --chain-id              chain-test
```

Using this configuration, start the testnet for the single node.
```bash
$ babylond start --home ./.testnet/node0/babylond
```

This will result in a Babylon node running in port `26657` and
a GRPC instance running in port `9090`.

### Bitcoin configuration

Create a directory that will store the Bitcoin configuration.
This will be later used to retrieve the certificate required for RPC connections.

```bash
$ mkdir ./.testnet/bitcoin
```

For a Docker deployment, we want the vigilante to be able to communicate with
the Babylon and Bitcoin instances running on the local network.
We can accomplish that through the `host.docker.internal` DNS name,
which the Docker network translates to the Docker machine.
To enable Bitcoin RPC requests, we need to add the `host.docker.internal`
DNS host to the `rpc.cert` file that was created by the previous command.
To do that we use the btcd `gencerts` utility,
```bash
$ gencerts -d ./.testnet/bitcoin/ -H host.docker.internal
```


Then, launch a simnet Bitcoin node,
which listens for RPC connections at port `18554` and
stores the RPC certificate under the above directory.
The mining address is an arbitrary address.
```bash
$ btcd --simnet --rpclisten 127.0.0.1:18554 --rpcuser rpcuser --rpcpass rpcpass \
    --rpccert ./.testnet/bitcoin/rpc.cert --rpckey ./.testnet/bitcoin/rpc.key \
    --miningaddr SQqHYFTSPh8WAyJvzbAC8hoLbF12UVsE5s
```

#### Generating BTC blocks

While running this setup, one might want to generate BTC blocks.
We accomplish that through the btcd `btcctl` utility and the use
of the parameters we defined above.
```bash
$ btcctl --simnet --wallet --skipverify \
         --rpcuser=rpcuser --rpcpass=rpcpass --rpccert=./.testnet/bitcoin/rpc.cert \
         generate 1
```

### Vigilante configuration

Create a directory which will store the vigilante configuration,
copy the sample vigilante configuration into a `vigilante.yaml` file, and
adapt it to the specific requirements.

Currently, the vigilante configuration should be edited manually.
In the future, we will add functionality for generating this file through
a script.
For Docker deployments, we have created the `sample-vigilante-docker.yaml`
file which contains a configuration that will work out of this box for this guide.

```bash
$ mkdir ./.testnet/vigilante
$ cp sample-vigilante-docker.yaml ./.testnet/vigilante/vigilante.yaml
```

### Running the vigilante locally

#### Running the vigilante reporter

```bash
$ go run ./cmd/main.go reporter \
         --config ./.testnet/vigilante/vigilante.yaml \
         --babylon-key ./.testnet/node0/babylond
```

#### Running the vigilante submitter

```bash
$ go run ./cmd/main.go submitter \
         --config ./.testnet/vigilante/vigilante.yaml \
         --babylon-key ./.testnet/node0/babylond
```

### Running the vigilante using Docker

#### Running the vigilante reporter

Initially, build a Docker image named `babylonchain/vigilante-reporter`
```bash
$ make GITHUBUSER=<your_Github_username> GITHUBPASS=<your_Github_access_token> reporter-build
```
where `<your_Github_access_token>` can be generated
at [github.com/settings/tokens](https://github.com/settings/tokens).
The Github access token is used for retrieving the `babylonchain/babylon`
dependency, which at the moment remains as a private repo.


See https://go.dev/doc/faq#git_https for more information.

Afterwards, run the above image and attach the directories
that contain the configuration for Babylon, Bitcoin, and the vigilante.

```bash
$ docker run --rm \
         -v $PWD/.testnet/bitcoin:/bitcoin \
         -v $PWD/.testnet/node0/babylond:/babylon \
         -v $PWD/.testnet/vigilante:/vigilante \
         babylonchain/vigilante-reporter
```

#### Running the vigilante submitter

Follow the same steps as above, but with the `babylonchain/vigilante-submitter` Docker image.
```bash
$ make GITHUBUSER=<your_Github_username> GITHUBPASS=<your_Github_access_token> submitter-build
$ docker run --rm \
         -v $PWD/.testnet/bitcoin:/bitcoin \
         -v $PWD/.testnet/node0/babylond:/babylon \
         -v $PWD/.testnet/vigilante:/vigilante \
         babylonchain/vigilante-submitter
```

#### buildx

The above `Dockerfile`s are also compatible with Docker's [buildx feature](https://docs.docker.com/desktop/multi-arch/)
that allows multi-architectural builds. To have a multi-architectural build,

```bash
$ docker buildx create --use
$ make GITHUBUSER=<your_Github_username> GITHUBPASS=<your_Github_access_token> reporter-buildx  # for the reporter
$ make GITHUBUSER=<your_Github_username> GITHUBPASS=<your_Github_access_token> submitter-buildx # for the submitter
```
