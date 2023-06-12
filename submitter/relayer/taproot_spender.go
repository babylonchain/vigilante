package relayer

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math"

	"github.com/babylonchain/vigilante/log"
	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/jinzhu/copier"
)

var (
	// TODO investigae how to best generate unspendabe internal private key:
	// https://github.com/bitcoin/bips/blob/master/bip-0341.mediawiki
	internalPrivateKey = "5JGgKfRy6vEcWBpLJV5FXUfMGNXzvdWzQHUM1rVLEUJfvZUSwvS"
)

// createTaprootAddress returns an address committing to a Taproot script with
// a single leaf containing the spend path with the script:
// <embedded data> OP_DROP <pubkey> OP_CHECKSIG
func createTaprootAddress(embeddedData []byte, privKey *btcec.PrivateKey, net *chaincfg.Params) (string, error) {
	if len(embeddedData) > 520 {
		return "", fmt.Errorf("embedded data must be less than 520 bytes")
	}

	// privKey, err := btcutil.DecodeWIF(pr)
	// if err != nil {
	// 	return "", fmt.Errorf("error decoding bob private key: %v", err)
	// }

	pubKey := privKey.PubKey()

	// Step 1: Construct the Taproot script with one leaf.
	builder := txscript.NewScriptBuilder()
	builder.AddOp(txscript.OP_0)
	builder.AddOp(txscript.OP_IF)
	builder.AddData(embeddedData)
	builder.AddOp(txscript.OP_ENDIF)
	builder.AddData(schnorr.SerializePubKey(pubKey))
	builder.AddOp(txscript.OP_CHECKSIG)
	pkScript, err := builder.Script()
	if err != nil {
		return "", fmt.Errorf("error building script: %v", err)
	}

	tapLeaf := txscript.NewBaseTapLeaf(pkScript)

	tapScriptTree := txscript.AssembleTaprootScriptTree(tapLeaf)

	internalPrivKey, err := btcutil.DecodeWIF(internalPrivateKey)
	if err != nil {
		return "", fmt.Errorf("error decoding internal private key: %v", err)
	}

	internalPubKey := internalPrivKey.PrivKey.PubKey()

	// Step 2: Generate the Taproot tree.
	tapScriptRootHash := tapScriptTree.RootNode.TapHash()
	outputKey := txscript.ComputeTaprootOutputKey(
		internalPubKey, tapScriptRootHash[:],
	)

	// Step 3: Generate the Bech32m address.
	address, err := btcutil.NewAddressTaproot(
		schnorr.SerializePubKey(outputKey), net)

	if err != nil {
		return "", fmt.Errorf("error encoding Taproot address: %v", err)
	}

	return address.String(), nil
}

func (rl *Relayer) commitTxBuild(addr string, net *chaincfg.Params, utxo *types.UTXO) (*chainhash.Hash, error) {
	log.Logger.Debugf("Building a BTC tx using %v", utxo.TxID.String())
	tx := wire.NewMsgTx(wire.TxVersion)

	outPoint := wire.NewOutPoint(utxo.TxID, utxo.Vout)

	txIn := wire.NewTxIn(outPoint, nil, nil)
	// Enable replace-by-fee
	// See https://river.com/learn/terms/r/replace-by-fee-rbf
	txIn.Sequence = math.MaxUint32 - 2

	tx.AddTxIn(txIn)

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
		panic(err)
	}

	// build txout for taproot
	address, err := btcutil.DecodeAddress(addr, net)
	if err != nil {
		return nil, fmt.Errorf("error decoding recipient address: %v", err)
	}

	amount, err := btcutil.NewAmount(0.001)
	if err != nil {
		return nil, fmt.Errorf("error creating new amount: %v", err)
	}

	taprootWitnessScript, err := txscript.PayToAddrScript(address)

	if err != nil {
		return nil, err
	}

	tx.AddTxOut(wire.NewTxOut(int64(amount), taprootWitnessScript))

	// build txout for change
	changeAddr, err := rl.GetChangeAddress()
	if err != nil {
		return nil, err
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
	txSize, err := calTxSizeTap(copiedTx, utxo, changeScript, taprootWitnessScript, segwit, wif.PrivKey)
	if err != nil {
		return nil, err
	}
	txFee := rl.GetTxFee(txSize)

	change := uint64(utxo.Amount.ToUnit(btcutil.AmountSatoshi)) - txFee - uint64(amount.ToUnit(btcutil.AmountSatoshi))

	tx.AddTxOut(wire.NewTxOut(int64(change), changeScript))

	// add unlocking script into the input of the tx
	tx, err = completeTxIn(tx, segwit, wif.PrivKey, utxo)
	if err != nil {
		return nil, err
	}

	// serialization
	var signedTxHex bytes.Buffer
	err = tx.Serialize(&signedTxHex)
	if err != nil {
		return nil, err
	}
	log.Logger.Debugf("Successfully composed a BTC tx with balance of input: %v satoshi, "+
		"tx fee: %v satoshi, output value: %v, estimated tx size: %v, actual tx size: %v, hex: %v",
		int64(utxo.Amount.ToUnit(btcutil.AmountSatoshi)), txFee, change, txSize, tx.SerializeSizeStripped(),
		hex.EncodeToString(signedTxHex.Bytes()))

	ch, err := rl.sendTxToBTC(tx)

	if err != nil {
		return nil, err
	}

	return ch, nil
}

