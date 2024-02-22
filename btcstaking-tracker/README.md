# BTC staking tracker

This package provides the implementation of the BTC staking tracker.

- [Overview](#overview)
- [Dependencies](#dependencies)
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

## Dependencies

The BTC staking tracker includes the following dependencies:

- A BTC client for querying Bitcoin blocks/transactions.
- A BTC notifier for getting new events (mainly new BTC blocks) in Bitcoin.
- A Babylon client for submitting transactions to and querying Babylon.

## Routines

### Unbonding watcher routine

The unbonding watcher routine aims to notify Babylon about early unbonding events of BTC delegations.
It includes the following subroutines:

- `handleNewBlocks` routine: Upon each new BTC block, save its height as the Bitcoin's tip height
- `fetchDelegations` routine: Periodically, fetch all active BTC delegations from Babylon, and send them to the `handleDelegations` routine.
- `handleDelegations` routine:
  - Upon each new BTC delegation, spawn a new goroutine watching the event that the BTC delegation's unbonding transaction occurs on Bitcoin.
  - Upon a BTC delegation is expired, stop tracking the BTC delegation.
- Upon seeing an unbonding transaction of a BTC delegation on Bitcoin, notify Babylon that the corresponding BTC delegation is unbonded via a `MsgBTCUndelegate` message.

### BTC slasher routine

The BTC slasher routine aims to slash adversarial finality providers and their BTC delegations.
The slashable offences launched by finality providers include 1) *Equivocation* where the finality provider signs two conflicting blocks at the same height, and 2) *Selecive slashing* where the finality provider signs the slashing transaction of a victim BTC delegation and submits it to Bitcoin.
The BTC slasher includes the following subroutines:

### Atomic slasher routine
