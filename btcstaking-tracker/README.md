# BTC staking tracker

This package provides the implementation of the BTC staking tracker.

- [Overview](#overview)
- [Dependencies](#dependencies)
- [Routines](#routines)
  - [Unbonding watcher routine](#unbonding-watcher-routine)
  - [BTC slasher routine](#btc-slasher-routine)
  - [Atomic slasher routine](#atomic-slasher-routine)

## Overview

BTC staking tracker is a daemon program that relays information between Babylon
and Bitcoin for facilitating the Bitcoin staking protocol. This includes three
routines:

- [Unbonding watcher routine](./unbondingwatcher/): upon observing an unbonding
  transaction of a BTC delegation on Bitcoin, the routine reports the
  transaction's signature from the staker to Babylon, such that Babylon will
  unbond the BTC delegation on its side.
- [BTC slasher](./btcslasher/): upon observing a slashable offence launched by a
  finality provider, the routine slashes the finality provider and its BTC
  delegations.
- [Atomic slasher](./atomicslasher/): upon observing a selective slashing
  offence where the finality provider maliciously signs and submits a BTC
  delegation's slashing transaction to Bitcoin, the routine reports the offence
  to BTC slasher and Babylon.

## Dependencies

The BTC staking tracker includes the following dependencies:

- A BTC client for querying Bitcoin blocks/transactions.
- A BTC notifier for getting new events (mainly new BTC blocks) in Bitcoin.
- A Babylon client for submitting transactions to and querying Babylon.

## Routines

### Unbonding watcher routine

The unbonding watcher routine aims to notify Babylon about early unbonding
events of BTC delegations. It includes the following subroutines:

- `handleNewBlocks` routine: Upon each new BTC block, save its height as the
  Bitcoin's tip height
- `fetchDelegations` routine: Periodically, fetch all active BTC delegations
  from Babylon, and send them to the `handleDelegations` routine.
- `handleDelegations` routine:
  - Upon each new BTC delegation, spawn a new goroutine watching the event that
    the BTC delegation's unbonding transaction occurs on Bitcoin.
  - Upon a BTC delegation is expired, stop tracking the BTC delegation.
- Upon seeing an unbonding transaction of a BTC delegation on Bitcoin, notify
  Babylon that the corresponding BTC delegation is unbonded via a
  `MsgBTCUndelegate` message.

### BTC slasher routine

The BTC slasher routine aims to slash adversarial finality providers and their
BTC delegations. The slashable offences launched by finality providers include

- *Equivocation* where the finality provider signs two conflicting blocks at the
same height, and
- *Selective slashing* where the finality provider signs the slashing
  transaction of a victim BTC delegation and submits it to Bitcoin.

The BTC slasher includes the following subroutines:

- a routine that subscribes to Babylon and forwards all events about finality
  signatures to the `equivocationTracker` routine.
- `equivocationTracker` routine: Upon an event of a finality signature,
  1. Check if the event contains an evidence of equivocation.
  2. If yes, then forward the event to the use the extractable one-time
     signature's extractability property to extract the equivocating finality
     provider's secret key.
  3. Forward the secret key to the `slashingEnforcer` routine.
- `slashingEnforcer` routine: upon a slashed finality provider's secret key,
  1. Find all BTC delegations that are active or unbonding under this finality
     provider
  2. Try to submit the slashing and unbonding slashing transactions of these BTC
     delegations to Bitcoin.

### Atomic slasher routine

The atomic slasher routine aims to slash finality providers that have conducted
selective slashing offences. In a selective slashing offence, the finality
provider maliciously signs and submits a BTC delegation's slashing transaction
to Bitcoin, without affecting other BTC delegations. Babylon circumvents the
selective slashing offences by enforcing the *atomic slashing* property: if one
BTC delegation is slashed, all other BTC delegations under this finality
provider will be slashed as well. This is achieved by using a cryptographic
primitive called [adaptor
signature](https://bitcoinops.org/en/topics/adaptor-signatures/). The atomic
slasher includes the following routines:

<!-- TODO: more technical details about atomic slashing via adaptor signatures -->

- `btcDelegationTracker` routine: periodically retrieves all BTC delegations and
  saves them to a `BTCDelegationIndex` cache.
- `slashingTxTracker` routine: upon a BTC block,
  1. For each transaction, check whether it is a slashing transaction in the
     `BTCDelegationIndex` cache.
  2. If a transaction is identified as a slashing transaction, send it to the
     `selectiveSlashingReporter` routine.
- `selectiveSlashingReporter` routine: upon a slashing transaction,
  2. Retrieve the BTC delegation and its finality provider from Babylon.
  3. If the finality provider is slashed, skip this BTC delegation.
  4. Try to extract the finality provider's secret key by comparing the covenant
     Schnorr signatures in the slashing transaction and the covenant adaptor
     signatures.
  5. If successful, then report the selective slashing offence to Babylon, and
     forward the extracted secret key to the BTC slasher routine.
