package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("Kopach CPU miner for Parallelcoin DUO")
	cfg, args, err := loadConfig()
	if err != nil {
		os.Exit(1)
	}
	fmt.Println(cfg, args)
}
