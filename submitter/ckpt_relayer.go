package submitter

import (
	"bytes"
	"errors"
	"github.com/babylonchain/babylon/btctxformatter"
	ckpttypes "github.com/babylonchain/babylon/x/checkpointing/types"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"sort"
)

// TODO: hardcoded for now, will be accessed securely
var privkeyWIF = "FosDkiJMxGjfDxSVqL9FQMnW83co4fDj6VsRPkkvvNkzxoyYm9WU"
var walletSeed = "e977bd4af4fa60e0534aa5cf864ab5b7297b72fa9d78135a166f35f4a879046a"
var btcaddress = "SPg2bpJoWwMz2UNYJsgAJfLRsSRs8HkNcD"

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
	tx1, tx2, err := s.ConvertCkptToTwoTx(ckpt)
	if err != nil {
		return err
	}

	err = s.sendTxToBTC(tx1)
	if err != nil {
		return err
	}

	err = s.sendTxToBTC(tx2)
	if err != nil {
		return err
	}

	return nil
}

func (s *Submitter) ConvertCkptToTwoTx(ckpt *ckpttypes.RawCheckpointWithMeta) (*wire.MsgTx, *wire.MsgTx, error) {
	lch, err := ckpt.Ckpt.LastCommitHash.Marshal()
	if err != nil {
		return nil, nil, err
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
		return nil, nil, err
	}
	tx1, err := s.buildTxWithData(data1)
	if err != nil {
		return nil, nil, err
	}
	tx2, err := s.buildTxWithData(data2)
	if err != nil {
		return nil, nil, err
	}

	return tx1, tx2, nil
}

func (s *Submitter) buildTxWithData(data []byte) (*wire.MsgTx, error) {
	utxos, err := s.btcWallet.ListUnspent()
	if err != nil {
		return nil, err
	}
	log.Infof("utxos %v", utxos)

	// sort utxos by amount in the descending order and pick the first one as input
	sort.Slice(utxos, func(i, j int) bool {
		return utxos[i].Spendable && utxos[i].Amount > utxos[j].Amount
	})
	pick := utxos[0]
	log.Infof("picked tx is %v", pick)
	if s.Cfg.TxFee.ToBTC() > pick.Amount {
		return nil, errors.New("insufficient fees")
	}

	tx := wire.NewMsgTx(wire.TxVersion)

	// build txin
	hash, err := chainhash.NewHashFromStr(pick.TxID)
	if err != nil {
		return nil, err
	}
	outPoint := wire.NewOutPoint(hash, pick.Vout)
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
	addr, err := btcutil.DecodeAddress(pick.Address, &chaincfg.SimNetParams)
	if err != nil {
		return nil, err
	}
	changeScript, err := txscript.PayToAddrScript(changeAddr)
	if err != nil {
		return nil, err
	}
	amount, err := btcutil.NewAmount(pick.Amount)
	if err != nil {
		return nil, err
	}
	tx.AddTxOut(wire.NewTxOut(int64(amount-s.Cfg.TxFee), changeScript))

	// sign tx
	err = s.btcWallet.WalletPassphrase("930812", 10)
	if err != nil {
		return nil, err
	}
	wif, err := s.btcWallet.DumpPrivKey(addr)
	if err != nil {
		return nil, err
	}

	prevTx, err := s.btcClient.GetRawTransaction(hash)
	prevOutputScript := prevTx.MsgTx().TxOut[0].PkScript

	//prevOutputScript, err := hex.DecodeString(pick.RedeemScript)
	log.Infof("prevOutputscript is %v", prevOutputScript)
	if err != nil {
		return nil, err
	}
	tx.TxIn[0].SignatureScript, err = txscript.SignatureScript(
		tx,
		0,
		prevOutputScript,
		txscript.SigHashAll,
		wif.PrivKey,
		false)
	if err != nil {
		return nil, err
	}

	var signedTxHex bytes.Buffer
	err = tx.Serialize(&signedTxHex)
	if err != nil {
		return nil, err
	}

	log.Infof("tx detail %v", tx)
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
