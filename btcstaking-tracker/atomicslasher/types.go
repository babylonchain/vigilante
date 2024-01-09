package atomicslasher

import (
	"fmt"
	"sync"

	asig "github.com/babylonchain/babylon/crypto/schnorr-adaptor-signature"
	bbn "github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

type slashingPath int

const (
	slashStakingTx slashingPath = iota
	slashUnbondingTx
)

type SlashingTxInfo struct {
	path          slashingPath
	StakingTxHash chainhash.Hash
	SlashingMsgTx *wire.MsgTx
}

func NewSlashingTxInfo(path slashingPath, stakingTxHash chainhash.Hash, slashingMsgTx *wire.MsgTx) *SlashingTxInfo {
	return &SlashingTxInfo{
		path:          path,
		StakingTxHash: stakingTxHash,
		SlashingMsgTx: slashingMsgTx,
	}
}

func (s *SlashingTxInfo) IsSlashStakingTx() bool {
	return s.path == slashStakingTx
}

type TrackedDelegation struct {
	StakingTxHash           chainhash.Hash
	SlashingTxHash          chainhash.Hash
	UnbondingSlashingTxHash chainhash.Hash
}

func NewTrackedBTCDelegation(btcDel *bstypes.BTCDelegation) (*TrackedDelegation, error) {
	stakingTxHash, err := btcDel.GetStakingTxHash()
	if err != nil {
		return nil, err
	}
	slashingTx, err := btcDel.SlashingTx.ToMsgTx()
	if err != nil {
		return nil, err
	}
	slashingTxHash := slashingTx.TxHash()
	unbondingSlashingTx, err := btcDel.BtcUndelegation.SlashingTx.ToMsgTx()
	if err != nil {
		return nil, err
	}
	unbondingSlashingTxHash := unbondingSlashingTx.TxHash()
	return &TrackedDelegation{
		StakingTxHash:           stakingTxHash,
		SlashingTxHash:          slashingTxHash,
		UnbondingSlashingTxHash: unbondingSlashingTxHash,
	}, nil
}

// BTCDelegationIndex is an index of BTC delegations that support efficient
// search of BTC delegations by using staking/slashing/unbondingslashing tx
type BTCDelegationIndex struct {
	sync.Mutex

	// staking tx hash -> tracked BTC delegation
	delMap map[chainhash.Hash]*TrackedDelegation
	// slashing tx hash -> staking tx hash of the corresponding BTC delegation
	slashingTxMap map[chainhash.Hash]chainhash.Hash
	// unbonding slashing tx hash -> staking tx hash of the corresponding BTC delegation
	unbondingSlashingTxMap map[chainhash.Hash]chainhash.Hash
}

// NewBTCDelegationIndex returns an empty BTC delegation index
func NewBTCDelegationIndex() *BTCDelegationIndex {
	return &BTCDelegationIndex{
		delMap:                 make(map[chainhash.Hash]*TrackedDelegation),
		slashingTxMap:          make(map[chainhash.Hash]chainhash.Hash),
		unbondingSlashingTxMap: make(map[chainhash.Hash]chainhash.Hash),
	}
}

// Get returns the tracked delegation for the given staking tx hash or nil if not found.
func (bdi *BTCDelegationIndex) Get(stakingTxHash chainhash.Hash) *TrackedDelegation {
	bdi.Lock()
	defer bdi.Unlock()

	del, ok := bdi.delMap[stakingTxHash]
	if !ok {
		return nil
	}

	return del
}

func (bdi *BTCDelegationIndex) Add(trackedDel *TrackedDelegation) {
	bdi.Lock()
	defer bdi.Unlock()

	// ensure the BTC delegation is not known yet
	if _, ok := bdi.delMap[trackedDel.StakingTxHash]; ok {
		return
	}

	// at this point, the BTC delegation is not known, add it
	bdi.delMap[trackedDel.StakingTxHash] = trackedDel
	// track slashing tx and unbonding slashing tx
	bdi.slashingTxMap[trackedDel.SlashingTxHash] = trackedDel.StakingTxHash
	bdi.unbondingSlashingTxMap[trackedDel.UnbondingSlashingTxHash] = trackedDel.StakingTxHash
}

func (bdi *BTCDelegationIndex) Remove(stakingTxHash chainhash.Hash) {
	bdi.Lock()
	defer bdi.Unlock()

	del, ok := bdi.delMap[stakingTxHash]
	if !ok {
		return
	}

	delete(bdi.delMap, stakingTxHash)
	delete(bdi.slashingTxMap, del.SlashingTxHash)
	delete(bdi.unbondingSlashingTxMap, del.UnbondingSlashingTxHash)
}

func (bdi *BTCDelegationIndex) FindSlashedBTCDelegation(txHash chainhash.Hash) (*TrackedDelegation, slashingPath) {
	bdi.Lock()
	defer bdi.Unlock()

	if stakingTxHash, ok := bdi.slashingTxMap[txHash]; ok {
		return bdi.delMap[stakingTxHash], slashStakingTx
	}

	if stakingTxHash, ok := bdi.unbondingSlashingTxMap[txHash]; ok {
		return bdi.delMap[stakingTxHash], slashUnbondingTx
	}

	return nil, -1
}

// parseSlashingTxWitness is a private helper function for extracting
// the PK/signature pairs of covenant members and the index of the
// slashed finality provider who signs the slashing tx
// TODO: fuzz test
func parseSlashingTxWitness(
	witnessStack wire.TxWitness,
	covPKs []bbn.BIP340PubKey,
	fpPKs []bbn.BIP340PubKey,
) (map[string]*bbn.BIP340Signature, int, *bbn.BIP340PubKey, error) {
	// sort covenant PKs and finality provider PKs as per the tx script structure
	orderedCovPKs := bbn.SortBIP340PKs(covPKs)
	orderedfpPKs := bbn.SortBIP340PKs(fpPKs)

	// decode covenant signatures
	covSigMap := make(map[string]*bbn.BIP340Signature)
	covWitnessStack := witnessStack[0:len(orderedCovPKs)]
	for i := range covWitnessStack {
		if len(covWitnessStack[i]) == 0 {
			// this covenant member does not sign, skip
			continue
		}
		sig, err := bbn.NewBIP340Signature(covWitnessStack[i])
		if err != nil {
			return nil, 0, nil, err
		}
		covSigMap[covPKs[i].MarshalHex()] = sig
	}

	// decode finality provider signatures
	var fpIdx int
	var fpPK *bbn.BIP340PubKey
	fpWitnessStack := witnessStack[len(orderedCovPKs) : len(orderedCovPKs)+len(orderedfpPKs)]
	for i := range fpWitnessStack {
		if len(fpWitnessStack[i]) != 0 {
			fpIdx = i
			fpPK = &fpPKs[i]
			break
		}
	}

	return covSigMap, fpIdx, fpPK, nil
}

// TODO: fuzz test
func tryExtractFPSK(
	covSigMap map[string]*bbn.BIP340Signature,
	fpIdx int,
	fpPK *bbn.BIP340PubKey,
	covASigLists []*bstypes.CovenantAdaptorSignatures,
) (*btcec.PrivateKey, error) {
	// try to recover
	for _, covASigList := range covASigLists {
		// find the covenant Schnorr signature in the map
		covSchnorrSig, ok := covSigMap[covASigList.CovPk.MarshalHex()]
		if !ok {
			// this covenant member does not sign adaptor signature
			continue
		}

		// find the covenant adaptor signature
		covSchnorrBTCSig, err := covSchnorrSig.ToBTCSig()
		if err != nil {
			return nil, err
		}
		covAdaptorSigBytes := covASigList.AdaptorSigs[fpIdx]
		covAdaptorSig, err := asig.NewAdaptorSignatureFromBytes(covAdaptorSigBytes)
		if err != nil {
			return nil, err
		}
		// try to recover the finality provider SK
		fpSK := covAdaptorSig.Recover(covSchnorrBTCSig).ToBTCSK()
		actualfpPK := bbn.NewBIP340PubKeyFromBTCPK(fpSK.PubKey())
		if !fpPK.Equals(actualfpPK) {
			// this covenant member must have colluded with finality provider and
			// signed a new Schnorr signature. try the next covenant member
			// TODO: decide what to do here
			continue
		}
		// successfully extracted finality provider's SK, return
		return fpSK, nil
	}

	// at this point, all signed covenant members must have colluded and
	// signed Schnorr signatures for the slashing tx
	return nil, fmt.Errorf("failed to extract finality provider's SK")
}
