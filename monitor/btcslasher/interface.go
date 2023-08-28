package btcslasher

type IBTCSlasher interface {
	// common functions
	Bootstrap(startHeight uint64) error
	Start()
	Stop()
}
