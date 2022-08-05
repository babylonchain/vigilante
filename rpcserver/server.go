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
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
)

func New(oneTimeTLSKey bool, RPCKeyFile string, RPCCertFile string, endpoints []string) (*grpc.Server, error) {

	// TODO: TLS and other server opts
	keyPair, err := openRPCKeyPair(oneTimeTLSKey, RPCKeyFile, RPCCertFile)
	if err != nil {
		return nil, err
	}
	creds := credentials.NewServerTLSFromCert(&keyPair)

	server := grpc.NewServer(grpc.Creds(creds))
	reflection.Register(server)
	StartVigilanteService(server)

	// create listeners for endpoints
	listeners := []net.Listener{}
	for _, endpoint := range endpoints {
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

	return server, nil
}
