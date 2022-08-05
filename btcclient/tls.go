package btcclient

import (
	"io/ioutil"

	"github.com/babylonchain/vigilante/config"
	log "github.com/sirupsen/logrus"
)

func readCAFile(cfg *config.BTCConfig) []byte {
	// Read certificate file if TLS is not disabled.
	var certs []byte
	if !cfg.DisableClientTLS {
		var err error
		certs, err = ioutil.ReadFile(cfg.CAFile)
		if err != nil {
			log.Errorf("Cannot open CA file: %v", err)
			// If there's an error reading the CA file, continue
			// with nil certs and without the client connection.
			certs = nil
		}
	} else {
		log.Infof("Chain server RPC TLS is disabled")
	}

	return certs
}
