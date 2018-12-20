package main
import (
	"github.com/parallelcointeam/pod/chaincfg"
	"github.com/parallelcointeam/pod/wire"
)
// activeNetParams is a pointer to the parameters specific to the
// currently active bitcoin network.
var activeNetParams = &mainNetParams
// params is used to group parameters for various networks such as the main
// network and test networks.
type params struct {
	*chaincfg.Params
	rpcPort            string
	ScryptRPCPort      string
	WhirlpoolListeners string
	Blake14lrRPCPort   string
	KeccakRPCPort      string
	Lyra2rev2RPCPort   string
	SkeinRPCPort       string
	X11RPCPort         string
	GostRPCPort        string
}
// mainNetParams contains parameters specific to the main network
// (wire.MainNet).  NOTE: The RPC port is intentionally different than the
// reference implementation because pod does not handle wallet requests.  The
// separate wallet process listens on the well-known port and forwards requests
// it does not handle on to pod.  This approach allows the wallet process
// to emulate the full reference implementation RPC API.
var mainNetParams = params{
	Params:             &chaincfg.MainNetParams,
	rpcPort:            "11048",
	ScryptRPCPort:      "11049",
	WhirlpoolListeners: "11050",
	Blake14lrRPCPort:   "11051",
	KeccakRPCPort:      "11052",
	Lyra2rev2RPCPort:   "11053",
	SkeinRPCPort:       "11054",
	X11RPCPort:         "11055",
	GostRPCPort:        "11056",
}
// regressionNetParams contains parameters specific to the regression test
// network (wire.TestNet).  NOTE: The RPC port is intentionally different
// than the reference implementation - see the mainNetParams comment for
// details.
var regressionNetParams = params{
	Params:             &chaincfg.RegressionNetParams,
	rpcPort:            "31048",
	ScryptRPCPort:      "31049",
	WhirlpoolListeners: "31050",
	Blake14lrRPCPort:   "31051",
	KeccakRPCPort:      "31052",
	Lyra2rev2RPCPort:   "31053",
	SkeinRPCPort:       "31054",
	X11RPCPort:         "31055",
	GostRPCPort:        "31056",
}
// testNet3Params contains parameters specific to the test network (version 3)
// (wire.TestNet3).  NOTE: The RPC port is intentionally different than the
// reference implementation - see the mainNetParams comment for details.
var testNet3Params = params{
	Params:             &chaincfg.TestNet3Params,
	rpcPort:            "21048",
	ScryptRPCPort:      "21049",
	WhirlpoolListeners: "21050",
	Blake14lrRPCPort:   "21051",
	KeccakRPCPort:      "21052",
	Lyra2rev2RPCPort:   "21053",
	SkeinRPCPort:       "21054",
	X11RPCPort:         "21055",
	GostRPCPort:        "21056",
}
// simNetParams contains parameters specific to the simulation test network
// (wire.SimNet).
var simNetParams = params{
	Params:             &chaincfg.SimNetParams,
	rpcPort:            "41048",
	ScryptRPCPort:      "41049",
	WhirlpoolListeners: "41050",
	Blake14lrRPCPort:   "41051",
	KeccakRPCPort:      "41052",
	Lyra2rev2RPCPort:   "41053",
	SkeinRPCPort:       "41054",
	X11RPCPort:         "41055",
	GostRPCPort:        "41056",
}
// netName returns the name used when referring to a bitcoin network.  At the
// time of writing, pod currently places blocks for testnet version 3 in the
// data and log directory "testnet", which does not match the Name field of the
// chaincfg parameters.  This function can be used to override this directory
// name as "testnet" when the passed active network matches wire.TestNet3.
// A proper upgrade to move the data and log directories for this network to
// "testnet3" is planned for the future, at which point this function can be
// removed and the network parameter's name used instead.
func netName(chainParams *params) string {
	switch chainParams.Net {
	case wire.TestNet3:
		return "testnet"
	default:
		return chainParams.Name
	}
}
