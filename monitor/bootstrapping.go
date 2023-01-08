package monitor

// Bootstrap processes BTC blocks from the base height up to the tip height of babylon/btclightclient,
// bootstrapping includes the consistency check between btc and Babylon (i.e., BTC headers and checkpoints)
func (m *Monitor) Bootstrap(skipBlockSubscription bool) {
	panic("implement me!")
}
