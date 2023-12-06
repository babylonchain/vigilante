package unbondingwatcher

import (
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

type Delegation struct {
	StakingTx             *wire.MsgTx
	StakingOutputIdx      uint32
	DelegationStartHeight uint64
	UnbondingOutput       *wire.TxOut
}

type BabylonNodeAdapter interface {
	ActiveBtcDelegations(offset uint64, limit uint64) ([]Delegation, error)
	IsDelegationActive(stakingTxHash chainhash.Hash) (bool, error)
	ReportUnbonding(stakingTxHash chainhash.Hash, stakerUnbondingSig *schnorr.Signature) error
	BtcClientTipHeight() (uint32, error)
}
