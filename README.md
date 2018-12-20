![](https://gitlab.com/parallelcoin/node/raw/master/assets/logo.png)

# The Parallelcoin Pod [![ISC License](http://img.shields.io/badge/license-ISC-blue.svg)](http://copyfree.org) [![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg)](http://godoc.org/github.com/parallelcointeam/pod)

<!-- [![Build Status](https://travis-ci.org/parallelcointeam/pod.png?branch=master)](https://travis-ci.org/parallelcointeam/pod) -->

Next generation full node for Parallelcoin, forked from [btcd](https://github.com/btcsuite/btcd)

## Stochastic Binomial Filter Difficulty Adjustment

After the upcoming hardfork, Parallelcoin will have the following features in its difficulty adjustment regime

- 9 algorithms can be used when mining:

  - Blake14lr (decred)
  - GOST Stribog \*
  - Lyra2REv2 (sia)
  - Keccac (maxcoin, smartcash)
  - Scrypt (litecoin)
  - SHA256D (bitcoin)
  - Skein (myriadcoin)
  - Whirlpool \*
  - X11 (dash)

- 293 second blocks (7 seconds less than 5 minutes), 1439 block averaging window (5 seconds less than 24 hours) - Prime numbers are used to reduce resonance caused by aliasing distortion

- Exponential curve with power of 3 to respond gently the natural drift while moving the difficulty fast in below 10% of target and 10x target, to deal with recovering after a large increase in network hashpower

- Difficulty adjustments are based on previous block of each algorithm, meaning sharp rises from one algorithm do not immediately affect the other algorithms, allowing a smoother recovery from a sudden drop in hashrate

- Deterministic noise is added to the difficulty adjustment in a similar way as is done with digital audio and images to improve the effective resolution of the signal

  The pod has no RPC wallet functionality, only core chain functions. It is fully compliant with the original [parallelcoind](https://github.com/marcetin/parallelcoin) but will be switching over to its first Hard Fork at a future appointed block height in the official release. For the wallet server, which works also with the CLI controller `podctl`, it will be possible to send commands to both mod (wallet) and pod full node using the command line.

  A Webview/Golang based GUI wallet will come a little later, following the release, and will be able to run on all platforms with with browser or supported built-in web application platforms, Blink and Webkit engines.

  ## Hard Fork 1: Plan 9 from Crypto Space

  At the time of release there will not be any GPU nor ASIC miners for the GOST Stribog (just stribog 256 bit hash, not combined) and whirlpool (only coin with this is long dead but it is a commonly used disk encryption hash function so likely there may be some on the network at the fork).

  ASIC miners can be pleased to hear that it runs SHA256D, Scrypt and X11, and though not so many ASICs exist yet, for Lyra2REv2, Skein, Keccac, and Blake14lr, which will have been proven in testing to work with multi-algo miners such as ccminer, sgminer and bfgminer.

  Possibly some blocks can be found by CPU miners for a few days. A multi-algo GPU miner that automatically recognises and configures and enables multi-mining based on benchmarks and difficulty adjustment has been scheduled for the next major work after the full Parallelcoin suite is completed.

## Installation

For the main full node server:

```bash
go get github.com/parallelcointeam/pod
```

You probably will also want CLI client (can also speak to other bitcoin protocol RPC endpoints also):

```bash
go get github.com/parallelcointeam/pod/cmd/podctl
```

## Requirements

[Go](http://golang.org) 1.11 or newer.

## Installation

#### Windows - MSI Available

https://github.com/parallelcointeam/pod/releases

#### Linux/BSD/MacOSX/POSIX - Build from Source

- Install Go according to the [installation instructions](http://golang.org/doc/install)
- Ensure Go was installed properly and is a supported version:

```bash
$ go version
$ go env GOROOT GOPATH
```

NOTE: The `GOROOT` and `GOPATH` above must not be the same path. It is recommended that `GOPATH` is set to a directory in your home directory such as `~/goprojects` to avoid write permission issues. It is also recommended to add `$GOPATH/bin` to your `PATH` at this point.

- Run the following commands to obtain pod, all dependencies, and install it:

```bash
$ go get github.com/parallelcointeam/pod
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

pod has several configuration options available to tweak how it runs, but all of the basic operations described in the intro section work with zero configuration.

#### Windows (Installed from MSI)

Launch pod from your Start menu.

#### Linux/BSD/POSIX/Source

```bash
$ ./pod
```

## Discord

Come and chat at our (discord server](https://discord.gg/nJKts94)

## Issue Tracker

The [integrated github issue tracker](https://github.com/parallelcointeam/pod/issues)
is used for this project.

## Documentation

The documentation is a work-in-progress. It is located in the [docs](https://github.com/parallelcointeam/pod/tree/master/docs) folder.

## License

pod is licensed under the [copyfree](http://copyfree.org) ISC License.