func payToTaprootScript(taprootKey *btcec.PublicKey) ([]byte, error) {
	return txscript.NewScriptBuilder().
		AddOp(txscript.OP_1).
		AddData(schnorr.SerializePubKey(taprootKey)).
		Script()
}

func (rl *Relayer) revealTx(
	embeddedData []byte,
	commitHash *chainhash.Hash,
	privKey *btcec.PrivateKey,
) (*chainhash.Hash, error) {
	rawCommitTx, err := rl.GetRawTransaction(commitHash)
	if err != nil {
		return nil, fmt.Errorf("error getting raw commit tx: %v", err)
	}

	// TODO: use a better way to find our output
	var commitIndex int
	var commitOutput *wire.TxOut
	for i, out := range rawCommitTx.MsgTx().TxOut {
		if out.Value == 100000 {
			commitIndex = i
			commitOutput = out
			break
		}
	}

	// privKey, err := btcutil.DecodeWIF(bobPrivateKey)
	// if err != nil {
	// 	return nil, fmt.Errorf("error decoding bob private key: %v", err)
	// }

	pubKey := privKey.PubKey()

	internalPrivKey, err := btcutil.DecodeWIF(internalPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("error decoding internal private key: %v", err)
	}

	internalPubKey := internalPrivKey.PrivKey.PubKey()

	// Our script will be a simple <embedded-data> OP_DROP OP_CHECKSIG as the
	// sole leaf of a tapscript tree.
	builder := txscript.NewScriptBuilder()
	builder.AddOp(txscript.OP_0)
	builder.AddOp(txscript.OP_IF)
	builder.AddData(embeddedData)
	builder.AddOp(txscript.OP_ENDIF)
	builder.AddData(schnorr.SerializePubKey(pubKey))
	builder.AddOp(txscript.OP_CHECKSIG)
	pkScript, err := builder.Script()
	if err != nil {
		return nil, fmt.Errorf("error building script: %v", err)
	}

	tapLeaf := txscript.NewBaseTapLeaf(pkScript)
	tapScriptTree := txscript.AssembleTaprootScriptTree(tapLeaf)

	ctrlBlock := tapScriptTree.LeafMerkleProofs[0].ToControlBlock(
		internalPubKey,
	)

	tapScriptRootHash := tapScriptTree.RootNode.TapHash()
	outputKey := txscript.ComputeTaprootOutputKey(
		internalPubKey, tapScriptRootHash[:],
	)
	p2trScript, err := payToTaprootScript(outputKey)
	if err != nil {
		return nil, fmt.Errorf("error building p2tr script: %v", err)
	}

	tx := wire.NewMsgTx(2)
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{
			Hash:  *rawCommitTx.Hash(),
			Index: uint32(commitIndex),
		},
	})
	txOut := &wire.TxOut{
		Value: 1e3, PkScript: p2trScript,
	}
	tx.AddTxOut(txOut)

	inputFetcher := txscript.NewCannedPrevOutputFetcher(
		commitOutput.PkScript,
		commitOutput.Value,
	)
	sigHashes := txscript.NewTxSigHashes(tx, inputFetcher)

	sig, err := txscript.RawTxInTapscriptSignature(
		tx, sigHashes, 0, txOut.Value,
		txOut.PkScript, tapLeaf, txscript.SigHashDefault,
		privKey,
	)

	if err != nil {
		return nil, fmt.Errorf("error signing tapscript: %v", err)
	}

	// Now that we have the sig, we'll make a valid witness
	// including the control block.
	ctrlBlockBytes, err := ctrlBlock.ToBytes()
	if err != nil {
		return nil, fmt.Errorf("error including control block: %v", err)
	}
	tx.TxIn[0].Witness = wire.TxWitness{
		sig, pkScript, ctrlBlockBytes,
	}

	hash, err := rl.SendRawTransaction(tx, true)
	if err != nil {
		return nil, fmt.Errorf("error sending reveal transaction: %v", err)
	}

	log.Logger.Debugf("Successfully sent taproot reaceal transaction with size %d", tx.SerializeSize())

	return hash, nil
}

func (rl *Relayer) WriteTaprootData(data []byte, privKey *btcec.PrivateKey, utxo *types.UTXO) (*chainhash.Hash, *chainhash.Hash, error) {
	params := rl.GetNetParams()

	log.Logger.Debugf("Sending taproot transaction for net: %s", params.Name)

	address, err := createTaprootAddress(data, privKey, params)
	if err != nil {
		return nil, nil, err
	}
	hash1, err := rl.commitTxBuild(address, params, utxo)
	if err != nil {
		return nil, nil, err
	}
	hash2, err := rl.revealTx(data, hash1, privKey)
	if err != nil {
		return nil, nil, err
	}
	return hash1, hash2, nil
}
