btcd
====

[![ISC License](http://img.shields.io/badge/license-ISC-blue.svg)](http://copyfree.org)
[![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg)](http://godoc.org/github.com/parallelcointeam/pod)

pod is an new full node implementation of parallelcoind written in Go (golang).

It is forked from btcd, and altered to support the multi-PoW and difficulty
adjustment consensus rules of the parallelcoin network.

In the `cmd/` directory you can find numerous tools, the two most important are
`pod` and `podctl`, which is the full node itself, and a CLI client that allows
you to send and receive responses from a full node via its JSONRPC interface.

Rather than keep separate repositories, and because there is such a massive
overlap between shared code, you will find the beginnings of a wallet client also.

The wallet is also a free-standing node, but stores a far reduced subset of the 
blockchain data, sufficient to provide the information required for a wallet, and
most of the information required to run a blockchain explorer or other app that needs 

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

NOTE: The `GOROOT` and `GOPATH` above must not be the same path.  It is
recommended that `GOPATH` is set to a directory in your home directory such as
`~/goprojects` to avoid write permission issues.  It is also recommended to add
`$GOPATH/bin` to your `PATH` at this point.

- Run the following commands to obtain btcd, all dependencies, and install it:

```bash
$ go get -u github.com/Masterminds/glide
$ git clone https://github.com/parallelcointeam/pod $GOPATH/src/github.com/parallelcointeam/pod
$ cd $GOPATH/src/github.com/parallelcointeam/pod
$ glide install
$ go install . ./cmd/...
```

- btcd (and utilities) will now be installed in ```$GOPATH/bin```.  If you did
  not already add the bin directory to your system path during Go installation,
  we recommend you do so now.

## Updating

#### Windows

Install a newer MSI

#### Linux/BSD/MacOSX/POSIX - Build from Source

- Run the following commands to update btcd, all dependencies, and install it:

```bash
$ cd $GOPATH/src/github.com/parallelcointeam/pod
$ git pull && glide install
$ go install . ./cmd/...
```

## Getting Started

btcd has several configuration options available to tweak how it runs, but all
of the basic operations described in the intro section work with zero
configuration.

#### Windows (Installed from MSI)

Launch btcd from your Start menu.

#### Linux/BSD/POSIX/Source

```bash
$ ./btcd
```

## IRC

- irc.freenode.net
- channel #btcd
- [webchat](https://webchat.freenode.net/?channels=btcd)

## Issue Tracker

The [integrated github issue tracker](https://github.com/parallelcointeam/pod/issues)
is used for this project.

## Documentation

The documentation is a work-in-progress.  It is located in the [docs](https://github.com/parallelcointeam/pod/tree/master/docs) folder.

## GPG Verification Key

All official release tags are signed by Conformal so users can ensure the code
has not been tampered with and is coming from the btcsuite developers.  To
verify the signature perform the following:

- Download the Conformal public key:
  https://raw.githubusercontent.com/btcsuite/btcd/master/release/GIT-GPG-KEY-conformal.txt

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

btcd is licensed under the [copyfree](http://copyfree.org) ISC License.
