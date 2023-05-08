package relayer

import (
	"errors"
	"github.com/btcsuite/btcd/btcutil"
	"math/rand"
)

// GetChangeAddress randomly picks one address from local addresses that have received funds
// it gives priority to SegWit Bech32 addresses
func (rl *Relayer) GetChangeAddress() (btcutil.Address, error) {
	addrResults, err := rl.ListUnspent()
	if err != nil {
		return nil, err
	}
	if len(addrResults) == 0 {
		return nil, errors.New("no available addresses found in the wallet")
	}

	var segwitBech32Addrs []btcutil.Address
	var legacyAddrs []btcutil.Address
	for _, res := range addrResults {
		addr, err := btcutil.DecodeAddress(res.Address, rl.GetNetParams())
		if err != nil {
			return nil, err
		}
		switch addr.(type) {
		case *btcutil.AddressWitnessPubKeyHash:
			segwitBech32Addrs = append(segwitBech32Addrs, addr)
		case *btcutil.AddressWitnessScriptHash:
			segwitBech32Addrs = append(segwitBech32Addrs, addr)
		default:
			legacyAddrs = append(legacyAddrs, addr)
		}
	}

	if len(segwitBech32Addrs) != 0 {
		return segwitBech32Addrs[rand.Intn(len(segwitBech32Addrs))], nil
	}

	return legacyAddrs[rand.Intn(len(legacyAddrs))], nil
}
