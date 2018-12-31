package main

import (
	"fmt"
	"github.com/parallelcointeam/pod/fork"
	"os"
)

func main() {
	fmt.Println("Kopach CPU miner for Parallelcoin DUO")
	cfg, args, err := LoadConfig()
	if err != nil {
		os.Exit(1)
	}
	_, _ = cfg, args
	if cfg.TestNet3 {
		fork.IsTestnet = true
	}
	cfg.Bench = true
	if cfg.Bench {
		Bench()
	}
}
