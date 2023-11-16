package types

import (
	babylontypes "github.com/babylonchain/babylon/types"
	btcltypes "github.com/babylonchain/babylon/x/btclightclient/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func NewMsgInsertHeaders(
	prefix string,
	signer sdk.AccAddress,
	headers []*IndexedBlock,
) *btcltypes.MsgInsertHeaders {
	signerBech32 := sdk.MustBech32ifyAddressBytes(prefix, signer)

	headerBytes := make([]babylontypes.BTCHeaderBytes, len(headers))
	for i, h := range headers {
		header := h
		headerBytes[i] = babylontypes.NewBTCHeaderBytesFromBlockHeader(header.Header)
	}

	return &btcltypes.MsgInsertHeaders{
		Signer:  signerBech32,
		Headers: headerBytes,
	}
}
