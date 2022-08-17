package babylonclient

import (
	babylontypes "github.com/babylonchain/babylon/types"
	btcltypes "github.com/babylonchain/babylon/x/btclightclient/types"
	"github.com/btcsuite/btcd/wire"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func NewMsgInsertHeader(signer sdk.AccAddress, header *wire.BlockHeader) *btcltypes.MsgInsertHeader {
	headerBytes := babylontypes.NewBTCHeaderBytesFromBlockHeader(header)
	return &btcltypes.MsgInsertHeader{
		Signer: signer.String(),
		Header: &headerBytes,
	}
}
