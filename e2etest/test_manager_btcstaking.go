package e2etest

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2/schnorr"

	sdkmath "cosmossdk.io/math"
	"github.com/babylonchain/babylon/btcstaking"
	"github.com/babylonchain/babylon/crypto/eots"
	asig "github.com/babylonchain/babylon/crypto/schnorr-adaptor-signature"
	"github.com/babylonchain/babylon/testutil/datagen"
	bbn "github.com/babylonchain/babylon/types"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	ftypes "github.com/babylonchain/babylon/x/finality/types"
	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
)

var (
	r = rand.New(rand.NewSource(time.Now().Unix()))

	// covenant
	covenantSk, _ = btcec.PrivKeyFromBytes(
		[]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
	)
)

func (tm *TestManager) getBTCUnbondingTime(t *testing.T) uint64 {
	bsParams, err := tm.BabylonClient.BTCStakingParams()
	require.NoError(t, err)
	btccParams, err := tm.BabylonClient.BTCCheckpointParams()
	require.NoError(t, err)

	return bstypes.MinimumUnbondingTime(bsParams.Params, btccParams.Params) + 1
}

func (tm *TestManager) CreateFinalityProvider(t *testing.T) (*bstypes.FinalityProvider, *btcec.PrivateKey) {
	var err error
	signerAddr := tm.BabylonClient.MustGetAddr()

	fpSK, _, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)
	btcFp, err := datagen.GenRandomFinalityProviderWithBTCSK(r, fpSK)
	require.NoError(t, err)

	/*
		create finality provider
	*/
	commission := sdkmath.LegacyZeroDec()
	msgNewVal := &bstypes.MsgCreateFinalityProvider{
		Signer:      signerAddr,
		Description: &stakingtypes.Description{Moniker: datagen.GenRandomHexStr(r, 10)},
		Commission:  &commission,
		BabylonPk:   btcFp.BabylonPk,
		BtcPk:       btcFp.BtcPk,
		Pop:         btcFp.Pop,
	}
	_, err = tm.BabylonClient.ReliablySendMsg(context.Background(), msgNewVal, nil, nil)
	require.NoError(t, err)

	return btcFp, fpSK
}

