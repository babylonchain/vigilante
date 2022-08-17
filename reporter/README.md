# reporter

This package implements the vigilant reporter. The vigilant reporter is responsible for

- syncing the latest BTC blocks with a BTC node
- extracting headers and checkpoints from BTC blocks
- forwarding headers and checkpoints to a Babylon node
- detecting and reporting inconsistency between BTC blockchain and Babylon BTCLightclient header chain
- detecting and reporting stalling attacks where a checkpoint is w-deep on BTC but Babylon hasn't included its k-deep proof

The code is adapted from https://github.com/btcsuite/btcwallet/tree/master/wallet.