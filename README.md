# IcyChecker

This repository contains a preliminary version of IcyChecker, a state inconsisitency bug checker for Ethereum smart contracts. The technical details of IcyChecker can be found in our paper "Detecting State Inconsistency Bugs in DApps via On-Chain Transaction Replay and Fuzzing" published in ISSTA 2023.

# Installation

IcyChecker is build on top of geth. Environment of Go is necessary

```
git clone https://github.com/MingxiYe/IcyChecker.git
cd IcyChecker
make all
```

# Usage

This component allow users to collect on-chain data through geth and detect SI bugs through on-chain transaction replay and fuzzing.

## Sync and Export Blockchain in Files

Syncing is the process by which geth catches up to the latest Ethereum block and current global state. There are several ways to sync a Geth node that differ in their speed, storage requirements and trust assumptions. IcyChecker requires `snap` mode.

```bash
# Build the source
make geth

./build/bin/geth --datadir <geth-datadir> --syncmode snap --gcmode archive

# export blockchain data from 13,000,001 to 14,000,000 (total 1M blocks)
./build/bin/geth --datadir <geth-datadir> --syncmode snap --gcmode archive export 13-14M.blockchain 13000001 14000000
```

## Contextual-information Collection (CIC)

IcyChecker collect historical information by replaying blockchain files exported from the previous step. Part of CIC is realized through the following instructions.

```bash
# Build the source
make geth

# replay and collect
./build/bin/geth --datadir <path-to-recorder-datadir> import <path-to-13-14M.blockchain>
```

## Transaction Sequence Generation & Mutation (TSG & TSM)

IcyChecker generates a set of feasible transaction sequence and perform differential analysis.

`substate-cli replay-SI` executes transaction substates in a given block range.

For example, if you want to replay transactions from block 13,000,001 to block 14,000,000 in `substate.ethereum`:
```bash
# Build the source
make all

./build/bin/substate-cli replay-mt 13000001 14000000 --dappDir <path-to-dappDir> --substateDir <path-to-recorder-datadir>
```

Here are command line options for `substate-cli replay-SI`:
```
replay-SI <blockNumFirst> <blockNumLast>  --dappDir <path-to-dapp.dir> --substateDir <path-to-recorder.datadir> [command options]

The substate-cli replay-SI command requires four arguments:
<blockNumFirst> <blockNumLast> <path-to-dapp.dir> <path-to-recorder.datadir>

<blockNumFirst> and <blockNumLast> are the first and
last block of the inclusive range of blocks to replay transactions.

<path-to-dapp.dir> and <path-to-recorder.datadir> are the path of dapp data folder 
and the path of substate that previously replay`

OPTIONS:
   --workers value           Number of worker threads that execute in parallel (default: 2)
   --rich-info               Collect historical information to enhance fuzzing
```