func (tm *TestManager) CreateBTCDelegation(
	t *testing.T,
	fpSK *btcec.PrivateKey,
) (*datagen.TestStakingSlashingInfo, *datagen.TestUnbondingSlashingInfo, *btcec.PrivateKey) {
	signerAddr := tm.BabylonClient.MustGetAddr()

	fpPK := fpSK.PubKey()

	/*
		create BTC delegation
	*/
	// generate staking tx and slashing tx
	bsParams, err := tm.BabylonClient.BTCStakingParams()
	require.NoError(t, err)
	covenantBtcPks, err := bbnPksToBtcPks(bsParams.Params.CovenantPks)
	require.NoError(t, err)
	stakingTimeBlocks := uint16(math.MaxUint16)
	// get top UTXO
	topUnspentResult, _, err := tm.BTCWalletClient.GetHighUTXOAndSum()
	require.NoError(t, err)
	topUTXO, err := types.NewUTXO(topUnspentResult, netParams)
	require.NoError(t, err)
	// staking value
	stakingValue := int64(topUTXO.Amount) / 3
	// dump SK
	wif, err := tm.BTCWalletClient.DumpPrivKey(topUTXO.Addr)
	require.NoError(t, err)

	// generate legitimate BTC del
	stakingSlashingInfo := datagen.GenBTCStakingSlashingInfoWithOutPoint(
		r,
		t,
		netParams,
		topUTXO.GetOutPoint(),
		wif.PrivKey,
		[]*btcec.PublicKey{fpPK},
		covenantBtcPks,
		bsParams.Params.CovenantQuorum,
		stakingTimeBlocks,
		stakingValue,
		bsParams.Params.SlashingAddress,
		bsParams.Params.SlashingRate,
		uint16(tm.getBTCUnbondingTime(t)),
	)
	// sign staking tx and overwrite the staking tx to the signed version
	// NOTE: the tx hash has changed here since stakingMsgTx is pre-segwit
	stakingMsgTx, signed, err := tm.BTCWalletClient.SignRawTransaction(stakingSlashingInfo.StakingTx)
	require.NoError(t, err)
	require.True(t, signed)
	// overwrite staking tx
	stakingSlashingInfo.StakingTx = stakingMsgTx
	// get signed staking tx hash
	stakingMsgTxHash1 := stakingSlashingInfo.StakingTx.TxHash()
	stakingMsgTxHash := &stakingMsgTxHash1
	t.Logf("signed staking tx hash: %s", stakingMsgTxHash.String())

	// change outpoint tx hash of slashing tx to the txhash of the signed staking tx
	slashingMsgTx, err := stakingSlashingInfo.SlashingTx.ToMsgTx()
	require.NoError(t, err)
	slashingMsgTx.TxIn[0].PreviousOutPoint.Hash = stakingSlashingInfo.StakingTx.TxHash()
	// update slashing tx
	stakingSlashingInfo.SlashingTx, err = bstypes.NewBTCSlashingTxFromMsgTx(slashingMsgTx)
	require.NoError(t, err)

	// send staking tx to Bitcoin node's mempool
	_, err = tm.BTCWalletClient.SendRawTransaction(stakingMsgTx, true)
	require.NoError(t, err)

	// mine a block with this tx, and insert it to Bitcoin / Babylon
	mBlock := tm.MineBlockWithTxs(t, tm.RetrieveTransactionFromMempool(t, []*chainhash.Hash{stakingMsgTxHash}))
	require.Equal(t, 2, len(mBlock.Transactions))
	// wait until staking tx is on Bitcoin
	require.Eventually(t, func() bool {
		_, err := tm.BTCClient.GetRawTransaction(stakingMsgTxHash)
		return err == nil
	}, eventuallyWaitTimeOut, eventuallyPollTime)
	// get spv proof of the BTC staking tx
	stakingTxInfo := getTxInfo(t, mBlock, 1)

	// insert k empty blocks to Bitcoin
	btccParamsResp, err := tm.BabylonClient.BTCCheckpointParams()
	require.NoError(t, err)
	btccParams := btccParamsResp.Params
	for i := 0; i < int(btccParams.BtcConfirmationDepth); i++ {
		tm.MineBlockWithTxs(t, tm.RetrieveTransactionFromMempool(t, []*chainhash.Hash{}))
	}

	stakingOutIdx, err := outIdx(stakingSlashingInfo.StakingTx, stakingSlashingInfo.StakingInfo.StakingOutput)
	require.NoError(t, err)

	// BTC delegation's Babylon key pair
	delBabylonSK, delBabylonPK, _ := datagen.GenRandomSecp256k1KeyPair(r)
	// create PoP
	pop, err := bstypes.NewPoP(delBabylonSK, wif.PrivKey)
	require.NoError(t, err)
	slashingSpendPath, err := stakingSlashingInfo.StakingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)
	// generate proper delegator sig
	require.NoError(t, err)
	delegatorSig, err := stakingSlashingInfo.SlashingTx.Sign(
		stakingMsgTx,
		stakingOutIdx,
		slashingSpendPath.GetPkScriptPath(),
		wif.PrivKey,
	)
	require.NoError(t, err)

	// Genearate all data necessary for unbonding
	fee := int64(1000)
	unbondingValue := stakingSlashingInfo.StakingInfo.StakingOutput.Value - fee
	unbondingSlashingInfo := datagen.GenBTCUnbondingSlashingInfo(
		r,
		t,
		netParams,
		wif.PrivKey,
		[]*btcec.PublicKey{fpPK},
		covenantBtcPks,
		bsParams.Params.CovenantQuorum,
		wire.NewOutPoint(stakingMsgTxHash, stakingOutIdx),
		stakingTimeBlocks,
		unbondingValue,
		bsParams.Params.SlashingAddress,
		bsParams.Params.SlashingRate,
		uint16(tm.getBTCUnbondingTime(t)),
	)
	require.NoError(t, err)
	unbondingTxBytes, err := bbn.SerializeBTCTx(unbondingSlashingInfo.UnbondingTx)
	require.NoError(t, err)

	unbondingSlashingPathSpendInfo, err := unbondingSlashingInfo.UnbondingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)
	slashingTxSig, err := unbondingSlashingInfo.SlashingTx.Sign(
		unbondingSlashingInfo.UnbondingTx,
		0, // Only one output in the unbonding tx
		unbondingSlashingPathSpendInfo.GetPkScriptPath(),
		wif.PrivKey,
	)
	require.NoError(t, err)

	// 	Build message to send
	tm.CatchUpBTCLightClient(t)

	// submit BTC delegation to Babylon
	msgBTCDel := &bstypes.MsgCreateBTCDelegation{
		Signer:               signerAddr,
		BabylonPk:            delBabylonPK.(*secp256k1.PubKey),
		Pop:                  pop,
		BtcPk:                bbn.NewBIP340PubKeyFromBTCPK(wif.PrivKey.PubKey()),
		FpBtcPkList:          []bbn.BIP340PubKey{*bbn.NewBIP340PubKeyFromBTCPK(fpPK)},
		StakingTime:          uint32(stakingTimeBlocks),
		StakingValue:         stakingValue,
		StakingTx:            stakingTxInfo,
		SlashingTx:           stakingSlashingInfo.SlashingTx,
		DelegatorSlashingSig: delegatorSig,
		// Ubonding related data
		UnbondingTime:                 uint32(tm.getBTCUnbondingTime(t)),
		UnbondingTx:                   unbondingTxBytes,
		UnbondingValue:                unbondingSlashingInfo.UnbondingInfo.UnbondingOutput.Value,
		UnbondingSlashingTx:           unbondingSlashingInfo.SlashingTx,
		DelegatorUnbondingSlashingSig: slashingTxSig,
	}
	_, err = tm.BabylonClient.ReliablySendMsg(context.Background(), msgBTCDel, nil, nil)
	require.NoError(t, err)
	t.Logf("submitted MsgCreateBTCDelegation")

	/*
		generate and insert new covenant signature, in order to activate the BTC delegation
	*/
	// TODO: Make this handle multiple covenant signatures
	fpEncKey, err := asig.NewEncryptionKeyFromBTCPK(fpSK.PubKey())
	require.NoError(t, err)
	covenantSig, err := stakingSlashingInfo.SlashingTx.EncSign(
		stakingMsgTx,
		stakingOutIdx,
		slashingSpendPath.GetPkScriptPath(),
		covenantSk,
		fpEncKey,
	)
	require.NoError(t, err)

	// TODO: Add covenant sigs for all covenants
	// add covenant sigs
	// covenant Schnorr sig on unbonding tx
	unbondingPathSpendInfo, err := stakingSlashingInfo.StakingInfo.UnbondingPathSpendInfo()
	require.NoError(t, err)
	unbondingTxCovenantSchnorrSig, err := btcstaking.SignTxWithOneScriptSpendInputStrict(
		unbondingSlashingInfo.UnbondingTx,
		stakingSlashingInfo.StakingTx,
		stakingOutIdx,
		unbondingPathSpendInfo.GetPkScriptPath(),
		covenantSk,
	)
	require.NoError(t, err)
	covenantUnbondingSig := bbn.NewBIP340SignatureFromBTCSig(unbondingTxCovenantSchnorrSig)
	// covenant adaptor sig on unbonding slashing tx
	require.NoError(t, err)
	covenantSlashingSig, err := unbondingSlashingInfo.SlashingTx.EncSign(
		unbondingSlashingInfo.UnbondingTx,
		0, // Only one output in the unbonding transaction
		unbondingSlashingPathSpendInfo.GetPkScriptPath(),
		covenantSk,
		fpEncKey,
	)
	require.NoError(t, err)
	msgAddCovenantSig := &bstypes.MsgAddCovenantSigs{
		Signer:                  signerAddr,
		Pk:                      bbn.NewBIP340PubKeyFromBTCPK(covenantSk.PubKey()),
		StakingTxHash:           stakingMsgTxHash.String(),
		SlashingTxSigs:          [][]byte{covenantSig.MustMarshal()},
		UnbondingTxSig:          covenantUnbondingSig,
		SlashingUnbondingTxSigs: [][]byte{covenantSlashingSig.MustMarshal()},
	}
	_, err = tm.BabylonClient.ReliablySendMsg(context.Background(), msgAddCovenantSig, nil, nil)
	require.NoError(t, err)
	t.Logf("submitted covenant signature")

	return stakingSlashingInfo, unbondingSlashingInfo, wif.PrivKey
}

