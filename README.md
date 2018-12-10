![](https://gitlab.com/parallelcoin/node/raw/master/assets/logo.png)

# The Parallelcoin Pod [![ISC License](http://img.shields.io/badge/license-ISC-blue.svg)](http://copyfree.org) [![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg)](http://godoc.org/github.com/parallelcointeam/pod)

<!-- [![Build Status](https://travis-ci.org/parallelcointeam/pod.png?branch=master)](https://travis-ci.org/parallelcointeam/pod) -->

Next generation full node for Parallelcoin, forked from [btcd](https://github.com/btcsuite/btcd)

## Stochastic Binomial Filter Difficulty Adjustment

After the upcoming hardfork, Parallelcoin will have the following features in its difficulty adjustment regime

- 9 algorithms can be used when mining:

  - Blake14lr (decred)
  - Lyra2REv2 (sia)
  - Keccac (maxcoin, smartcash)
  - Scrypt (litecoin)
  - SHA256D (bitcoin)
  - Skein (myriadcoin)
  - X11 (dash)
  - X13 (navcoin)

- 293 second blocks (7 seconds less than 5 minutes), 1439 block averaging window (5 seconds less than 24 hours) - Prime numbers are used to reduce resonance caused by aliasing distortion

- Exponential curve with power of 3 to respond gently the natural drift while moving the difficulty fast in below 10% of target and 10x target, to deal with recovering after a large increase in network hashpower

- Difficulty adjustments are based on previous block of each algorithm, meaning sharp rises from one algorithm do not immediately affect the other algorithms, allowing a smoother recovery from a sudden drop in hashrate

- Difficulty reset when the last block from an algorithm is the only one in the averaging window, as a reverse incentive (envy) to keep all 9 algorithms producing blocks within the averaging window

- Deterministic noise is added to the difficulty adjustment in a similar way as is done with digital audio and images to improve the effective resolution of the signal

- To address the issue of recovery after a drop in hashrate, miners have the option of producing a block with zero block reward and one transaction, with a relaxation of target by 1 byte or a factor of 256 (one pair of hex digits in the hash less must be zero). This zero reward block with one tx fee serves the function of resetting the target (based on the previous target and the time), which puts the target back in reach of the remaining miners on the chain.

Modular build for reproducibility and maintenance efficiency, therefore Go >=1.11 is required.

The pod has no RPC wallet functionality, only core chain functions. It is fully compliant with the original [parallelcoind](https://github.com/marcetin/parallelcoin), though it will accept but not relay transactions with big S signatures (theoretical signature malleability double spend attack that enables an output to be spent by two private keys).

It also potentially can enable BIP9 softforks, a module system that makes it possible also for miners to vote for a fork, however this would fork from the legacy client so it would depend on overwhelming adoption of this new client. This also depends on other clients accepting the 'nonstandard' transactions that hardforks might require (witness, for example), which again will cause a fork.

## Installation

For the main full node server:

    go get github.com/parallelcointeam/pod

You probably will also want CLI client (can also speak to other bitcoin protocol RPC endpoints):

    go get github.com/parallelcointeam/pod/cmd/podctl

# pod

pod is an alternative full node bitcoin implementation written in Go (golang).

This project is currently under active development and is in a Beta state. It
is extremely stable and has been in production use since October 2013.

It properly downloads, validates, and serves the block chain using the exact
rules (including consensus bugs) for block acceptance as Bitcoin Core. We have
taken great care to avoid pod causing a fork to the block chain. It includes a
full block validation testing framework which contains all of the 'official'
block acceptance tests (and some additional ones) that is run on every pull
request to help ensure it properly follows consensus. Also, it passes all of
the JSON test data in the Bitcoin Core code.

It also properly relays newly mined blocks, maintains a transaction pool, and
relays individual transactions that have not yet made it into a block. It
ensures all individual transactions admitted to the pool follow the rules
required by the block chain and also includes more strict checks which filter
transactions based on miner requirements ("standard" transactions).

One key difference between pod and Bitcoin Core is that pod does _NOT_ include
wallet functionality and this was a very intentional design decision. See the
blog entry [here](https://blog.conformal.com/pod-not-your-moms-bitcoin-daemon)
for more details. This means you can't actually make or receive payments
directly with pod. That functionality is provided by the
[btcwallet](https://github.com/btcsuite/btcwallet) and
[Paymetheus](https://github.com/btcsuite/Paymetheus) (Windows-only) projects
which are both under active development.

## Requirements

[Go](http://golang.org) 1.8 or newer.

## Installation

#### Windows - MSI Available

https://github.com/parallelcointeam/pod/releases

#### Linux/BSD/MacOSX/POSIX - Build from Source

- Install Go according to the installation instructions here:
  http://golang.org/doc/install

- Ensure Go was installed properly and is a supported version:

```bash
$ go version
$ go env GOROOT GOPATH
```

NOTE: The `GOROOT` and `GOPATH` above must not be the same path. It is
recommended that `GOPATH` is set to a directory in your home directory such as
`~/goprojects` to avoid write permission issues. It is also recommended to add
`$GOPATH/bin` to your `PATH` at this point.

- Run the following commands to obtain pod, all dependencies, and install it:

```bash
$ go get -u github.com/Masterminds/glide
$ git clone https://github.com/parallelcointeam/pod $GOPATH/src/github.com/parallelcointeam/pod
$ cd $GOPATH/src/github.com/parallelcointeam/pod
$ glide install
$ go install . ./cmd/...
```

- pod (and utilities) will now be installed in `$GOPATH/bin`. If you did
  not already add the bin directory to your system path during Go installation,
  we recommend you do so now.

## Updating

#### Windows

Install a newer MSI

#### Linux/BSD/MacOSX/POSIX - Build from Source

- Run the following commands to update pod, all dependencies, and install it:

```bash
$ cd $GOPATH/src/github.com/parallelcointeam/pod
$ git pull && glide install
$ go install . ./cmd/...
```

## Getting Started

pod has several configuration options available to tweak how it runs, but all
of the basic operations described in the intro section work with zero
configuration.

#### Windows (Installed from MSI)

Launch pod from your Start menu.

#### Linux/BSD/POSIX/Source

```bash
$ ./pod
```

## IRC

- irc.freenode.net
- channel #pod
- [webchat](https://webchat.freenode.net/?channels=pod)

## Issue Tracker

The [integrated github issue tracker](https://github.com/parallelcointeam/pod/issues)
is used for this project.

## Documentation

The documentation is a work-in-progress. It is located in the [docs](https://github.com/parallelcointeam/pod/tree/master/docs) folder.

## GPG Verification Key

All official release tags are signed by Conformal so users can ensure the code
has not been tampered with and is coming from the btcsuite developers. To
verify the signature perform the following:

- Download the Conformal public key:
  https://raw.githubusercontent.com/parallelcointeam/pod/master/release/GIT-GPG-KEY-conformal.txt

- Import the public key into your GPG keyring:

  ```bash
  gpg --import GIT-GPG-KEY-conformal.txt
  ```

- Verify the release tag with the following command where `TAG_NAME` is a
  placeholder for the specific tag:
  ```bash
  git tag -v TAG_NAME
  ```

## License

pod is licensed under the [copyfree](http://copyfree.org) ISC License.
