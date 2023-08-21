package relayer

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/babylonchain/babylon/btctxformatter"
	ckpttypes "github.com/babylonchain/babylon/x/checkpointing/types"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/mempool"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcwallet/wallet/txrules"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/jinzhu/copier"
	"github.com/lightningnetwork/lnd/lnwallet/chainfee"

	"github.com/babylonchain/vigilante/btcclient"
	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/log"
	"github.com/babylonchain/vigilante/metrics"
	"github.com/babylonchain/vigilante/types"
)

type Relayer struct {
	chainfee.Estimator
	btcclient.BTCWallet
	lastSubmittedCheckpoint *types.CheckpointInfo
	tag                     btctxformatter.BabylonTag
	version                 btctxformatter.FormatVersion
	submitterAddress        sdk.AccAddress
	metrics                 *metrics.RelayerMetrics
	config                  *config.SubmitterConfig
}

func New(
	wallet btcclient.BTCWallet,
	tag btctxformatter.BabylonTag,
	version btctxformatter.FormatVersion,
	submitterAddress sdk.AccAddress,
	metrics *metrics.RelayerMetrics,
	est chainfee.Estimator,
	config *config.SubmitterConfig,
) *Relayer {
	metrics.ResendIntervalSecondsGauge.Set(float64(config.ResendIntervalSeconds))
	return &Relayer{
		Estimator:        est,
		BTCWallet:        wallet,
		tag:              tag,
		version:          version,
		submitterAddress: submitterAddress,
		metrics:          metrics,
		config:           config,
	}
}

// SendCheckpointToBTC converts the checkpoint into two transactions and send them to BTC
// if the checkpoint has been sent but the status is still Sealed, we will bump the fee
// of the second tx of the checkpoint and resend the tx
// Note: we only consider bumping the second tx of a submitted checkpoint because
// it is as effective as bumping the two but simpler
func (rl *Relayer) SendCheckpointToBTC(ckpt *ckpttypes.RawCheckpointWithMeta) error {
	ckptEpoch := ckpt.Ckpt.EpochNum
	if ckpt.Status != ckpttypes.Sealed {
		log.Logger.Errorf("The checkpoint for epoch %v is not sealed", ckptEpoch)
		rl.metrics.InvalidCheckpointCounter.Inc()
		// we do not consider this case as a failed submission but a software bug
		return nil
	}

	if rl.lastSubmittedCheckpoint == nil || rl.lastSubmittedCheckpoint.Epoch < ckptEpoch {
		log.Logger.Infof("Submitting a raw checkpoint for epoch %v for the first time", ckptEpoch)

		submittedCheckpoint, err := rl.convertCkptToTwoTxAndSubmit(ckpt)
		if err != nil {
			return err
		}

		rl.lastSubmittedCheckpoint = submittedCheckpoint

		return nil
	}

	lastSubmittedEpoch := rl.lastSubmittedCheckpoint.Epoch
	if ckptEpoch < lastSubmittedEpoch {
		log.Logger.Errorf("The checkpoint for epoch %v is lower than the last submission for epoch %v",
			ckptEpoch, lastSubmittedEpoch)
		rl.metrics.InvalidCheckpointCounter.Inc()
		// we do not consider this case as a failed submission but a software bug
		return nil
	}

	// now that the checkpoint has been sent, we should try to resend it
	// if the resend interval has passed
	durSeconds := uint(time.Since(rl.lastSubmittedCheckpoint.Ts).Seconds())
	if durSeconds >= rl.config.ResendIntervalSeconds {
		log.Logger.Debugf("The checkpoint for epoch %v was sent more than %v seconds ago but not included on BTC",
			ckptEpoch, rl.config.ResendIntervalSeconds)

		bumpedFee := rl.calculateBumpedFee(rl.lastSubmittedCheckpoint)

		// make sure the bumped fee is effective
		if !rl.shouldResendCheckpoint(rl.lastSubmittedCheckpoint, bumpedFee) {
			return nil
		}

		log.Logger.Debugf("Resending the second tx of the checkpoint %v, old fee of the second tx: %v Satoshis, txid: %s",
			ckptEpoch, rl.lastSubmittedCheckpoint.Tx2.Fee, rl.lastSubmittedCheckpoint.Tx2.TxId.String())

		resubmittedTx2, err := rl.resendSecondTxOfCheckpointToBTC(rl.lastSubmittedCheckpoint.Tx2, bumpedFee)
		if err != nil {
			rl.metrics.FailedResentCheckpointsCounter.Inc()
			return fmt.Errorf("failed to re-send the second tx of the checkpoint %v: %w", rl.lastSubmittedCheckpoint.Epoch, err)
		}

		// record the metrics of the resent tx2
		rl.metrics.NewSubmittedCheckpointSegmentGaugeVec.WithLabelValues(
			strconv.Itoa(int(ckptEpoch)),
			"1",
			resubmittedTx2.TxId.String(),
			strconv.Itoa(int(resubmittedTx2.Fee)),
		).SetToCurrentTime()
		rl.metrics.ResentCheckpointsCounter.Inc()

		log.Logger.Infof("Successfully re-sent the second tx of the checkpoint %v, txid: %s, bumped fee: %v Satoshis",
			rl.lastSubmittedCheckpoint.Epoch, resubmittedTx2.TxId.String(), resubmittedTx2.Fee)

		// update the second tx of the last submitted checkpoint as it is replaced
		rl.lastSubmittedCheckpoint.Tx2 = resubmittedTx2
	}

	return nil
}

