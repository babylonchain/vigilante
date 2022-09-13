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
	err := s.ConvertCkptToTwoTxAndSubmit(ckpt)
	if err != nil {
		return err
	}

	return nil
}

func (s *Submitter) ConvertCkptToTwoTxAndSubmit(ckpt *ckpttypes.RawCheckpointWithMeta) error {
	lch, err := ckpt.Ckpt.LastCommitHash.Marshal()
	if err != nil {
		return err
	}
	data1, data2, err := btctxformatter.EncodeCheckpointData(
		s.Cfg.GetTag(),
		s.Cfg.GetVersion(),
		ckpt.Ckpt.EpochNum,
		lch,
		ckpt.Ckpt.Bitmap,
		ckpt.Ckpt.BlsMultiSig.Bytes(),
		s.submitterAddress,
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

	// TODO: add a looper to send BTC txs asynchronously
	err = s.sendTxToBTC(tx1)
	if err != nil {
		return err
	}

	tx2, err := s.buildTxWithData(*utxo2, data2)
	if err != nil {
		return err
	}

	err = s.sendTxToBTC(tx2)
	if err != nil {
		return err
	}

	return nil
}

func (s *Submitter) getTopTwoUTXOs() (*btcjson.ListUnspentResult, *btcjson.ListUnspentResult, error) {
	utxos, err := s.btcWallet.ListUnspent()
	if err != nil {
		return nil, nil, err
	}

	if len(utxos) < 2 {
		return nil, nil, errors.New("insufficient unspent transactions")
	}

	// sort utxos by amount in the descending order and pick the first one as input
	sort.Slice(utxos, func(i, j int) bool {
		return utxos[i].Spendable && utxos[i].Amount > utxos[j].Amount
	})

	if utxos[0].Amount < s.Cfg.TxFee.ToBTC() {
		return nil, nil, errors.New("insufficient fees")
	}

	if utxos[1].Amount < s.Cfg.TxFee.ToBTC() {
		return nil, nil, errors.New("insufficient fees")
	}

	return &utxos[0], &utxos[1], nil
}

func (s *Submitter) buildTxWithData(utxo btcjson.ListUnspentResult, data []byte) (*wire.MsgTx, error) {
	tx := wire.NewMsgTx(wire.TxVersion)

	// build txin
	hash, err := chainhash.NewHashFromStr(utxo.TxID)
	if err != nil {
		return nil, err
	}
	outPoint := wire.NewOutPoint(hash, 0)
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
	prevPKScript, err := hex.DecodeString(utxo.ScriptPubKey)
	if err != nil {
		return nil, err
	}
	changeScript, err := txscript.PayToAddrScript(changeAddr)
	if err != nil {
		return nil, err
	}
	amount, err := btcutil.NewAmount(50)
	if err != nil {
		return nil, err
	}
	tx.AddTxOut(wire.NewTxOut(int64(amount-s.Cfg.TxFee), changeScript))

	// sign tx
	err = s.btcWallet.WalletPassphrase(s.Cfg.WalletPass, 10)
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
	return tx, nil
}

func (s *Submitter) sendTxToBTC(tx *wire.MsgTx) error {
	ha, err := s.btcWallet.SendRawTransaction(tx, true)
	if err != nil {
		panic(err)
	}
	log.Infof("tx id is %x", ha)
	return nil
}
