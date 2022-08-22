package types

import (
	babylontypes "github.com/babylonchain/babylon/types"
	btcltypes "github.com/babylonchain/babylon/x/btclightclient/types"
	"github.com/btcsuite/btcd/wire"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func NewMsgInsertHeader(prefix string, signer sdk.AccAddress, header *wire.BlockHeader) *btcltypes.MsgInsertHeader {
	signerBech32 := sdk.MustBech32ifyAddressBytes(prefix, signer)
	headerBytes := babylontypes.NewBTCHeaderBytesFromBlockHeader(header)
	return &btcltypes.MsgInsertHeader{
		Signer: signerBech32, // TODO: avoid using accAddress.String() everywhere (including babylonchain/babylon) since it uses the default prefix of Cosmos SDK
		Header: &headerBytes,
	}
}
