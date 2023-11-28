package zmq

import (
	vlog "github.com/babylonchain/vigilante/log"
	"go.uber.org/zap"
)

var log = vlog.Logger.With(zap.String("module", "zmq")).Sugar()
