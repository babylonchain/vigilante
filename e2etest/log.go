package e2etest

import (
	"github.com/babylonchain/vigilante/config"
)

var (
	logger, _ = config.NewRootLogger("auto", "debug")
	log       = logger.Sugar()
)
