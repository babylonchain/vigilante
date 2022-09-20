package submitter

import (
	"bytes"
	"encoding/hex"
	"errors"
	"github.com/babylonchain/babylon/btctxformatter"
	ckpttypes "github.com/babylonchain/babylon/x/checkpointing/types"
	"github.com/babylonchain/vigilante/netparams"
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"sort"
	"time"
)

func (s *Submitter) sealedCkptHandler() {
	defer s.wg.Done()
	quit := s.quitChan()

	for {
		select {
		case ckpt := <-s.rawCkptChan:
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
		s.Cfg.GetTag(s.babylonClient.GetTagIdx()),
		s.Cfg.GetVersion(),
		btcCkpt,
	)
	if err != nil {
		return err
	}

	utxo1, utxo2, err := s.getTopTwoUTXOs()
	if err != nil {
		return err
	}
	tx1, err := s.buildTxWithData(*utxo1, data1)
	if err != nil {
		return err
	}

	tx2, err := s.buildTxWithData(*utxo2, data2)
	if err != nil {
		return err
	}

	// TODO: add a looper to send BTC txs asynchronously
	txid1, err := s.sendTxToBTC(tx1)
	if err != nil {
		return err
	}

	txid2, err := s.sendTxToBTC(tx2)
	if err != nil {
		return err
	}

	// TODO: if tx1 succeeds but tx2 fails, we should not resent tx1
	s.sentCheckpoints.Add(ckpt.Ckpt.EpochNum, txid1, txid2)

	// this is to wait for btcwallet to update utxo database so that
	// the tx that tx1 consumes will not appear in the next unspent txs lit
	time.Sleep(1 * time.Second)

	log.Infof("Sent two txs to BTC for checkpointing epoch %v, first txid: %v, second txid: %v", ckpt.Ckpt.EpochNum, txid1.String(), txid2.String())

	return nil
}

// TODO: investigate whether the two UTXOs can be linked by the input and output
func (s *Submitter) getTopTwoUTXOs() (*btcjson.ListUnspentResult, *btcjson.ListUnspentResult, error) {
	log.Debugf("Searching for unspent transactions...")
	utxos, err := s.btcWallet.ListUnspent()
	if err != nil {
		return nil, nil, err
	}

	if len(utxos) < 2 {
		return nil, nil, errors.New("lack of spendable transactions in the wallet")
	}

	// TODO: consider dust, reference: https://www.oreilly.com/library/view/mastering-bitcoin/9781491902639/ch08.html#tx_verification
	txfee := s.btcWallet.Cfg.TxFee.ToBTC()
	// sort utxos by confirmations in the descending order and pick the first one as input
	sort.Slice(utxos, func(i, j int) bool {
		return utxos[i].Amount > utxos[j].Amount
	})

	log.Debugf("Found %v unspent transactions", len(utxos))
	for i, utxo := range utxos {
		log.Debugf("tx %v id: %v, amount: %v, confirmations: %v", i+1, utxo.TxID, utxo.Amount, utxo.Confirmations)
	}

	if utxos[0].Amount < txfee {
		return nil, nil, errors.New("insufficient fees")
	}

	if utxos[1].Amount < txfee {
		return nil, nil, errors.New("insufficient fees")
	}

	log.Debugf("Found two unspent transactions, tx1: %v, tx2: %v", utxos[0].TxID, utxos[1].TxID)

	return &utxos[0], &utxos[1], nil
}

func (s *Submitter) buildTxWithData(utxo btcjson.ListUnspentResult, data []byte) (*wire.MsgTx, error) {
	log.Debugf("Building a BTC tx using %v with data %x", utxo.TxID, data)
	tx := wire.NewMsgTx(wire.TxVersion)

	// build txin
	hash, err := chainhash.NewHashFromStr(utxo.TxID)
	if err != nil {
		return nil, err
	}
	outPoint := wire.NewOutPoint(hash, utxo.Vout)
	txIn := wire.NewTxIn(outPoint, nil, nil)
	tx.AddTxIn(txIn)

	// build txout for data
	builder := txscript.NewScriptBuilder()
	dataScript, err := builder.AddOp(txscript.OP_RETURN).AddData(data).Script()
	if err != nil {
		return nil, err
	}
	tx.AddTxOut(wire.NewTxOut(0, dataScript))

	// build txout for change
	changeAddr, err := s.btcWallet.GetRawChangeAddress(s.account)
	if err != nil {
		return nil, err
	}
	log.Debugf("Got a change address %v", changeAddr.String())
	prevPKScript, err := hex.DecodeString(utxo.ScriptPubKey)
	if err != nil {
		return nil, err
	}
	changeScript, err := txscript.PayToAddrScript(changeAddr)
	if err != nil {
		return nil, err
	}
	amount, err := btcutil.NewAmount(utxo.Amount)
	if err != nil {
		return nil, err
	}
	change := amount.ToUnit(btcutil.AmountSatoshi) - s.btcWallet.Cfg.TxFee.ToUnit(btcutil.AmountSatoshi)
	log.Debugf("balance of input: %v satoshi, tx fee: %v satoshi, output value: %v",
		amount.ToUnit(btcutil.AmountSatoshi), s.btcWallet.Cfg.TxFee.ToUnit(btcutil.AmountSatoshi), int64(change))
	tx.AddTxOut(wire.NewTxOut(int64(change), changeScript))

	// sign tx
	err = s.btcWallet.WalletPassphrase(s.btcWallet.Cfg.WalletPassword, s.btcWallet.Cfg.WalletLockTime)
	if err != nil {
		return nil, err
	}
	prevAddr, err := btcutil.DecodeAddress(utxo.Address, netparams.GetBTCParams(s.Cfg.NetParams))
	wif, err := s.btcWallet.DumpPrivKey(prevAddr)
	if err != nil {
		return nil, err
	}
	sig, err := txscript.SignatureScript(
		tx,
		0,
		prevPKScript,
		txscript.SigHashAll,
		wif.PrivKey,
		true)
	if err != nil {
		return nil, err
	}
	tx.TxIn[0].SignatureScript = sig

	// serialization
	var signedTxHex bytes.Buffer
	err = tx.Serialize(&signedTxHex)
	if err != nil {
		return nil, err
	}

	log.Debugf("Successfully composed a BTC tx, hex: %v", hex.EncodeToString(signedTxHex.Bytes()))
	return tx, nil
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
