package btcclient

import (
	"github.com/babylonchain/vigilante/config"
	"os"
)

func readCAFile(cfg *config.BTCConfig) []byte {
	// Read certificate file if TLS is not disabled.
	if !cfg.DisableClientTLS {
		certs, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			log.Errorf("Cannot open CA file: %v", err)
			// If there's an error reading the CA file, continue
			// with nil certs and without the client connection.
			return nil
		}
		return certs
	} else {
		log.Infof("Chain server RPC TLS is disabled")
	}

	return nil
}

func readWalletCAFile(cfg *config.BTCConfig) []byte {
	// Read certificate file if TLS is not disabled.
	if !cfg.DisableClientTLS {
		certs, err := os.ReadFile(cfg.WalletCAFile)
		if err != nil {
			log.Errorf("Cannot open wallet CA file in %v: %v", cfg.WalletCAFile, err)
			// If there's an error reading the CA file, continue
			// with nil certs and without the client connection.
			return nil
		}
		return certs
	} else {
		log.Infof("Chain server RPC TLS is disabled")
	}

	return nil
}
