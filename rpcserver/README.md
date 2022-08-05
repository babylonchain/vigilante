# rpcserver

This package implements a GRPC server. The code is adapted from https://github.com/btcsuite/btcwallet/tree/master/rpc.

To test:

```bash
$ grpcurl --insecure localhost:8080 rpc.VigilanteService/Version
```