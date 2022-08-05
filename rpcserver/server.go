// Copyright (c) 2015-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

// Package rpcserver implements the RPC API and is used by the main package to
// start gRPC services.
//
// Full documentation of the API implemented by this package is maintained in a
// language-agnostic document:
//
//   https://github.com/btcsuite/btcwallet/blob/master/rpc/documentation/api.md
//
// Any API changes must be performed according to the steps listed here:
//
//   https://github.com/btcsuite/btcwallet/blob/master/rpc/documentation/serverchanges.md
package rpcserver

import (
	"net"

	"github.com/babylonchain/vigilante/config"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
)

func New(cfg *config.GRPCConfig) (*grpc.Server, error) {
	keyPair, err := openRPCKeyPair(cfg.OneTimeTLSKey, cfg.RPCKeyFile, cfg.RPCCertFile)
	if err != nil {
		return nil, err
	}
	creds := credentials.NewServerTLSFromCert(&keyPair)

	server := grpc.NewServer(grpc.Creds(creds))
	reflection.Register(server)
	StartVigilanteService(server)

	// create listeners for endpoints
	listeners := []net.Listener{}
	for _, endpoint := range cfg.Endpoints {
		lis, err := net.Listen("tcp", endpoint)
		if err != nil {
			log.Fatalf("failed to listen: %v", err)
		}
		listeners = append(listeners, lis)
	}

	// start the server with listeners, each in a goroutine
	for _, lis := range listeners {
		go func(l net.Listener) {
			if err := server.Serve(l); err != nil {
				log.Errorf("serve RPC server: %v", err)
			}
		}(lis)
	}

	log.Infof("Successfully started the GRPC server")

	return server, nil
}