// shouldResendCheckpoint checks whether the bumpedFee is effective for replacement
func (rl *Relayer) shouldResendCheckpoint(ckptInfo *types.CheckpointInfo, bumpedFee uint64) bool {
	// if the bumped fee is less than the fee of the previous second tx plus the minimum required bumping fee
	// then the bumping would not be effective
	requiredBumpingFee := ckptInfo.Tx2.Fee + rl.calcMinRequiredTxReplacementFee(ckptInfo.Tx2.Size)

	log.Logger.Debugf("the bumped fee: %v Satoshis, the required fee: %v Satoshis",
		bumpedFee, requiredBumpingFee)

	return bumpedFee >= requiredBumpingFee
}

// calculateBumpedFee calculates the bumped fees of the second tx of the checkpoint
// based on the current BTC load, considering both tx sizes
// the result is multiplied by ResubmitFeeMultiplier set in config
func (rl *Relayer) calculateBumpedFee(ckptInfo *types.CheckpointInfo) uint64 {
	tx1Size := int(ckptInfo.Tx1.Size)
	tx2Size := int(ckptInfo.Tx2.Size)

	feeRate := btcutil.Amount(rl.getFeeRate())
	newTx1Fee := uint64(txrules.FeeForSerializeSize(feeRate, tx1Size))
	newTx2Fee := uint64(txrules.FeeForSerializeSize(feeRate, tx2Size))
	// minus the old fee of the first transaction because we do not want to pay again for the first transaction
	return uint64(float64(newTx1Fee+newTx2Fee-ckptInfo.Tx1.Fee) * rl.config.ResubmitFeeMultiplier)
}

// resendSecondTxOfCheckpointToBTC resends the second tx of the checkpoint with bumpedFee
func (rl *Relayer) resendSecondTxOfCheckpointToBTC(tx2 *types.BtcTxInfo, bumpedFee uint64) (*types.BtcTxInfo, error) {
	// set output value of the second tx to be the balance minus the bumped fee
	// if the bumped fee is higher than the balance, then set the bumped fee to
	// be equal to the balance to ensure the output value is not negative
	balance := uint64(tx2.Utxo.Amount.ToUnit(btcutil.AmountSatoshi))
	if bumpedFee > balance {
		log.Logger.Debugf("the bumped fee %v Satoshis for the second tx is more than UTXO amount %v Satoshis",
			bumpedFee, balance)
		bumpedFee = balance
	}
	tx2.Tx.TxOut[1].Value = int64(balance - bumpedFee)

	// resign the tx as the output is changed
	tx, err := rl.dumpPrivKeyAndSignTx(tx2.Tx, tx2.Utxo)
	if err != nil {
		return nil, err
	}

	txid, err := rl.sendTxToBTC(tx)
	if err != nil {
		return nil, err
	}

	// update tx info
	tx2.Fee = bumpedFee
	tx2.TxId = txid

	return tx2, nil
}

