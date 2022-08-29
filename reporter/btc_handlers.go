package reporter

import (
	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/wire"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (r *Reporter) indexedBlockHandler() {
	defer r.wg.Done()
	quit := r.quitChan()

	for {
		select {
		case ib := <-r.btcClient.IndexedBlockChan:
			header := ib.Header
			signer := r.babylonClient.MustGetAddr()
			log.Infof("Start handling block %v from BTC client", ib.BlockHash())

			// wrap header into MsgInsertHeader message and submit to Babylon
			if err := r.handleHeader(header, signer); err != nil {
				log.Errorf("Failed to handle header %v from Bitcoin: %v", header.BlockHash(), err)
				// TODO: handle error
			}
			log.Infof("Successfully submitted MsgInsertHeader with header hash %v to Babylon", header.BlockHash())
			// TODO: ensure that the header is inserted into BTCLightclient, then filter txs
			// (see relevant discussion in https://github.com/babylonchain/vigilante/pull/5)

			// for each tx,
			// - try to extract a ckpt half from it
			// - cache the half locally
			// - try to match the half with an existing half
			// - wrap the two halves to InsertBTCSpvProof and submit to Babylon
			for _, tx := range ib.Txs {
				if err := r.handleTx(tx); err != nil {
					log.Errorf("Failed to handle Tx %v from Bitcoin: %v", tx.Hash(), err)
					// TODO: handle error
					continue
				}
			}
		case <-quit:
			// We have been asked to stop
			return
		}
	}
}

func (r *Reporter) handleHeader(header *wire.BlockHeader, signer sdk.AccAddress) error {
	msgInsertHeader := types.NewMsgInsertHeader(r.babylonClient.Cfg.AccountPrefix, signer, header)
	log.Debugf("signer: %v, headerHex: %v", signer, msgInsertHeader.Header.MarshalHex())
	_, err := r.babylonClient.InsertHeader(msgInsertHeader)
	if err != nil {
		return err
	}
	return nil
}

func (r *Reporter) handleTx(*btcutil.Tx) error {
	// TODO: decode to objects
	// TODO: check if the filtered entry can assemble with an existing entry to a new valid ckpt
	// TODO: upon a newly assembled checkpoint, forward it to BTCCheckpoint module
	return nil
}
