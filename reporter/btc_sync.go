package reporter

import (
	"github.com/btcsuite/btcwallet/chain"
)

func (r *Reporter) handleBTCNotifications() {
	defer r.wg.Done()

	btcClient := r.MustGetBtcClient()

	for {
		select {
		case n, ok := <-btcClient.Notifications():
			if !ok {
				log.Errorf("failed to receive notifiications from BTC client")
				return
			}

			switch n := n.(type) {
			case chain.BlockConnected:
				block := n.Block
				log.Infof("block %v at height %d is newly connected to BTC", block.Hash, block.Height)
				// obtain the full block
				msgBlock, err := btcClient.GetBlock(&block.Hash)
				if err != nil {
					log.Errorf("failed to get block %v from BTC client", block.Hash)
					continue
				}
				// TODO: append block to the local BTC block queue
				// TODO: filter and decode ckpts
				entry1, entry2 := filter(msgBlock)
				log.Infof("filrered first half: %v; second half: %v", entry1, entry2)
			case chain.BlockDisconnected:
				log.Info("block %v has been disconnected from BTC", n.Hash)
			}
		case <-r.quit:
			return
		}
	}
}
