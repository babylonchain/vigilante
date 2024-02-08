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
//	https://github.com/btcsuite/btcwallet/blob/master/rpc/documentation/api.md
//
// Any API changes must be performed according to the steps listed here:
//
//	https://github.com/btcsuite/btcwallet/blob/master/rpc/documentation/serverchanges.md
package rpcserver

import (
	"fmt"
	"net"

	"go.uber.org/zap"

	bst "github.com/babylonchain/vigilante/btcstaking-tracker"
	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/monitor"
	"github.com/babylonchain/vigilante/reporter"
	"github.com/babylonchain/vigilante/submitter"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
)

type Server struct {
	*grpc.Server
	Cfg               *config.GRPCConfig
	logger            *zap.SugaredLogger
	Submitter         *submitter.Submitter
	Reporter          *reporter.Reporter
	Monitor           *monitor.Monitor
	BTCStakingTracker *bst.BTCStakingTracker
}

func New(
	cfg *config.GRPCConfig,
	parentLogger *zap.Logger,
	submitter *submitter.Submitter,
	reporter *reporter.Reporter,
	monitor *monitor.Monitor,
	bstracker *bst.BTCStakingTracker,
) (*Server, error) {
	if submitter == nil && reporter == nil && monitor == nil && bstracker == nil {
		return nil, fmt.Errorf("at least one of submitter, reporter, and monitor should be non-empty")
	}
	logger := parentLogger.With(zap.String("module", "rpcserver")).Sugar()

	keyPair, err := openRPCKeyPair(cfg.OneTimeTLSKey, cfg.RPCKeyFile, cfg.RPCCertFile)
	if err != nil {
		return nil, fmt.Errorf("open RPC key pair: %v", err)
	}
	creds := credentials.NewServerTLSFromCert(&keyPair)

	server := grpc.NewServer(
		grpc.Creds(creds),
		grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(
			grpc_prometheus.StreamServerInterceptor,
		)),
		grpc.ChainUnaryInterceptor(grpc_middleware.ChainUnaryServer(
			grpc_prometheus.UnaryServerInterceptor,
		)),
	)
	reflection.Register(server)      // register reflection service
	StartVigilanteService(server)    // register our vigilante service
	grpc_prometheus.Register(server) // register Prometheus metrics service

	return &Server{server, cfg, logger, submitter, reporter, monitor, bstracker}, nil
}

func (s *Server) Start() {
	// create listeners for endpoints
	// TODO: negotiate API version
	listeners := []net.Listener{}
	for _, endpoint := range s.Cfg.Endpoints {
		lis, err := net.Listen("tcp", endpoint)
		if err != nil {
			s.logger.Errorf("Listen: %v", err)
		} else {
			listeners = append(listeners, lis)
		}
	}

	// start the server with listeners, each in a goroutine
	for _, lis := range listeners {
		go func(l net.Listener) {
			if err := s.Serve(l); err != nil {
				s.logger.Errorf("Serve RPC server: %v", err)
			}
		}(lis)
		s.logger.Infof("Successfully started the GRPC server at %v", lis.Addr().String())
	}
}
