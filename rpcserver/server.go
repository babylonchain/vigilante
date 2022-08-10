// Copyright (c) 2022-2022 The Babylon developers
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
	"fmt"
	"net"

	"github.com/babylonchain/vigilante/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
)

func New(cfg *config.GRPCConfig) (*grpc.Server, error) {
	keyPair, err := openRPCKeyPair(cfg.OneTimeTLSKey, cfg.RPCKeyFile, cfg.RPCCertFile)
	if err != nil {
		return nil, fmt.Errorf("Open RPC key pair: %v", err)
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
			log.Errorf("Listen: %v", err)
		} else {
			listeners = append(listeners, lis)
		}
	}

	// start the server with listeners, each in a goroutine
	for _, lis := range listeners {
		go func(l net.Listener) {
			if err := server.Serve(l); err != nil {
				log.Errorf("Serve RPC server: %v", err)
			} else {
				log.Infof("Successfully started the GRPC server at %v", l.Addr().String())
			}
		}(lis)
	}

	return server, nil
}
