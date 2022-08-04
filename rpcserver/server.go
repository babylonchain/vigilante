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
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	pb "github.com/babylonchain/vigilante/rpcserver/api"
)

// Public API version constants
const (
	verString = "2.0.1"
	verMajor  = 2
	verMinor  = 0
	verPatch  = 1
)

type server struct{}

// StartVigilanteService creates an implementation of the VigilanteService and
// registers it with the gRPC server.
func StartVigilanteService(gs *grpc.Server) {
	pb.RegisterVigilanteServiceServer(gs, &server{})
}

func (*server) Version(ctx context.Context, req *pb.VersionRequest) (*pb.VersionResponse, error) {
	return &pb.VersionResponse{
		VersionString: verString,
		Major:         verMajor,
		Minor:         verMinor,
		Patch:         verPatch,
	}, nil
}
