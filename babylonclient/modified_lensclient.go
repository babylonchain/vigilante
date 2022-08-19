package babylonclient

import (
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	lensclient "github.com/strangelove-ventures/lens/client"
)

// adapted from https://github.com/strangelove-ventures/lens/blob/v0.5.1/client/chain_client.go#L48-L63
// The notable difference is parsing the key directory
// TODO: key directory support for different types of keyring backend
func newLensClient(ccc *lensclient.ChainClientConfig, kro ...keyring.Option) (*lensclient.ChainClient, error) {
	// attach the supported algorithms to the keyring options
	keyringOptions := []keyring.Option{}
	keyringOptions = append(keyringOptions, func(options *keyring.Options) {
		options.SupportedAlgos = keyring.SigningAlgoList{hd.Secp256k1}
		options.SupportedAlgosLedger = keyring.SigningAlgoList{hd.Secp256k1}
	})
	keyringOptions = append(keyringOptions, kro...)

	cc := &lensclient.ChainClient{
		KeyringOptions: keyringOptions,
		Config:         ccc,
		Codec:          lensclient.MakeCodec(ccc.Modules),
	}
	if err := cc.Init(); err != nil {
		return nil, err
	}
	return cc, nil
}
