package babylonclient

import (
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	lensclient "github.com/strangelove-ventures/lens/client"
	ethhd "github.com/tharsis/ethermint/crypto/hd"
)

// adapted from https://github.com/strangelove-ventures/lens/blob/main/client/chain_client.go#L48-L63
func newLensClient(ccc *lensclient.ChainClientConfig, kro ...keyring.Option) (*lensclient.ChainClient, error) {
	cc := &lensclient.ChainClient{
		KeyringOptions: append([]keyring.Option{ethhd.EthSecp256k1Option()}, kro...),
		Config:         ccc,
		Codec:          lensclient.MakeCodec(ccc.Modules),
	}
	if err := cc.Init(); err != nil {
		return nil, err
	}
	return cc, nil
}
