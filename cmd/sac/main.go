package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/parallelcointeam/pod/chaincfg"

	"github.com/parallelcointeam/pod/neutrinoclient"

	"github.com/parallelcointeam/pod/neutrino"
	"github.com/parallelcointeam/pod/walletdb"
	_ "github.com/parallelcointeam/pod/walletdb/bdb"
)

func main() {
	config, _, err := LoadConfig()
	if err != nil {
		fmt.Println("ERROR", err)
		os.Exit(1)
	}

	fmt.Println("connect peers", config.ConnectPeers)
	fmt.Println("add peers", config.AddPeers)

	// Ensure that the neutrino db path exists.
	if err := os.MkdirAll(config.DataDir, 0700); err != nil {
		fmt.Println("ERROR", err)
		os.Exit(1)
	}
	dbName := filepath.Join(config.DataDir, "neutrino.db")
	spvDatabase, err := walletdb.Create("bdb", dbName)
	if err != nil {
		fmt.Println("ERROR", err)
		os.Exit(1)
	}

	params := chaincfg.MainNetParams

	switch {
	case config.RegressionTest:
		params = chaincfg.RegressionNetParams
	case config.TestNet3:
		params = chaincfg.TestNet3Params
	case config.SimNet:
		params = chaincfg.SimNetParams
	}

	spvConfig := neutrino.Config{
		DataDir:      config.DataDir,
		Database:     spvDatabase,
		ChainParams:  params,
		AddPeers:     config.AddPeers,
		ConnectPeers: config.ConnectPeers,
	}

	spvNode, err := neutrino.NewChainService(spvConfig)
	if err != nil {
		fmt.Println("ERROR", err)
		os.Exit(1)
	}
	spvNode.Start()

	client := nclient.NewNeutrinoClient(&params, spvNode)

	fmt.Println("BEST BLOCK", func() string {
		hash, height, err := client.GetBestBlock()
		return fmt.Sprint(height, " hash ", hash.String(), " err ", err)
	}())

	// Wait until the node has fully synced up to the local
	// btcd node.
	for {
		if spvNode.IsCurrent() {
			fmt.Println("Node is now current")
			fmt.Println(spvNode.BestBlock())
		}
		time.Sleep(time.Millisecond * 10000)
		// fmt.Println(spvNode.NetTotals())
	}

	spvDatabase.Close()
	spvNode.Stop()
}
