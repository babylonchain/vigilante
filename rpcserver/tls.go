package rpcserver

import (
	"crypto/tls"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/btcsuite/btcd/btcutil"
)

// openRPCKeyPair creates or loads the RPC TLS keypair specified by the
// application config.  This function respects the cfg.OneTimeTLSKey setting.
func openRPCKeyPair(oneTimeTLSKey bool, RPCKeyFile string, RPCCertFile string) (tls.Certificate, error) {
	// Check for existence of the TLS key file.  If one time TLS keys are
	// enabled but a key already exists, this function should error since
	// it's possible that a persistent certificate was copied to a remote
	// machine.  Otherwise, generate a new keypair when the key is missing.
	// When generating new persistent keys, overwriting an existing cert is
	// acceptable if the previous execution used a one time TLS key.
	// Otherwise, both the cert and key should be read from disk.  If the
	// cert is missing, the read error will occur in LoadX509KeyPair.
	_, e := os.Stat(RPCKeyFile)
	keyExists := !os.IsNotExist(e)
	switch {
	case oneTimeTLSKey && keyExists:
		err := fmt.Errorf("one time TLS keys are enabled, but TLS key "+
			"`%s` already exists", RPCKeyFile)
		return tls.Certificate{}, err
	case oneTimeTLSKey:
		return generateRPCKeyPair(RPCKeyFile, RPCCertFile, false)
	case !keyExists:
		return generateRPCKeyPair(RPCKeyFile, RPCCertFile, true)
	default:
		return tls.LoadX509KeyPair(RPCCertFile, RPCKeyFile)
	}
}

// generateRPCKeyPair generates a new RPC TLS keypair and writes the cert and
// possibly also the key in PEM format to the paths specified by the config.  If
// successful, the new keypair is returned.
func generateRPCKeyPair(RPCKeyFile string, RPCCertFile string, writeKey bool) (tls.Certificate, error) {
	// Create directories for cert and key files if they do not yet exist.
	certDir, _ := filepath.Split(RPCCertFile)
	err := os.MkdirAll(certDir, 0700)
	if err != nil {
		return tls.Certificate{}, err
	}

	// Generate cert pair.
	org := "Babylon vigilante autogenerated cert"
	validUntil := time.Now().Add(time.Hour * 24 * 365 * 10)
	cert, key, err := btcutil.NewTLSCertPair(org, validUntil, nil)
	if err != nil {
		return tls.Certificate{}, err
	}
	keyPair, err := tls.X509KeyPair(cert, key)
	if err != nil {
		return tls.Certificate{}, err
	}

	// Write cert and (potentially) the key files.
	err = os.WriteFile(RPCCertFile, cert, 0600)
	if err != nil {
		return tls.Certificate{}, err
	}
	if writeKey {
		keyDir, _ := filepath.Split(RPCKeyFile)
		err = os.MkdirAll(keyDir, 0700)
		if err != nil {
			return tls.Certificate{}, err
		}
		err = os.WriteFile(RPCKeyFile, key, 0600)
		if err != nil {
			os.Remove(RPCCertFile) //nolint: errcheck
			return tls.Certificate{}, err
		}
	}

	return keyPair, nil
}