// calcMinRequiredTxReplacementFee returns the minimum transaction fee required for a
// transaction with the passed serialized size to be accepted into the memory
// pool and relayed.
// Adapted from https://github.com/btcsuite/btcd/blob/f9cbff0d819c951d20b85714cf34d7f7cc0a44b7/mempool/policy.go#L61
func (rl *Relayer) calcMinRequiredTxReplacementFee(serializedSize uint64) uint64 {
	// Calculate the minimum fee for a transaction to be allowed into the
	// mempool and relayed by scaling the base fee (which is the minimum
	// free transaction relay fee).
	minRelayFeeRate := btcutil.Amount(rl.RelayFeePerKW().FeePerKVByte())

	log.Logger.Debugf("current minimum relay fee rate is %v", minRelayFeeRate)

	minRelayFee := txrules.FeeForSerializeSize(minRelayFeeRate, int(serializedSize))

	// Set the minimum fee to the maximum possible value if the calculated
	// fee is not in the valid range for monetary amounts.
	if minRelayFee > btcutil.MaxSatoshi {
		minRelayFee = btcutil.MaxSatoshi
	}

	return uint64(minRelayFee)
}

func (rl *Relayer) dumpPrivKeyAndSignTx(tx *wire.MsgTx, utxo *types.UTXO) (*wire.MsgTx, error) {
	// get private key
	err := rl.WalletPassphrase(rl.GetWalletPass(), rl.GetWalletLockTime())
	if err != nil {
		return nil, err
	}
	wif, err := rl.DumpPrivKey(utxo.Addr)
	if err != nil {
		return nil, err
	}
	// add signature/witness depending on the type of the previous address
	// if not segwit, add signature; otherwise, add witness
	segwit, err := isSegWit(utxo.Addr)
	if err != nil {
		return nil, err
	}
	// add unlocking script into the input of the tx
	tx, err = completeTxIn(tx, segwit, wif.PrivKey, utxo)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

func (rl *Relayer) convertCkptToTwoTxAndSubmit(ckpt *ckpttypes.RawCheckpointWithMeta) (*types.CheckpointInfo, error) {
	btcCkpt, err := ckpttypes.FromRawCkptToBTCCkpt(ckpt.Ckpt, rl.submitterAddress)
	if err != nil {
		return nil, err
	}
	data1, data2, err := btctxformatter.EncodeCheckpointData(
		rl.tag,
		rl.version,
		btcCkpt,
	)
	if err != nil {
		return nil, err
	}

	utxo, err := rl.PickHighUTXO()
	if err != nil {
		return nil, err
	}

	log.Logger.Debugf("Found one unspent tx with sufficient amount: %v", utxo.TxID)

	tx1, tx2, err := rl.ChainTwoTxAndSend(
		utxo,
		data1,
		data2,
	)
	if err != nil {
		return nil, err
	}

	// this is to wait for btcwallet to update utxo database so that
	// the tx that tx1 consumes will not appear in the next unspent txs lit
	time.Sleep(1 * time.Second)

	log.Logger.Infof("Sent two txs to BTC for checkpointing epoch %v, first txid: %s, second txid: %s",
		ckpt.Ckpt.EpochNum, tx1.Tx.TxHash().String(), tx2.Tx.TxHash().String())

	// record metrics of the two transactions
	rl.metrics.NewSubmittedCheckpointSegmentGaugeVec.WithLabelValues(
		strconv.Itoa(int(ckpt.Ckpt.EpochNum)),
		"0",
		tx1.Tx.TxHash().String(),
		strconv.Itoa(int(tx1.Fee)),
	).SetToCurrentTime()
	rl.metrics.NewSubmittedCheckpointSegmentGaugeVec.WithLabelValues(
		strconv.Itoa(int(ckpt.Ckpt.EpochNum)),
		"1",
		tx2.Tx.TxHash().String(),
		strconv.Itoa(int(tx2.Fee)),
	).SetToCurrentTime()

	return &types.CheckpointInfo{
		Epoch: ckpt.Ckpt.EpochNum,
		Ts:    time.Now(),
		Tx1:   tx1,
		Tx2:   tx2,
	}, nil
}

// ChainTwoTxAndSend consumes one utxo and build two chaining txs:
// the second tx consumes the output of the first tx
func (rl *Relayer) ChainTwoTxAndSend(
	utxo *types.UTXO,
	data1 []byte,
	data2 []byte,
) (*types.BtcTxInfo, *types.BtcTxInfo, error) {

	// recipient is a change address that all the
	// remaining balance of the utxo is sent to
	tx1, err := rl.buildTxWithData(
		utxo,
		data1,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to add data to tx1: %w", err)
	}

	tx1.TxId, err = rl.sendTxToBTC(tx1.Tx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to send tx1 to BTC: %w", err)
	}

	changeUtxo := &types.UTXO{
		TxID:     tx1.TxId,
		Vout:     1,
		ScriptPK: tx1.Tx.TxOut[1].PkScript,
		Amount:   btcutil.Amount(tx1.Tx.TxOut[1].Value),
		Addr:     tx1.ChangeAddress,
	}

	// the second tx consumes the second output (index 1)
	// of the first tx, as the output at index 0 is OP_RETURN
	tx2, err := rl.buildTxWithData(
		changeUtxo,
		data2,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to add data to tx2: %w", err)
	}

	tx2.TxId, err = rl.sendTxToBTC(tx2.Tx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to send tx2 to BTC: %w", err)
	}

	// TODO: if tx1 succeeds but tx2 fails, we should not resent tx1

	return tx1, tx2, nil
}

// PickHighUTXO picks a UTXO that has the highest amount
func (rl *Relayer) PickHighUTXO() (*types.UTXO, error) {
	utxos, err := rl.ListUnspent()
	if err != nil {
		return nil, fmt.Errorf("failed to list unspent UTXOs: %w", err)
	}

	if len(utxos) == 0 {
		return nil, errors.New("lack of spendable transactions in the wallet")
	}

	log.Logger.Debugf("Found %v unspent transactions", len(utxos))

	topUtxo := utxos[0]
	sum := 0.0
	for i, utxo := range utxos {
		log.Logger.Debugf("tx %v id: %v, amount: %v BTC, confirmations: %v", i+1, utxo.TxID, utxo.Amount, utxo.Confirmations)
		if topUtxo.Amount < utxo.Amount {
			topUtxo = utxo
		}
		sum += utxo.Amount
	}
	rl.metrics.AvailableBTCBalance.Set(sum)

	// the following checks might cause panicking situations
	// because each of them indicates terrible errors brought
	// by btcclient
	prevPKScript, err := hex.DecodeString(topUtxo.ScriptPubKey)
	if err != nil {
		panic(err)
	}
	txID, err := chainhash.NewHashFromStr(topUtxo.TxID)
	if err != nil {
		panic(err)
	}
	prevAddr, err := btcutil.DecodeAddress(topUtxo.Address, rl.GetNetParams())
	if err != nil {
		panic(err)
	}
	amount, err := btcutil.NewAmount(topUtxo.Amount)
	if err != nil {
		panic(err)
	}

	log.Logger.Debugf("pick utxo with id: %v, amount: %v, confirmations: %v", topUtxo.TxID, topUtxo.Amount, topUtxo.Confirmations)

	utxo := &types.UTXO{
		TxID:     txID,
		Vout:     topUtxo.Vout,
		ScriptPK: prevPKScript,
		Amount:   amount,
		Addr:     prevAddr,
	}

	return utxo, nil
}

// buildTxWithData builds a tx with data inserted as OP_RETURN
// note that OP_RETURN is set as the first output of the tx (index 0)
// and the rest of the balance is sent to a new change address
// as the second output with index 1
func (rl *Relayer) buildTxWithData(
	utxo *types.UTXO,
	data []byte,
) (*types.BtcTxInfo, error) {
	log.Logger.Debugf("Building a BTC tx using %v with data %x", utxo.TxID.String(), data)
	tx := wire.NewMsgTx(wire.TxVersion)

	outPoint := wire.NewOutPoint(utxo.TxID, utxo.Vout)
	txIn := wire.NewTxIn(outPoint, nil, nil)
	// Enable replace-by-fee
	// See https://river.com/learn/terms/r/replace-by-fee-rbf
	txIn.Sequence = math.MaxUint32 - 2
	tx.AddTxIn(txIn)

	// build txout for data
	builder := txscript.NewScriptBuilder()
	dataScript, err := builder.AddOp(txscript.OP_RETURN).AddData(data).Script()
	if err != nil {
		return nil, err
	}
	tx.AddTxOut(wire.NewTxOut(0, dataScript))

	// build txout for change
	changeAddr, err := rl.GetChangeAddress()
	if err != nil {
		return nil, fmt.Errorf("failed to get change address: %w", err)
	}
	log.Logger.Debugf("Got a change address %v", changeAddr.String())
	changeScript, err := txscript.PayToAddrScript(changeAddr)
	if err != nil {
		return nil, err
	}
	copiedTx := &wire.MsgTx{}
	err = copier.Copy(copiedTx, tx)
	if err != nil {
		return nil, err
	}
	txSize := calculateTxSize(copiedTx, utxo, changeScript)
	minRelayFee := rl.RelayFeePerKW().FeePerKVByte().FeeForVSize(int64(txSize))
	if utxo.Amount < minRelayFee {
		return nil, fmt.Errorf("the value of the utxo is not sufficient for relaying the tx. Require: %v. Have: %v", minRelayFee, utxo.Amount)
	}
	feeRate := btcutil.Amount(rl.getFeeRate())
	txFee := txrules.FeeForSerializeSize(feeRate, txSize)
	// ensuring the tx fee is not higher than the utxo value
	if utxo.Amount < txFee {
		txFee = utxo.Amount
	}
	change := utxo.Amount - txFee
	tx.AddTxOut(wire.NewTxOut(int64(change), changeScript))

	// sign tx
	tx, err = rl.dumpPrivKeyAndSignTx(tx, utxo)
	if err != nil {
		return nil, fmt.Errorf("failed to sign tx: %w", err)
	}

	// serialization
	var signedTxBytes bytes.Buffer
	err = tx.Serialize(&signedTxBytes)
	if err != nil {
		return nil, err
	}

	btcTx, err := btcutil.NewTxFromBytes(signedTxBytes.Bytes())
	if err != nil {
		return nil, err
	}

	log.Logger.Debugf("Successfully composed a BTC tx with balance of input: %v, "+
		"tx fee: %v, output value: %v, estimated tx size: %v, actual tx size: %v, hex: %v",
		utxo.Amount, txFee, change, txSize, mempool.GetTxVirtualSize(btcTx),
		hex.EncodeToString(signedTxBytes.Bytes()))

	return &types.BtcTxInfo{
		Tx:            tx,
		Utxo:          utxo,
		ChangeAddress: changeAddr,
		Size:          uint64(txSize),
		Fee:           uint64(txFee),
	}, nil
}

// getFeeRate returns the estimated fee rate, ensuring it within [tx-fee-max, tx-fee-min]
func (rl *Relayer) getFeeRate() chainfee.SatPerKVByte {
	fee, err := rl.EstimateFeePerKW(uint32(rl.GetBTCConfig().TargetBlockNum))
	if err != nil {
		defaultFee := rl.GetBTCConfig().DefaultFee
		log.Logger.Errorf("failed to estimate transaction fee. Using default fee %v: %s", defaultFee, err.Error())
		return defaultFee
	}

	feePerKVByte := fee.FeePerKVByte()

	log.Logger.Debugf("current tx fee rate is %v", feePerKVByte)

	cfg := rl.GetBTCConfig()
	if feePerKVByte > cfg.TxFeeMax {
		log.Logger.Debugf("current tx fee rate is higher than the maximum tx fee rate %v, using the max", cfg.TxFeeMax)
		feePerKVByte = cfg.TxFeeMax
	}
	if feePerKVByte < cfg.TxFeeMin {
		log.Logger.Debugf("current tx fee rate is lower than the minimum tx fee rate %v, using the min", cfg.TxFeeMin)
		feePerKVByte = cfg.TxFeeMin
	}

	return feePerKVByte
}

func (rl *Relayer) sendTxToBTC(tx *wire.MsgTx) (*chainhash.Hash, error) {
	log.Logger.Debugf("Sending tx %v to BTC", tx.TxHash().String())
	ha, err := rl.SendRawTransaction(tx, true)
	if err != nil {
		return nil, err
	}
	log.Logger.Debugf("Successfully sent tx %v to BTC", tx.TxHash().String())
	return ha, nil
}
