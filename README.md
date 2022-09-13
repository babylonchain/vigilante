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
```shell
make build
```

## Running the vigilante

For the following:
```shell
BABYLON_PATH="path_where_babylon_is_built" # example: $HOME/Projects/Babylon/babylon
VIGILANTE_PATH="root_vigilante_dir" # example: $HOME/Projects/Babylon/vigilante
TESTNET_PATH="path_where_the_testnet_files_will_be_stored" # example: $HOME/Projects/Babylon/babylon/.testnet
```

### Babylon configuration

Initially, create a testnet files for Babylon.
In this snippet, we create only one node, but this can work
for an arbitrary number of nodes.

```shell
$BABYLON_PATH/build/babylond testnet \
    --v                     1 \
    --output-dir            $TESTNET_PATH \
    --starting-ip-address   192.168.10.2 \
    --keyring-backend       test \
    --chain-id              chain-test
```

Using this configuration, start the testnet for the single node.
```shell
$BABYLON_PATH/build/babylond start --home $TESTNET_PATH/node0/babylond
```

This will result in a Babylon node running in port `26657` and
a GRPC instance running in port `9090`.

### Bitcoin configuration

Create a directory that will store the Bitcoin configuration.
This will be later used to retrieve the certificate required for RPC connections.

```shell
mkdir $TESTNET_PATH/bitcoin
```

For a Docker deployment, we want the vigilante to be able to communicate with
the Babylon and Bitcoin instances running on the local network.
We can accomplish that through the `host.docker.internal` DNS name,
which the Docker network translates to the Docker machine.
To enable Bitcoin RPC requests, we need to add the `host.docker.internal`
DNS host to the `rpc.cert` file that was created by the previous command.
To do that we use the btcd `gencerts` utility,

```shell
gencerts -d $TESTNET_PATH/bitcoin/ -H host.docker.internal
```

#### Running a Bitcoin simnet with an arbitrary mining address

Launch a simnet Bitcoin node
which listens for RPC connections at port `18556` and
stores the RPC certificate under the `$TESTNET_PATH/bitcoin` directory.
The mining address is arbitrary.

```shell
btcd --simnet --rpclisten 127.0.0.1:18556 --rpcuser rpcuser --rpcpass rpcpass \
    --rpccert $TESTNET_PATH/bitcoin/rpc.cert --rpckey $TESTNET_PATH/bitcoin/rpc.key \
    --miningaddr SQqHYFTSPh8WAyJvzbAC8hoLbF12UVsE5s
```

#### Running a Bitcoin simnet with a wallet

Launch a simnet Bitcoin node
which listens for RPC connections at port `18556` and
stores the RPC certificate under the `$TESTNET_PATH/bitcoin` directory.

```shell
btcd --simnet --rpclisten 127.0.0.1:18556 --rpcuser rpcuser --rpcpass rpcpass \
     --rpccert $TESTNET_PATH/bitcoin/rpc.cert --rpckey $TESTNET_PATH/bitcoin/rpc.key
```

Leave this process running.

Then, create a simnet Bitcoin wallet.
If you want to use the default vigilante file, then give the password `walletpass`.
Otherwise, make sure to edit the `vigilante.yaml` to reflect the correct password.
```shell
btcwallet --simnet -u rpcuser -P rpcpass \
          --rpccert $TESTNET_PATH/bitcoin/rpc-wallet.cert --rpckey $TESTNET_PATH/bitcoin/rpc-wallet.key \
          --cafile $TESTNET_PATH/bitcoin/rpc.cert \
          --create
```

The above instruction is going to prompt you for a password and going to give you the seed.
Store those securely.

Afterwards, start the wallet service listening to port `18554`:
```shell
btcwallet --simnet -u rpcuser -P rpcpass --rpclisten=127.0.0.1:18554 \
          --rpccert $TESTNET_PATH/bitcoin/rpc-wallet.cert --rpckey $TESTNET_PATH/bitcoin/rpc-wallet.key \
          --cafile $TESTNET_PATH/bitcoin/rpc.cert
```
Leave this process running. If you get an error that a wallet already exists and you still want
to create one, delete the `wallet.db` file located in the path displayed by the error message.

Create an address that will be later used for mining. The output below is a sample one.
```shell
$ btcctl --simnet --wallet -u rpcuser -P rpcpass \
       --rpccert $TESTNET_PATH/bitcoin/rpc-wallet.cert \
       --rpcserver 127.0.0.1 getnewaddress
       
SQqHYFTSPh8WAyJvzbAC8hoLbF12UVsE5s
```

