package utils

import (
	"io/ioutil"

	"github.com/babylonchain/vigilante/btcclient"
	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/netparams"
	log "github.com/sirupsen/logrus"
)

func NewBTCClient(cfg *config.BTCConfig) (*btcclient.Client, error) {
	certs := readCAFile(cfg)
	params := netparams.GetParams(cfg.NetParams)
	// TODO: parameterise the reconnect attempts?
	client, err := btcclient.New(params.Params, cfg.Endpoint, cfg.Username, cfg.Password, certs, cfg.DisableClientTLS, 3)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func BTCClientConnectLoop(cfg *config.BTCConfig, client *btcclient.Client) {
	go func() {
		log.Infof("Attempting RPC client connection to %v", cfg.Endpoint)
		if err := client.Start(); err != nil {
			log.Errorf("Unable to open connection to consensus RPC server: %v", err)
		}
		client.WaitForShutdown()
	}()
}

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
