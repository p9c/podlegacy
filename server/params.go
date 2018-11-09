// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package server

import (
	"github.com/parallelcointeam/pod/chaincfg"
	"github.com/parallelcointeam/pod/wire"
)

// ActiveNetParams is a pointer to the parameters specific to the
// currently active bitcoin network.
var ActiveNetParams = &MainNetParams

// Params is used to group parameters for various networks such as the main
// network and test networks.
type Params struct {
	*chaincfg.Params
	RPCPort string
}

// MainNetParams contains parameters specific to the main network
// (wire.MainNet).  NOTE: The RPC port is intentionally different than the
// reference implementation because btcd does not handle wallet requests.  The
// separate wallet process listens on the well-known port and forwards requests
// it does not handle on to btcd.  This approach allows the wallet process
// to emulate the full reference implementation RPC API.
var MainNetParams = Params{
	Params:  &chaincfg.MainNetParams,
	RPCPort: "11048",
}

// RegressionNetParams contains parameters specific to the regression test
// network (wire.TestNet).  NOTE: The RPC port is intentionally different
// than the reference implementation - see the MainNetParams comment for
// details.
var RegressionNetParams = Params{
	Params:  &chaincfg.RegressionNetParams,
	RPCPort: "31048",
}

// TestNet3Params contains parameters specific to the test network (version 3)
// (wire.TestNet3).  NOTE: The RPC port is intentionally different than the
// reference implementation - see the MainNetParams comment for details.
var TestNet3Params = Params{
	Params:  &chaincfg.TestNet3Params,
	RPCPort: "21048",
}

// SimNetParams contains parameters specific to the simulation test network
// (wire.SimNet).
var SimNetParams = Params{
	Params:  &chaincfg.SimNetParams,
	RPCPort: "41048",
}

// NetName returns the name used when referring to a bitcoin network.  At the
// time of writing, btcd currently places blocks for testnet version 3 in the
// data and log directory "testnet", which does not match the Name field of the
// chaincfg parameters.  This function can be used to override this directory
// name as "testnet" when the passed active network matches wire.TestNet3.
//
// A proper upgrade to move the data and log directories for this network to
// "testnet3" is planned for the future, at which point this function can be
// removed and the network parameter's name used instead.
func NetName(chainParams *Params) string {
	switch chainParams.Net {
	case wire.TestNet3:
		return "testnet"
	default:
		return chainParams.Name
	}
}