Finally, restart the btcd service with the new address.
First, kill the `btcd` process that you started in the first step, and then:
```sh
btcd --simnet --rpclisten 127.0.0.1:18556 --rpcuser rpcuser --rpcpass rpcpass \
    --rpccert $TESTNET_PATH/bitcoin/rpc.cert --rpckey $TESTNET_PATH/bitcoin/rpc.key \
    --miningaddr $MINING_ADDRESS
```
where `$MINING_ADDRESS` is the address that you got as an output in the previous command.


#### Generating BTC blocks

While running this setup, one might want to generate BTC blocks.
We accomplish that through the btcd `btcctl` utility and the use
of the parameters we defined above.
```shell
btcctl --simnet --wallet --rpcuser=rpcuser --rpcpass=rpcpass \
       --rpccert=$TESTNET_PATH/bitcoin/rpc-wallet.cert \
       generate $NUM_BLOCKS
```
where `$NUM_BLOCKS` is the number of blocks you want to generate.

### Vigilante configuration

Create a directory which will store the vigilante configuration,
copy the sample vigilante configuration into a `vigilante.yml` file, and
adapt it to the specific requirements.

Currently, the vigilante configuration should be edited manually.
In the future, we will add functionality for generating this file through
a script.
For Docker deployments, we have created the `sample-vigilante-docker.yaml`
file which contains a configuration that will work out of this box for this guide.

```shell
mkdir $TESTNET_PATH/vigilante
```

### Running the vigilante locally

#### Running the vigilante reporter

Initially, copy the sample configuration
```shell
cp sample-vigilante.yml $TESTNET_PATH/vigilante/vigilante.yml
nano $TESTNET_PATH/vigilante/vigilante.yml # edit the config file to replace $TESTNET instances 
```

```shell
go run $VIGILANTE_PATH/cmd/main.go reporter \
         --config $TESTNET_PATH/vigilante/vigilante.yml \
         --babylon-key $BABYLON_PATH/.testnet/node0/babylond
```

#### Note
If you face permission issues doing `go mod download `or `go get <dependency>`, try following
```
export GOPRIVATE=github.com/babylonchain/babylon
```

#### Running the vigilante submitter

```shell
go run $VIGILANTE_PATH/cmd/main.go submitter \
         --config $TESTNET_PATH/vigilante/vigilante.yml \
         --babylon-key $TESTNET_PATH/node0/babylond
```

### Running the vigilante using Docker

#### Running the vigilante reporter

Initially, build a Docker image named `babylonchain/vigilante-reporter`
```shell
cp sample-vigilante-docker.yaml $TESTNET_PATH/vigilante/vigilante.yml
make GITHUBUSER=<your_Github_username> GITHUBPASS=<your_Github_access_token> reporter-build
```
where `<your_Github_access_token>` can be generated
at [github.com/settings/tokens](https://github.com/settings/tokens).
The Github access token is used for retrieving the `babylonchain/babylon`
dependency, which at the moment remains as a private repo.


See https://go.dev/doc/faq#git_https for more information.

Afterwards, run the above image and attach the directories
that contain the configuration for Babylon, Bitcoin, and the vigilante.

```shell
docker run --rm \
         -v $TESTNET_PATH/bitcoin:/bitcoin \
         -v $TESTNET_PATH/node0/babylond:/babylon \
         -v $TESTNET_PATH/vigilante:/vigilante \
         babylonchain/vigilante-reporter
```

#### Running the vigilante submitter

Follow the same steps as above, but with the `babylonchain/vigilante-submitter` Docker image.
```shell
make GITHUBUSER=<your_Github_username> GITHUBPASS=<your_Github_access_token> submitter-build
docker run --rm \
         -v $TESTNET_PATH/bitcoin:/bitcoin \
         -v $TESTNET_PATH/node0/babylond:/babylon \
         -v $TESTNET_PATH/vigilante:/vigilante \
         babylonchain/vigilante-submitter
```

#### buildx

The above `Dockerfile`s are also compatible with Docker's [buildx feature](https://docs.docker.com/desktop/multi-arch/)
that allows multi-architectural builds. To have a multi-architectural build,

```shell
docker buildx create --use
make GITHUBUSER=<your_Github_username> GITHUBPASS=<your_Github_access_token> reporter-buildx  # for the reporter
make GITHUBUSER=<your_Github_username> GITHUBPASS=<your_Github_access_token> submitter-buildx # for the submitter
```
