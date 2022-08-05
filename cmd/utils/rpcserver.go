package utils

import (
	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/rpcserver"
	"google.golang.org/grpc"
)

func NewRPCServer(cfg *config.Config) (*grpc.Server, error) {
	return rpcserver.New(cfg.GRPC.OneTimeTLSKey, cfg.GRPC.RPCKeyFile, cfg.GRPC.RPCCertFile, cfg.GRPC.Endpoints)
}
