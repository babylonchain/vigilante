package btcscanner

import (
	"github.com/babylonchain/vigilante/types"
)

// blockEventHandler handles connected and disconnected blocks from the BTC client.
func (bs *BtcScanner) blockEventHandler() {
	defer bs.wg.Done()
	quit := bs.quitChan()

	for {
		select {
		case event, open := <-bs.BtcClient.BlockEventChan():
			if !open {
				log.Errorf("Block event channel is closed")
				return // channel closed
			}

			if event.EventType == types.BlockConnected {
				bs.handleConnectedBlocks(event)
			} else if event.EventType == types.BlockDisconnected {
				bs.handleDisconnectedBlocks(event)
			}
		case <-quit:
			// We have been asked to stop
			return
		}
	}
}

// handleConnectedBlocks handles connected blocks from the BTC client.
func (bs *BtcScanner) handleConnectedBlocks(event *types.BlockEvent) {
	panic("implement me")
}

// handleDisconnectedBlocks handles disconnected blocks from the BTC client.
func (bs *BtcScanner) handleDisconnectedBlocks(event *types.BlockEvent) {
	panic("implement me")
}
