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

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

func New() (*grpc.Server, error) {
	// TODO: TLS and other server opts
	server := grpc.NewServer()
	StartVigilanteService(server)

	// endpoint
	// TODO: config for ip:port
	lis, err := net.Listen("tcp", "localhost:8080")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// start the server listenting to this endpoint
	go func() {
		if err := server.Serve(lis); err != nil {
			log.Errorf("serve RPC server: %v", err)
		}
	}()

	return server, nil
}
