package btcclient

import (
	"fmt"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/wire"

	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/netparams"
	"github.com/babylonchain/vigilante/types"
)

// NewWallet creates a new BTC wallet
// used by vigilant submitter
// a wallet is essentially a BTC client
// that connects to the btcWallet daemon
func NewWallet(cfg *config.BTCConfig) (*Client, error) {
	params, err := netparams.GetBTCParams(cfg.NetParams)
	if err != nil {
		return nil, err
	}
	wallet := &Client{}
	wallet.Cfg = cfg
	wallet.Params = params

	connCfg := &rpcclient.ConnConfig{}
	switch cfg.BtcBackend {
	case types.Bitcoind:
		connCfg = &rpcclient.ConnConfig{
			// this will work with node loaded with multiple wallets
			Host:         cfg.Endpoint + "/wallet/" + cfg.WalletName,
			HTTPPostMode: true,
			User:         cfg.Username,
			Pass:         cfg.Password,
			DisableTLS:   cfg.DisableClientTLS,
			Params:       params.Name,
		}
	case types.Btcd:
		connCfg = &rpcclient.ConnConfig{
			Host:         cfg.WalletEndpoint,
			Endpoint:     "ws", // websocket
			User:         cfg.Username,
			Pass:         cfg.Password,
			DisableTLS:   cfg.DisableClientTLS,
			Params:       params.Name,
			Certificates: readWalletCAFile(cfg),
		}
	}

	rpcClient, err := rpcclient.New(connCfg, nil) // TODO: subscribe to wallet stuff?
	if err != nil {
		return nil, err
	}
	log.Info("Successfully connected to the BTC wallet server")

	wallet.Client = rpcClient

	return wallet, nil
}

// GetTxFee returns tx fee according to its size
// if tx size is zero, it returns the default tx
// fee in config
func (c *Client) GetTxFee(txSize uint64) uint64 {
	var (
		feeRate float64 // BTC/kB
		err     error
	)

	// use MaxTxFee by default
	defaultFee := c.GetMaxTxFee(txSize)

	// if estimatesmartfee is not supported, we use estimatefee instead
	estimateRes, err := c.Client.EstimateSmartFee(c.Cfg.TargetBlockNum, &btcjson.EstimateModeEconomical)
	if err == nil {
		if estimateRes.FeeRate == nil {
			feeRate = 0
		} else {
			feeRate = *estimateRes.FeeRate
			log.Logger.Debugf("fee rate is calculated by estimatesmartfee: %v", feeRate)
		}
	} else {
		log.Logger.Debugf("the btc backend does not support EstimateSmartFee, so EstimateFee is used: %s", err.Error())
		feeRate, err = c.Client.EstimateFee(c.Cfg.TargetBlockNum)
		if err != nil {
			log.Logger.Debugf("failed to estimate fee using EstimateFee, using default fee %v instead: %s",
				defaultFee, err.Error())
			return defaultFee
		}
		log.Logger.Debugf("fee rate is calculated by estimatefee: %v", feeRate)
	}

	// convert the fee rate from BTC/kB to Satoshi/kB for better readability in logs
	feeRateAmount, err := btcutil.NewAmount(feeRate)
	if err != nil {
		log.Logger.Debugf("invalid estimated fee, using default fee %v instead: %s", defaultFee, err.Error())
		return defaultFee
	}

	fee, err := CalculateTxFee(feeRateAmount, txSize)
	if err != nil {
		log.Logger.Debugf("failed to calculate fee, using default fee %v instead: %s", defaultFee, err.Error())
		return defaultFee
	}

	log.Logger.Debugf("the calculated fee is %v based on its size: %v", fee, txSize)
	maxTxFee := c.GetMaxTxFee(txSize)
	if fee > maxTxFee {
		log.Logger.Debugf("the calculated fee %v is higher than MaxTxFee %v, using MaxTxFee instead",
			fee, maxTxFee)
		return maxTxFee
	}
	minTxFee := c.GetMinTxFee(txSize)
	if fee < minTxFee {
		log.Logger.Debugf("the calculated fee %v is lower than MinTxFee %v, using MinTxFee instead",
			fee, minTxFee)
		return minTxFee
	}

	return fee
}

func (c *Client) GetMaxTxFee(size uint64) uint64 {
	return uint64(c.Cfg.TxFeeMax.MulF64(float64(size)))
}

func (c *Client) GetMinTxFee(size uint64) uint64 {
	return uint64(c.Cfg.TxFeeMin.MulF64(float64(size)))
}

func (c *Client) GetWalletName() string {
	return c.Cfg.WalletName
}

func (c *Client) GetWalletPass() string {
	return c.Cfg.WalletPassword
}

func (c *Client) GetWalletLockTime() int64 {
	return c.Cfg.WalletLockTime
}

func (c *Client) GetNetParams() *chaincfg.Params {
	net, err := netparams.GetBTCParams(c.Cfg.NetParams)
	if err != nil {
		panic(fmt.Errorf("failed to get BTC network params: %w", err))
	}
	return net
}

func (c *Client) ListUnspent() ([]btcjson.ListUnspentResult, error) {
	return c.Client.ListUnspent()
}

func (c *Client) SendRawTransaction(tx *wire.MsgTx, allowHighFees bool) (*chainhash.Hash, error) {
	return c.Client.SendRawTransaction(tx, allowHighFees)
}

func (c *Client) ListReceivedByAddress() ([]btcjson.ListReceivedByAddressResult, error) {
	return c.Client.ListReceivedByAddress()
}

func (c *Client) GetRawChangeAddress(account string) (btcutil.Address, error) {
	return c.Client.GetRawChangeAddress(account)
}

func (c *Client) WalletPassphrase(passphrase string, timeoutSecs int64) error {
	return c.Client.WalletPassphrase(passphrase, timeoutSecs)
}

func (c *Client) DumpPrivKey(address btcutil.Address) (*btcutil.WIF, error) {
	return c.Client.DumpPrivKey(address)
}

// GetHighUTXO returns the UTXO that has the highest amount
func (c *Client) GetHighUTXOAndSum() (*btcjson.ListUnspentResult, float64, error) {
	utxos, err := c.ListUnspent()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list unspent UTXOs: %w", err)
	}
	if len(utxos) == 0 {
		return nil, 0, fmt.Errorf("lack of spendable transactions in the wallet")
	}

	highUTXO := utxos[0] // freshest UTXO
	sum := float64(0)
	for _, utxo := range utxos {
		if highUTXO.Amount < utxo.Amount {
			highUTXO = utxo
		}
		sum += utxo.Amount
	}
	return &highUTXO, sum, nil
}

// CalculateTxFee calculates tx fee based on the given fee rate (BTC/kB) and the tx size
func CalculateTxFee(feeRateAmount btcutil.Amount, size uint64) (uint64, error) {
	return uint64(feeRateAmount.MulF64(float64(size) / 1024)), nil
}
