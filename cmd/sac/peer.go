package main

import (
	"github.com/parallelcointeam/pod/chaincfg"
	"github.com/parallelcointeam/pod/peer"
	"github.com/parallelcointeam/pod/wire"
)

var defConf = peer.Config{
	UserAgentName:    "Sac",
	UserAgentVersion: Version(),
	ChainParams:      &chaincfg.TestNet3Params,
	Services:         wire.SFTaCNode | wire.SFNodeNetwork,
	ProtocolVersion:  wire.TaCVersion,
	TrickleInterval:  peer.DefaultTrickleInterval,
}
