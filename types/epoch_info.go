package types

import (
	"bytes"
	"fmt"
	"github.com/babylonchain/babylon/crypto/bls12381"
	ckpttypes "github.com/babylonchain/babylon/x/checkpointing/types"
	"github.com/boljen/go-bitmap"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/pkg/errors"
)

// EpochInfo maintains information for a specific epoch from Babylon
type EpochInfo struct {
	epochNum uint64
	valSet   ckpttypes.ValidatorWithBlsKeySet
}

func NewEpochInfo(epochNum uint64, valSet ckpttypes.ValidatorWithBlsKeySet) *EpochInfo {
	return &EpochInfo{
		epochNum: epochNum,
		valSet:   valSet,
	}
}

// GetSignersKeySetWithPowerSum returns the signer BLS key set and the sum of the voting power
// based the given bitmap
func (ei *EpochInfo) GetSignersKeySetWithPowerSum(bm bitmap.Bitmap) ([]bls12381.PublicKey, uint64, error) {
	signers, powerSum, err := ei.valSet.FindSubsetWithPowerSum(bm)
	if err != nil {
		return nil, 0, err
	}

	return signers.GetBLSKeySet(), powerSum, nil
}

func (ei *EpochInfo) GetEpochNumber() uint64 {
	return ei.epochNum
}

func (ei *EpochInfo) GetTotalPower() uint64 {
	return ei.valSet.GetTotalPower()
}

func (ei *EpochInfo) Equal(epochInfo *EpochInfo) bool {
	if ei.epochNum != epochInfo.epochNum {
		return false
	}
	for i, val := range ei.valSet.ValSet {
		val1 := epochInfo.valSet.ValSet[i]
		if val.ValidatorAddress != val1.ValidatorAddress {
			return false
		}
		if !bytes.Equal(val.BlsPubKey, val1.BlsPubKey) {
			return false
		}
		if val.VotingPower != val1.VotingPower {
			return false
		}
	}
	return true
}

// VerifyMultiSig verifies the multi-sig of a given checkpoint using BLS public keys
func (ei *EpochInfo) VerifyMultiSig(ckpt *ckpttypes.RawCheckpoint) error {
	signerKeySet, sumPower, err := ei.GetSignersKeySetWithPowerSum(ckpt.Bitmap)
	if sumPower <= ei.GetTotalPower()*1/3 {
		return errors.Wrapf(ErrInsufficientPower, fmt.Sprintf("expected to be greater than %v, got %v", ei.GetTotalPower()*1/3, sumPower))
	}
	if err != nil {
		return errors.Wrapf(ErrInvalidMultiSig, fmt.Sprintf("failed to get signer set: %s", err.Error()))
	}
	msgBytes := GetMsgBytes(ckpt.EpochNum, ckpt.AppHash)
	valid, err := bls12381.VerifyMultiSig(*ckpt.BlsMultiSig, signerKeySet, msgBytes)
	if !valid || err != nil {
		return ErrInvalidMultiSig
	}
	return nil
}

func GetMsgBytes(epoch uint64, lch *ckpttypes.AppHash) []byte {
	return append(sdk.Uint64ToBigEndian(epoch), lch.MustMarshal()...)
}
