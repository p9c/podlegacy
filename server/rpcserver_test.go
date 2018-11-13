// Copyright (c) 2013-2017 The btcsuite developers
// Copyright (c) 2015-2017 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package server

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/parallelcointeam/pod/chain"
)

func TestDiff(t *testing.T) {
	fmt.Println("testing")
	genbits := big.NewFloat(0.0).SetInt(chain.CompactToBig(0x1e0fffff))
	bestbits := big.NewFloat(0.0).SetInt(chain.CompactToBig(0x19598ccc))
	gendiff := big.NewFloat(float64(0.00024414))
	bestdiff := big.NewFloat(float64(47960941.88119169))

	fmt.Printf("gen bits %0.0f\n", genbits)
	fmt.Printf("gen diff %0.12f\n", gendiff)
	gendifff := big.NewFloat(0.0).Mul(bestbits, bestdiff)
	gendiffs := fmt.Sprintf("%.0f", gendifff)
	gendiffi, _ := big.NewInt(0).SetString(gendiffs, 10)
	fmt.Printf("%08x\n", chain.BigToCompact(gendiffi))

	fmt.Println()

	fmt.Printf("best bits %0.0f\n", bestbits)
	fmt.Printf("best diff %0.12f\n", bestdiff)
	bestdifff := big.NewFloat(0.0).Mul(bestbits, bestdiff)
	bestdiffs := fmt.Sprintf("%.0f", bestdifff)
	bestdiffi, _ := big.NewInt(0).SetString(bestdiffs, 10)
	fmt.Printf("%08x\n", chain.BigToCompact(bestdiffi))

	fmt.Printf("should be 1d00ffff")
}
