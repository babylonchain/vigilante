package rpcserver

import (
	vlog "github.com/babylonchain/vigilante/log"
	"go.uber.org/zap"
)

var log = vlog.Logger.With(zap.String("module", "rpcserver")).Sugar()
