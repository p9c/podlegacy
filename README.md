# Parallelcoin Pod - complete base package for the Parallelcoin network

Parallelcoin is a bitcoin fork which adds the ability to be mined by more than
one proof of work algorithm. Specifically it currently supports SHA256D and
Scrypt, but more are planned for an upcoming hardfork.

Parallelcoin development is moving in the direction of expanding the connectivity
of nodes and creating a mesh network upon which many other applications can be
linked together. Atomic swaps, decentralised, federated and democratic applications
like distributed exchanges, messaging systems, marketplaces, and file sharing
applications will become possible.

Currently there is now a fully functional full node, which can be used to mine DUO
or support any application needing raw blockchain data.

The immediate current development work is focusing on building a light wallet node
that protects privacy by using a form of onion routing for message broadcasts where
the swarm of wallet users route each other's transactions to prevent geographical
location of holders of the secret keys. In order to implement this, an
interconnection protocol will be developed that enables also remote control of 
virtually anything that can be linked to the node, and the discovery of service
providers on the network including detailed information about the enabled API calls
that they support.

## Building

Pretty much you can install it directly, once you have a functioning Go installation,
like this:

    go get github.com/parallelcointeam/duo/cmd/pod

to install 'pod' which will then be available in your GOBIN folder (defaults to 
home/go/bin), and you can run it and it will immediately start synchronising to the
network.

The `podctl` program allows you to send and receive JSONRPC requests, and can be
installed like this:

    go get github.com/parallelcointeam/duo/cmd/podctl

Use the argument `--help` to get information about available commands and configuration.

In `cmd/pod/sample-pod.conf` you find a full annotated configuration, copy this to the
folder `home/.pod/pod.conf` to configure the launch settings. `podctl` also has a 
configuration that you can find in `home/.podctl`, which allows you to set the RPC
endpoint and other things.