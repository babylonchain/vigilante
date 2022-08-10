package rpcserver

import (
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	pb "github.com/babylonchain/vigilante/rpcserver/api"
)

// Public API version constants
const (
	verString = "0.0.1"
	verMajor  = 0
	verMinor  = 0
	verPatch  = 1
)

type service struct{}

// StartVigilanteService creates an implementation of the VigilanteService and
// registers it with the gRPC server.
func StartVigilanteService(gs *grpc.Server) {
	pb.RegisterVigilanteServiceServer(gs, &service{})
}

func (s *service) Version(ctx context.Context, req *pb.VersionRequest) (*pb.VersionResponse, error) {
	return &pb.VersionResponse{
		VersionString: verString,
		Major:         verMajor,
		Minor:         verMinor,
		Patch:         verPatch,
	}, nil
}
