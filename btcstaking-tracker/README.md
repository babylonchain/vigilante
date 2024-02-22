# BTC staking tracker

This package provides the implementation of the BTC staking tracker.

- [Overview](#overview)
- [Routines](#routines)
  - [Unbonding watcher routine](#unbonding-watcher-routine)
  - [BTC slasher routine](#btc-slasher-routine)
  - [Atomic slasher routine](#atomic-slasher-routine)

## Overview

BTC staking tracker is a daemon program that relays information between Babylon and Bitcoin for facilitating the Bitcoin staking protocol.
This includes three routines:

- [Unbonding watcher routine](./unbondingwatcher/): upon observing an unbonding transaction of a BTC delegation on Bitcoin, the routine reports the transaction's signature from the staker to Babylon, such that Babylon will unbond the BTC delegation on its side.
- [BTC slasher](./btcslasher/): upon observing a slashable offence launched by a finality provider, the routine slashes the finality provider and its BTC delegations.
- [Atomic slasher](./atomicslasher/): upon observing a selective slashing offence where the finality provider maliciously signs and submits a BTC delegation's slashing transaction to Bitcoin, the routine reports the offence to BTC slasher and Babylon.

## Routines

### Unbonding watcher routine

### BTC slasher routine

### Atomic slasher routine
