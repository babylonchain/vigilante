package relayer

import (
	"bytes"
	"encoding/hex"
	"errors"
	"github.com/babylonchain/babylon/btctxformatter"
	ckpttypes "github.com/babylonchain/babylon/x/checkpointing/types"
	"github.com/babylonchain/vigilante/btcclient"
	"github.com/babylonchain/vigilante/log"
	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"time"
)

type Relayer struct {
	btcclient.BTCWallet
	sentCheckpoints  types.SentCheckpoints
	tag              btctxformatter.BabylonTag
	version          btctxformatter.FormatVersion
	submitterAddress sdk.AccAddress
	resendIntervals  uint
}

func New(
	wallet btcclient.BTCWallet,
	tag btctxformatter.BabylonTag,
	version btctxformatter.FormatVersion,
	submitterAddress sdk.AccAddress,
	resendIntervals uint,
) *Relayer {
	return &Relayer{
		BTCWallet:        wallet,
		sentCheckpoints:  types.NewSentCheckpoints(resendIntervals),
		tag:              tag,
		version:          version,
		submitterAddress: submitterAddress,
	}
}

// SendCheckpointToBTC converts the checkpoint into two transactions and send them to BTC
func (rl *Relayer) SendCheckpointToBTC(ckpt *ckpttypes.RawCheckpointWithMeta) error {
	if ckpt.Status != ckpttypes.Sealed {
		return errors.New("checkpoint is not Sealed")
	}
	if !rl.sentCheckpoints.ShouldSend(ckpt.Ckpt.EpochNum) {
		log.Logger.Debugf("Skip submitting the raw checkpoint for epoch %v", ckpt.Ckpt.EpochNum)
		return errors.New("checkpoint has already been submitted")
	}
	log.Logger.Debugf("Submitting a raw checkpoint for epoch %v", ckpt.Ckpt.EpochNum)
	err := rl.convertCkptToTwoTxAndSubmit(ckpt)
	if err != nil {
		return err
	}

	return nil
}

func (rl *Relayer) convertCkptToTwoTxAndSubmit(ckpt *ckpttypes.RawCheckpointWithMeta) error {
	btcCkpt, err := ckpttypes.FromRawCkptToBTCCkpt(ckpt.Ckpt, rl.submitterAddress)
	if err != nil {
		return err
	}
	data1, data2, err := btctxformatter.EncodeCheckpointData(
		rl.tag,
		rl.version,
		btcCkpt,
	)
	if err != nil {
		return err
	}

	utxo, err := rl.PickHighUTXO()
	if err != nil {
		return err
	}

	log.Logger.Debugf("Found one unspent tx with sufficient amount: %v", utxo.TxID)

	tx1, tx2, err := rl.ChainTwoTxAndSend(
		utxo,
		data1,
		data2,
	)
	if err != nil {
		return err
	}

	rl.sentCheckpoints.Add(ckpt.Ckpt.EpochNum, tx1.Hash(), tx2.Hash())

	// this is to wait for btcwallet to update utxo database so that
	// the tx that tx1 consumes will not appear in the next unspent txs lit
	time.Sleep(1 * time.Second)

	log.Logger.Infof("Sent two txs to BTC for checkpointing epoch %v, first txid: %v, second txid: %v", ckpt.Ckpt.EpochNum, tx1.Hash().String(), tx2.Hash().String())

	return nil
}

// ChainTwoTxAndSend consumes one utxo and build two chaining txs:
// the second tx consumes the output of the first tx
func (rl *Relayer) ChainTwoTxAndSend(
	utxo *types.UTXO,
	data1 []byte,
	data2 []byte,
) (*btcutil.Tx, *btcutil.Tx, error) {

	// recipient is a change address that all the
	// remaining balance of the utxo is sent to
	tx1, recipient, err := rl.buildTxWithData(
		utxo,
		data1,
	)
	if err != nil {
		return nil, nil, err
	}

	txid1, err := rl.sendTxToBTC(tx1)
	if err != nil {
		return nil, nil, err
	}

	changeUtxo := &types.UTXO{
		TxID:     txid1,
		Vout:     1,
		ScriptPK: tx1.TxOut[1].PkScript,
		Amount:   btcutil.Amount(tx1.TxOut[1].Value),
		Addr:     recipient,
	}

	// the second tx consumes the second output (index 1)
	// of the first tx, as the output at index 0 is OP_RETURN
	tx2, _, err := rl.buildTxWithData(
		changeUtxo,
		data2,
	)
	if err != nil {
		return nil, nil, err
	}

	_, err = rl.sendTxToBTC(tx2)
	if err != nil {
		return nil, nil, err
	}

	// TODO: if tx1 succeeds but tx2 fails, we should not resent tx1

	return btcutil.NewTx(tx1), btcutil.NewTx(tx2), nil
}

