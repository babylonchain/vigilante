package btcstaking_tracker

type IBTCSlasher interface {
	// common functions
	Bootstrap(startHeight uint64) error
	Start() error
	Stop() error
}

type IAtomicSlasher interface {
	Start() error
	Stop() error
}