func (tm *TestManager) Undelegate(
	t *testing.T,
	stakingSlashingInfo *datagen.TestStakingSlashingInfo,
	unbondingSlashingInfo *datagen.TestUnbondingSlashingInfo,
	delSK *btcec.PrivateKey) (*datagen.TestUnbondingSlashingInfo, *schnorr.Signature) {
	signerAddr := tm.BabylonClient.MustGetAddr()

	// TODO: This generates unbonding tx signature, move it to undelegate
	unbondingPathSpendInfo, err := stakingSlashingInfo.StakingInfo.UnbondingPathSpendInfo()
	require.NoError(t, err)

	// the only input to unbonding tx is the staking tx
	stakingOutIdx, err := outIdx(unbondingSlashingInfo.UnbondingTx, unbondingSlashingInfo.UnbondingInfo.UnbondingOutput)
	require.NoError(t, err)
	unbondingTxSchnorrSig, err := btcstaking.SignTxWithOneScriptSpendInputStrict(
		unbondingSlashingInfo.UnbondingTx,
		stakingSlashingInfo.StakingTx,
		stakingOutIdx,
		unbondingPathSpendInfo.GetPkScriptPath(),
		delSK,
	)
	require.NoError(t, err)

	msgUndel := &bstypes.MsgBTCUndelegate{
		Signer:         signerAddr,
		StakingTxHash:  stakingSlashingInfo.StakingTx.TxHash().String(),
		UnbondingTxSig: bbn.NewBIP340SignatureFromBTCSig(unbondingTxSchnorrSig),
	}
	_, err = tm.BabylonClient.ReliablySendMsg(context.Background(), msgUndel, nil, nil)
	require.NoError(t, err)
	t.Logf("submitted MsgBTCUndelegate")

	resp, err := tm.BabylonClient.BTCDelegation(stakingSlashingInfo.StakingTx.TxHash().String())
	require.NoError(t, err)
	covenantSigs := resp.BtcDelegation.UndelegationResponse.CovenantUnbondingSigList
	witness, err := unbondingPathSpendInfo.CreateUnbondingPathWitness(
		[]*schnorr.Signature{covenantSigs[0].Sig.MustToBTCSig()},
		unbondingTxSchnorrSig,
	)
	require.NoError(t, err)
	unbondingSlashingInfo.UnbondingTx.TxIn[0].Witness = witness

	// send unbonding tx to Bitcoin node's mempool
	unbondingTxHash, err := tm.BTCWalletClient.SendRawTransaction(unbondingSlashingInfo.UnbondingTx, true)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		_, err := tm.BTCClient.GetRawTransaction(unbondingTxHash)
		return err == nil
	}, eventuallyWaitTimeOut, eventuallyPollTime)
	t.Logf("submitted unbonding tx with hash %s", unbondingTxHash.String())
	// mine a block with this tx, and insert it to Bitcoin
	mBlock := tm.MineBlockWithTxs(t, tm.RetrieveTransactionFromMempool(t, []*chainhash.Hash{unbondingTxHash}))
	require.Equal(t, 2, len(mBlock.Transactions))
	// wait until unbonding tx is on Bitcoin
	require.Eventually(t, func() bool {
		resp, err := tm.BTCClient.GetRawTransactionVerbose(unbondingTxHash)
		if err != nil {
			t.Logf("err of GetRawTransactionVerbose: %v", err)
			return false
		}
		return len(resp.BlockHash) > 0
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	return unbondingSlashingInfo, unbondingTxSchnorrSig
}

func (tm *TestManager) VoteAndEquivocate(t *testing.T, fpSK *btcec.PrivateKey) {
	signerAddr := tm.BabylonClient.MustGetAddr()

	// get the finality provider
	fpBTCPK := bbn.NewBIP340PubKeyFromBTCPK(fpSK.PubKey())
	fpResp, err := tm.BabylonClient.FinalityProvider(fpBTCPK.MarshalHex())
	require.NoError(t, err)
	btcFp := fpResp.FinalityProvider

	/*
		commit a number of public randomness since activatedHeight
	*/
	// commit public randomness list
	require.Eventually(t, func() bool {
		// need to wait for activatedHeight to not return error
		_, err := tm.BabylonClient.ActivatedHeight()
		return err == nil
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	activatedHeightResp, err := tm.BabylonClient.ActivatedHeight()
	require.NoError(t, err)
	activatedHeight := activatedHeightResp.Height
	srList, msgCommitPubRandList, err := datagen.GenRandomMsgCommitPubRandList(r, fpSK, activatedHeight, 100)
	require.NoError(t, err)
	msgCommitPubRandList.Signer = signerAddr
	_, err = tm.BabylonClient.ReliablySendMsg(context.Background(), msgCommitPubRandList, nil, nil)
	require.NoError(t, err)
	t.Logf("committed public randomness")

	/*
		submit finality signature
	*/
	// get block to vote
	blockToVote, err := tm.BabylonClient.GetBlock(int64(activatedHeight))
	require.NoError(t, err)
	msgToSign := append(sdk.Uint64ToBigEndian(activatedHeight), blockToVote.Block.AppHash...)
	// generate EOTS signature
	sig, err := eots.Sign(fpSK, srList.SRList[0], msgToSign)
	require.NoError(t, err)
	eotsSig := bbn.NewSchnorrEOTSSigFromModNScalar(sig)
	// submit finality signature
	msgAddFinalitySig := &ftypes.MsgAddFinalitySig{
		Signer:       signerAddr,
		FpBtcPk:      btcFp.BtcPk,
		BlockHeight:  activatedHeight,
		PubRand:      &srList.PRList[0],
		Proof:        srList.ProofList[0].ToProto(),
		BlockAppHash: blockToVote.Block.AppHash,
		FinalitySig:  eotsSig,
	}
	_, err = tm.BabylonClient.ReliablySendMsg(context.Background(), msgAddFinalitySig, nil, nil)
	require.NoError(t, err)
	t.Logf("submitted finality signature")

	/*
		equivocate
	*/
	invalidAppHash := datagen.GenRandomByteArray(r, 32)
	invalidMsgToSign := append(sdk.Uint64ToBigEndian(activatedHeight), invalidAppHash...)
	invalidSig, err := eots.Sign(fpSK, srList.SRList[0], invalidMsgToSign)
	require.NoError(t, err)
	invalidEotsSig := bbn.NewSchnorrEOTSSigFromModNScalar(invalidSig)
	invalidMsgAddFinalitySig := &ftypes.MsgAddFinalitySig{
		Signer:       signerAddr,
		FpBtcPk:      btcFp.BtcPk,
		BlockHeight:  activatedHeight,
		PubRand:      &srList.PRList[0],
		Proof:        srList.ProofList[0].ToProto(),
		BlockAppHash: invalidAppHash,
		FinalitySig:  invalidEotsSig,
	}
	_, err = tm.BabylonClient.ReliablySendMsg(context.Background(), invalidMsgAddFinalitySig, nil, nil)
	require.NoError(t, err)
	t.Logf("submitted equivocating finality signature")
}

func getTxInfo(t *testing.T, block *wire.MsgBlock, txIdx uint) *btcctypes.TransactionInfo {
	mHeaderBytes := bbn.NewBTCHeaderBytesFromBlockHeader(&block.Header)
	var txBytes [][]byte
	for _, tx := range block.Transactions {
		buf := bytes.NewBuffer(make([]byte, 0, tx.SerializeSize()))
		_ = tx.Serialize(buf)
		txBytes = append(txBytes, buf.Bytes())
	}
	spvProof, err := btcctypes.SpvProofFromHeaderAndTransactions(&mHeaderBytes, txBytes, 1)
	require.NoError(t, err)
	return btcctypes.NewTransactionInfoFromSpvProof(spvProof)
}

// TODO: these functions should be enabled by Babylon
func bbnPksToBtcPks(pks []bbn.BIP340PubKey) ([]*btcec.PublicKey, error) {
	btcPks := make([]*btcec.PublicKey, 0, len(pks))
	for _, pk := range pks {
		btcPk, err := pk.ToBTCPK()
		if err != nil {
			return nil, err
		}
		btcPks = append(btcPks, btcPk)
	}
	return btcPks, nil
}

func outIdx(tx *wire.MsgTx, candOut *wire.TxOut) (uint32, error) {
	for idx, out := range tx.TxOut {
		if bytes.Equal(out.PkScript, candOut.PkScript) && out.Value == candOut.Value {
			return uint32(idx), nil
		}
	}
	return 0, fmt.Errorf("couldn't find output")
}
