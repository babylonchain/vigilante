//go:build e2e
// +build e2e

package e2etest

import (
	"encoding/hex"
	"testing"
	"time"

	bbn "github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/babylonchain/vigilante/monitor/unbondingwatcher"
	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/wire"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/stretchr/testify/require"
)

type ReportedInfo struct {
	StakingTxHash      chainhash.Hash
	StakerUnbondingSig *schnorr.Signature
}

type TestBabylonClient struct {
	tm             *TestManager
	reportedTxChan chan *ReportedInfo
}

func NewTestBabylonClient(tm *TestManager) *TestBabylonClient {
	return &TestBabylonClient{tm: tm, reportedTxChan: make(chan *ReportedInfo)}
}

var _ unbondingwatcher.BabylonNodeAdapter = (*TestBabylonClient)(nil)

// TODO: Hacky way to simualte babylon having pre-signed unbonding tx
func (tbc *TestBabylonClient) ActiveBtcDelegations(offset uint64, limit uint64) ([]unbondingwatcher.Delegation, error) {
	delegations, err := tbc.tm.BabylonClient.BTCDelegations(
		bstypes.BTCDelegationStatus_ANY,
		&query.PageRequest{
			Key:    nil,
			Offset: offset,
			Limit:  limit,
		},
	)

	if err != nil {
		return nil, err
	}

	var delegationsToReturn []unbondingwatcher.Delegation

	for _, delegation := range delegations.BtcDelegations {

		stakingtx, err := bbn.NewBTCTxFromBytes(
			delegation.StakingTx,
		)

		if err != nil {
			panic(err)
		}

		if delegation.BtcUndelegation != nil {
			unbondingTx, err := bbn.NewBTCTxFromBytes(
				delegation.BtcUndelegation.UnbondingTx,
			)

			if err != nil {
				panic(err)
			}

			delegationsToReturn = append(delegationsToReturn, unbondingwatcher.Delegation{
				StakingTx:             stakingtx,
				StakingOutputIdx:      delegation.StakingOutputIdx,
				DelegationStartHeight: delegation.StartHeight,
				UnbondingOutput:       unbondingTx.TxOut[0],
			})
		}
	}

	return delegationsToReturn, nil
}

func (tbc *TestBabylonClient) IsDelegationActive(stakingTxHash chainhash.Hash) (bool, error) {
	// TODO: Hacky way to simualte babylon having pre-signed unbonding tx. We always retrun try, so that
	// we will submit unbonding tx to BTC chain
	return true, nil
}

func (tbc *TestBabylonClient) ReportUnbonding(stakingTxHash chainhash.Hash, stakerUnbondingSig *schnorr.Signature) error {
	tbc.reportedTxChan <- &ReportedInfo{
		StakingTxHash:      stakingTxHash,
		StakerUnbondingSig: stakerUnbondingSig,
	}
	return nil
}

func (tbc *TestBabylonClient) BtcClientTipHeight() (uint32, error) {
	tipResponse, err := tbc.tm.BabylonClient.BTCHeaderChainTip()

	if err != nil {
		return 0, err
	}

	return uint32(tipResponse.Header.Height), nil
}

func Test_Unbonding_Watcher(t *testing.T) {
	// segwit is activated at height 300. It's needed by staking/slashing tx
	numMatureOutputs := uint32(300)

	submittedTxs := []*chainhash.Hash{}
	blockEventChan := make(chan *types.BlockEvent, 1000)
	handlers := &rpcclient.NotificationHandlers{
		OnFilteredBlockConnected: func(height int32, header *wire.BlockHeader, txs []*btcutil.Tx) {
			log.Debugf("Block %v at height %d has been connected at time %v", header.BlockHash(), height, header.Timestamp)
			blockEventChan <- types.NewBlockEvent(types.BlockConnected, height, header)
		},
		OnFilteredBlockDisconnected: func(height int32, header *wire.BlockHeader) {
			log.Debugf("Block %v at height %d has been disconnected at time %v", header.BlockHash(), height, header.Timestamp)
			blockEventChan <- types.NewBlockEvent(types.BlockDisconnected, height, header)
		},
		OnTxAccepted: func(hash *chainhash.Hash, amount btcutil.Amount) {
			submittedTxs = append(submittedTxs, hash)
		},
	}

	tm := StartManager(t, numMatureOutputs, 2, handlers, blockEventChan)
	// this is necessary to receive notifications about new transactions entering mempool
	err := tm.MinerNode.Client.NotifyNewTransactions(false)
	require.NoError(t, err)
	err = tm.MinerNode.Client.NotifyBlocks()
	require.NoError(t, err)
	defer tm.Stop(t)
	// Insert all existing BTC headers to babylon node
	tm.CatchUpBTCLightClient(t)

	emptyHintCache := unbondingwatcher.EmptyHintCache{}

	// TODO:L our config only support btcd wallet tls, not btcd dierectly
	tm.Config.BTC.DisableClientTLS = false
	backend, err := unbondingwatcher.NewNodeBackend(
		unbondingwatcher.CfgToBtcNodeBackendConfig(tm.Config.BTC, hex.EncodeToString(tm.MinerNode.RPCConfig().Certificates)),
		&chaincfg.SimNetParams,
		&emptyHintCache,
	)
	require.NoError(t, err)

	err = backend.Start()
	require.NoError(t, err)

	watcherCfg := unbondingwatcher.DefaultUnbondingWatcherConfig()
	watcherCfg.CheckDelegationsInterval = 1 * time.Second

	testBabylonClient := NewTestBabylonClient(tm)
	watcher := unbondingwatcher.NewUnbondingWatcher(
		backend,
		testBabylonClient,
		&watcherCfg,
	)
	watcher.Start()
	defer watcher.Stop()

	// set up a BTC validator
	tm.createBTCValidator(t)
	// set up a BTC delegation
	del, sk := tm.createBTCDelegation(t)

	// undelgate
	_, stakerSig := tm.undelegate(t, del, sk)
	stakingTxHash := del.StakingTx.TxHash()

	// wait for unbonding tx to be reported
	reportedInfo := <-testBabylonClient.reportedTxChan
	// check that reported tx is the one from staking tx, as we always identify each
	// each delegation by staking tx hash
	require.True(t, reportedInfo.StakingTxHash.IsEqual(&stakingTxHash))
	require.True(t, reportedInfo.StakerUnbondingSig.IsEqual(stakerSig))
}
