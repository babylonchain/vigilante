package relayer

import (
	"errors"
	"github.com/btcsuite/btcutil"
)

func isSegWit(addr btcutil.Address) (bool, error) {
	switch addr.(type) {
	case *btcutil.AddressPubKeyHash:
		return false, nil
	case *btcutil.AddressScriptHash:
		return false, nil
	case *btcutil.AddressPubKey:
		return false, nil
	case *btcutil.AddressWitnessPubKeyHash:
		return true, nil
	case *btcutil.AddressWitnessScriptHash:
		return true, nil
	default:
		return false, errors.New("non-supported address type")
	}
}
