package submitter

import (
	"bytes"
	"encoding/hex"
	"errors"
	"github.com/babylonchain/babylon/btctxformatter"
	ckpttypes "github.com/babylonchain/babylon/x/checkpointing/types"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"sort"
)

// TODO: hardcoded for now, will be accessed securely
var privkeyWIF = "cMcv2Y3vDY2STEkFqsDrVryZ7dZHkL9gNExMg1jmk2BSVMizinHu"

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
	tx1, tx2, err := s.ConvertCkptToTwoTxBytes(ckpt)
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

func (s *Submitter) ConvertCkptToTwoTxBytes(ckpt *ckpttypes.RawCheckpointWithMeta) ([]byte, []byte, error) {
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
	tx1, err := s.buildTxBytesWithData(data1)
	if err != nil {
		return nil, nil, err
	}
	tx2, err := s.buildTxBytesWithData(data2)
	if err != nil {
		return nil, nil, err
	}

	return tx1, tx2, nil
}

func (s *Submitter) buildTxBytesWithData(data []byte) ([]byte, error) {
	utxos, err := s.btcClient.ListUnspent()
	if err != nil {
		return nil, err
	}

	// sort utxos by amount in the descending order and pick the first one as input
	sort.Slice(utxos, func(i, j int) bool {
		return utxos[i].Spendable && utxos[i].Amount > utxos[j].Amount
	})
	pick := utxos[0]
	if s.Cfg.TxFee.ToBTC() < pick.Amount {
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
	changeAddr, err := s.btcClient.GetRawChangeAddress(s.account)
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
	wif, err := btcutil.DecodeWIF(privkeyWIF)
	if err != nil {
		return nil, err
	}
	prevOutputScript, err := hex.DecodeString(pick.RedeemScript)
	if err != nil {
		return nil, err
	}
	tx.TxIn[0].SignatureScript, err = txscript.SignatureScript(
		tx,
		0,
		prevOutputScript,
		txscript.SigHashAll,
		wif.PrivKey,
		true)
	if err != nil {
		return nil, err
	}

	var signedTxHex bytes.Buffer
	err = tx.Serialize(&signedTxHex)
	if err != nil {
		return nil, err
	}

	return signedTxHex.Bytes(), nil
}

func (s *Submitter) sendTxToBTC(signedHex []byte) error {
	panic("implement me")
}
