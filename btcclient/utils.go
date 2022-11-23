package btcclient

import "github.com/btcsuite/btcutil"

func FeeApplicable(fee btcutil.Amount, min btcutil.Amount, max btcutil.Amount) bool {
	return fee >= min && fee <= max
}
