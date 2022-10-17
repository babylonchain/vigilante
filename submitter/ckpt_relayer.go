package submitter

import (
	"bytes"
	"encoding/hex"
	"errors"
	"github.com/babylonchain/babylon/btctxformatter"
	ckpttypes "github.com/babylonchain/babylon/x/checkpointing/types"
	"github.com/babylonchain/vigilante/netparams"
	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"time"
)

func (s *Submitter) sealedCkptHandler() {
	defer s.wg.Done()
	quit := s.quitChan()

	for {
		select {
		case ckpt := <-s.poller.GetSealedCheckpointChan():
			if ckpt.Status == ckpttypes.Sealed {
				log.Infof("A sealed raw checkpoint for epoch %v is found", ckpt.Ckpt.EpochNum)
				err := s.SubmitCkpt(ckpt)
				if err != nil {
					log.Errorf("Failed to submit the raw checkpoint for %v: %v", ckpt.Ckpt.EpochNum, err)
				}
			}
		case <-quit:
			// We have been asked to stop
			return
		}
	}
}

func (s *Submitter) SubmitCkpt(ckpt *ckpttypes.RawCheckpointWithMeta) error {
	if !s.sentCheckpoints.ShouldSend(ckpt.Ckpt.EpochNum) {
		log.Debugf("Skip submitting the raw checkpoint for epoch %v", ckpt.Ckpt.EpochNum)
		return nil
	}
	log.Debugf("Submitting a raw checkpoint for epoch %v", ckpt.Ckpt.EpochNum)
	err := s.ConvertCkptToTwoTxAndSubmit(ckpt)
	if err != nil {
		return err
	}

	return nil
}

func (s *Submitter) ConvertCkptToTwoTxAndSubmit(ckpt *ckpttypes.RawCheckpointWithMeta) error {
	btcCkpt, err := ckpttypes.FromRawCkptToBTCCkpt(ckpt.Ckpt, s.submitterAddress)
	data1, data2, err := btctxformatter.EncodeCheckpointData(
		s.Cfg.GetTag(s.poller.GetTagIdx()),
		s.Cfg.GetVersion(),
		btcCkpt,
	)
	if err != nil {
		return err
	}

	utxo, err := s.pickHighUTXO()

	log.Debugf("Found one unspent tx with sufficient amount: %v", utxo.TxID)

	txid1, txid2, err := s.chainTwoTxAndSend(
		utxo,
		data1,
		data2,
	)

	s.sentCheckpoints.Add(ckpt.Ckpt.EpochNum, txid1, txid2)

	// this is to wait for btcwallet to update utxo database so that
	// the tx that tx1 consumes will not appear in the next unspent txs lit
	time.Sleep(1 * time.Second)

	log.Infof("Sent two txs to BTC for checkpointing epoch %v, first txid: %v, second txid: %v", ckpt.Ckpt.EpochNum, txid1.String(), txid2.String())

	return nil
}

// chainTwoTxAndSend consumes one utxo and build two chaining txs:
// the second tx consumes the output of the first tx
func (s *Submitter) chainTwoTxAndSend(
	utxo *types.UTXO,
	data1 []byte,
	data2 []byte,
) (txid1 *chainhash.Hash, txid2 *chainhash.Hash, err error) {

	// recipient is a change address that all the
	// remaining balance of the utxo is sent to
	tx1, recipient, err := s.buildTxWithData(
		utxo,
		data1,
	)
	if err != nil {
		return nil, nil, err
	}

	txid1, err = s.sendTxToBTC(tx1)
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
	tx2, _, err := s.buildTxWithData(
		changeUtxo,
		data2,
	)

	txid2, err = s.sendTxToBTC(tx2)
	if err != nil {
		return nil, nil, err
	}

	// TODO: if tx1 succeeds but tx2 fails, we should not resent tx1

	return txid1, txid2, nil
}

// getTopUTXO picks a UTXO that has the highest amount
func (s *Submitter) pickHighUTXO() (*types.UTXO, error) {
	log.Debugf("Searching for unspent transactions...")
	utxos, err := s.btcWallet.ListUnspent()
	if err != nil {
		return nil, err
	}

	if len(utxos) == 0 {
		return nil, errors.New("lack of spendable transactions in the wallet")
	}

	log.Debugf("Found %v unspent transactions", len(utxos))

	topUtxo := utxos[0]
	for i, utxo := range utxos {
		log.Debugf("tx %v id: %v, amount: %v, confirmations: %v", i+1, utxo.TxID, utxo.Amount, utxo.Confirmations)
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
	prevAddr, err := btcutil.DecodeAddress(topUtxo.Address, netparams.GetBTCParams(s.Cfg.NetParams))
	if err != nil {
		panic(err)
	}
	amount, err := btcutil.NewAmount(topUtxo.Amount)
	if err != nil {
		panic(err)
	}

	// TODO: consider dust, reference: https://www.oreilly.com/library/view/mastering-bitcoin/9781491902639/ch08.html#tx_verification
	txfee := s.btcWallet.Cfg.TxFee.ToUnit(btcutil.AmountSatoshi)
	if amount.ToUnit(btcutil.AmountSatoshi) < txfee*2 {
		return nil, errors.New("insufficient fees")
	}

	log.Debugf("pick utxo with id: %v, amount: %v, confirmations: %v", topUtxo.TxID, topUtxo.Amount, topUtxo.Confirmations)

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
func (s *Submitter) buildTxWithData(
	utxo *types.UTXO,
	data []byte,
) (*wire.MsgTx, btcutil.Address, error) {
	log.Debugf("Building a BTC tx using %v with data %x", utxo.TxID.String(), data)
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
	changeAddr, err := s.btcWallet.GetRawChangeAddress(s.account)
	if err != nil {
		return nil, nil, err
	}
	log.Debugf("Got a change address %v", changeAddr.String())

	changeScript, err := txscript.PayToAddrScript(changeAddr)
	if err != nil {
		return nil, nil, err
	}
	change := utxo.Amount.ToUnit(btcutil.AmountSatoshi) - s.btcWallet.Cfg.TxFee.ToUnit(btcutil.AmountSatoshi)
	log.Debugf("balance of input: %v satoshi, tx fee: %v satoshi, output value: %v",
		int64(utxo.Amount.ToUnit(btcutil.AmountSatoshi)), int64(s.btcWallet.Cfg.TxFee.ToUnit(btcutil.AmountSatoshi)), int64(change))
	tx.AddTxOut(wire.NewTxOut(int64(change), changeScript))

	// sign tx
	err = s.btcWallet.WalletPassphrase(s.btcWallet.Cfg.WalletPassword, s.btcWallet.Cfg.WalletLockTime)
	if err != nil {
		return nil, nil, err
	}
	wif, err := s.btcWallet.DumpPrivKey(utxo.Addr)
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

	log.Debugf("Successfully composed a BTC tx, hex: %v", hex.EncodeToString(signedTxHex.Bytes()))
	return tx, changeAddr, nil
}

func (s *Submitter) sendTxToBTC(tx *wire.MsgTx) (*chainhash.Hash, error) {
	log.Debugf("Sending tx %v to BTC", tx.TxHash().String())
	ha, err := s.btcWallet.SendRawTransaction(tx, true)
	if err != nil {
		return nil, err
	}
	log.Debugf("Successfully sent tx %v to BTC", tx.TxHash().String())
	return ha, nil
}
