package btcclient

import (
	"bytes"

	"github.com/btcsuite/btcd/wire"
)

var (
	IdentifierFirstHalf  []byte = []byte{0x62, 0x62, 0x6e, 0x00} // BBN + 0x00
	IdentifierSecondHalf []byte = []byte{0x62, 0x62, 0x6e, 0x01} // BBN + 0x01
	LengthFirstHalf      int    = 73 + 1                         // 1 is for opcode OP_RETURN
	LengthSecondHalf     int    = 62 + 1                         // 1 is for opcode OP_RETURN
)

func filter(block *wire.MsgBlock) ([][]byte, [][]byte) {
	entry1, entry2 := [][]byte{}, [][]byte{}
	for _, tx := range block.Transactions {
		for _, out := range tx.TxOut {
			script := out.PkScript
			// check protocol identifier and first/second half identifier
			if isFirstHalf(script) {
				entry1 = append(entry1, script) // TODO: deserialise
			} else if isSecondHalf(script) {
				entry2 = append(entry2, script) // TODO: deserialise
			} else {
				continue
			}
		}
	}
	return entry1, entry2
}

// TODO: change this to a decoder directly
func isFirstHalf(script []byte) bool {
	if len(script) != LengthFirstHalf {
		return false
	}
	if bytes.Compare(script[:4], IdentifierFirstHalf) == 0 {
		return true
	}
	return false
}

// TODO: change this to a decoder directly
func isSecondHalf(script []byte) bool {
	if len(script) != LengthSecondHalf {
		return false
	}
	if bytes.Compare(script[:4], IdentifierSecondHalf) == 0 {
		return true
	}
	return false
}
