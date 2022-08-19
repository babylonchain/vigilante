package reporter

import (
	"bytes"

	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

var (
	MaxOpReturnPkScriptSize        = 83
	IdentifierFirstHalf     []byte = []byte{0x62, 0x62, 0x6e, 0x00} // BBN + 0x00
	IdentifierSecondHalf    []byte = []byte{0x62, 0x62, 0x6e, 0x01} // BBN + 0x01
	LengthFirstHalf         int    = 73                             // identifier (4) + epoch_num (4) + last_commit_hash (32) + bitmap (13) + bbn_addr (20)
	LengthSecondHalf        int    = 62                             // identifier (4) + bls_multi_sig (48) + checksum (10)
)

func filterTx(tx *wire.MsgTx) ([]byte, []byte) {
	opReturnData := extractOpReturnData(tx)
	// check protocol identifier and first/second half identifier
	if isFirstHalf(opReturnData) {
		return opReturnData, nil // TODO: decode to object
	} else if isSecondHalf(opReturnData) {
		return nil, opReturnData // TODO: decode to object
	}
	return nil, nil
}

func filterBlock(block *wire.MsgBlock) ([][]byte, [][]byte) {
	entries1, entries2 := [][]byte{}, [][]byte{}
	for _, tx := range block.Transactions {
		entry1, entry2 := filterTx(tx)
		if entry1 != nil {
			entries1 = append(entries1, entry1) // TODO: decode to object
		} else if entry2 != nil {
			entries2 = append(entries2, entry2) // TODO: decode to object
		} else {
			continue
		}
	}
	return entries1, entries2
}

// TODO: change this to a decoder directly
func isFirstHalf(opReturnData []byte) bool {
	if len(opReturnData) != LengthFirstHalf {
		return false
	}
	if bytes.Compare(opReturnData[:4], IdentifierFirstHalf) == 0 {
		return true
	}
	return false
}

// TODO: change this to a decoder directly
func isSecondHalf(opReturnData []byte) bool {
	if len(opReturnData) != LengthSecondHalf {
		return false
	}
	if bytes.Compare(opReturnData[:4], IdentifierSecondHalf) == 0 {
		return true
	}
	return false
}

// adapted from https://github.com/babylonchain/babylon/blob/648b804bc492ded2cb826ba261d7164b4614d78a/x/btccheckpoint/btcutils/btcutils.go#L105-L131
func extractOpReturnData(msgTx *wire.MsgTx) []byte {
	opReturnData := []byte{}

	for _, output := range msgTx.TxOut {
		pkScript := output.PkScript
		pkScriptLen := len(pkScript)
		// valid op return script will have at least 2 bytes
		// - fisrt byte should be OP_RETURN marker
		// - second byte should indicate how many bytes there are in opreturn script
		if pkScriptLen > 1 &&
			pkScriptLen <= MaxOpReturnPkScriptSize &&
			pkScript[0] == txscript.OP_RETURN {

			// if this is OP_PUSHDATA1, we need to drop first 3 bytes as those are related
			// to script itself i.e OP_RETURN + OP_PUSHDATA1 + len of bytes
			if pkScript[1] == txscript.OP_PUSHDATA1 {
				opReturnData = append(opReturnData, pkScript[3:]...)
			} else {
				// this should be one of OP_DATAXX opcodes we drop first 2 bytes
				opReturnData = append(opReturnData, pkScript[2:]...)
			}
		}
	}

	return opReturnData
}