// PickHighUTXO picks a UTXO that has the highest amount
func (rl *Relayer) PickHighUTXO() (*types.UTXO, error) {
	log.Logger.Debugf("Searching for unspent transactions...")
	utxos, err := rl.ListUnspent()
	if err != nil {
		return nil, err
	}

	if len(utxos) == 0 {
		return nil, errors.New("lack of spendable transactions in the wallet")
	}

	log.Logger.Debugf("Found %v unspent transactions", len(utxos))

	topUtxo := utxos[0]
	for i, utxo := range utxos {
		log.Logger.Debugf("tx %v id: %v, amount: %v, confirmations: %v", i+1, utxo.TxID, utxo.Amount, utxo.Confirmations)
		if topUtxo.Amount < utxo.Amount {
			topUtxo = utxo
		}
	}

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

	// TODO: consider dust, reference: https://www.oreilly.com/library/view/mastering-bitcoin/9781491902639/ch08.html#tx_verification
	if uint64(amount.ToUnit(btcutil.AmountSatoshi)) < rl.GetTxFee()*2 {
		return nil, errors.New("insufficient fees")
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
) (*wire.MsgTx, btcutil.Address, error) {
	log.Logger.Debugf("Building a BTC tx using %v with data %x", utxo.TxID.String(), data)
	tx := wire.NewMsgTx(wire.TxVersion)

	outPoint := wire.NewOutPoint(utxo.TxID, utxo.Vout)
	txIn := wire.NewTxIn(outPoint, nil, nil)
	tx.AddTxIn(txIn)

	// build txout for data
	builder := txscript.NewScriptBuilder()
	dataScript, err := builder.AddOp(txscript.OP_RETURN).AddData(data).Script()
	if err != nil {
		return nil, nil, err
	}
	tx.AddTxOut(wire.NewTxOut(0, dataScript))

	// build txout for change
	changeAddr, err := rl.GetRawChangeAddress()
	if err != nil {
		return nil, nil, err
	}
	log.Logger.Debugf("Got a change address %v", changeAddr.String())

	changeScript, err := txscript.PayToAddrScript(changeAddr)
	if err != nil {
		return nil, nil, err
	}
	// TODO
	// 		If this will become a dynamic calculation this might lead to a different fee being used in the log and the actual transaction.
	change := uint64(utxo.Amount.ToUnit(btcutil.AmountSatoshi)) - rl.GetTxFee()
	log.Logger.Debugf("balance of input: %v satoshi, tx fee: %v satoshi, output value: %v",
		int64(utxo.Amount.ToUnit(btcutil.AmountSatoshi)), rl.GetTxFee(), change)
	tx.AddTxOut(wire.NewTxOut(int64(change), changeScript))

	// sign tx
	err = rl.WalletPassphrase(rl.GetWalletPass(), rl.GetWalletLockTime())
	if err != nil {
		return nil, nil, err
	}
	wif, err := rl.DumpPrivKey(utxo.Addr)
	if err != nil {
		return nil, nil, err
	}
	sig, err := txscript.SignatureScript(
		tx,
		0,
		utxo.ScriptPK,
		txscript.SigHashAll,
		wif.PrivKey,
		true)
	if err != nil {
		return nil, nil, err
	}
	tx.TxIn[0].SignatureScript = sig

	// serialization
	var signedTxHex bytes.Buffer
	err = tx.Serialize(&signedTxHex)
	if err != nil {
		return nil, nil, err
	}

	log.Logger.Debugf("Successfully composed a BTC tx, hex: %v", hex.EncodeToString(signedTxHex.Bytes()))
	return tx, changeAddr, nil
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
