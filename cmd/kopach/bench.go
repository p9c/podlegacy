package main

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"github.com/parallelcointeam/pod/fork"
	"time"
)

// Bench runs benchmarks on all algorithms for each hardfork level
func Bench() {
	fmt.Println("Benchmark requested")
	fmt.Println("Please turn off any high cpu processes for a more accurate benchmark")
	// time.Sleep(3 * time.Second)
	fmt.Println("Pre-HF1 benchmarks:")
	fork.IsTestnet = false
	for a := range fork.List[0].AlgoVers {
		fmt.Println("Benchmarking algo", fork.List[0].AlgoVers[a])
		speed := bench(0, fork.List[0].AlgoVers[a])
		fmt.Println(speed, "ns/hash")

	}
	fmt.Println("HF1 benchmarks:")
	fork.IsTestnet = true
	for a := range fork.List[1].AlgoVers {
		fmt.Println("Benchmarking algo", fork.List[1].AlgoVers[a])
		speed := bench(1, fork.List[1].AlgoVers[a])
		fmt.Println(speed, "ns/hash")
	}
}

func bench(hf int, algo string) int64 {
	startTime := time.Now()
	b := make([]byte, 80)
	rand.Read(b)
	// Zero out the nonce value
	for i := 76; i < 80; i++ {
		b[i] = 0
	}
	var height int32
	if hf == 1 {
		height = fork.List[1].ActivationHeight
	}
	max := uint32(1 << 8)
	var done bool
	var i uint32
	for i = uint32(0); i < max && !done; i++ {
		b, done = updateNonce(b)
		// fmt.Println(b, algo, height)
		fork.Hash(b, algo, height)
	}
	endTime := time.Now()
	return int64(endTime.Sub(startTime)) / int64(i)
}

func updateNonce(b []byte) (out []byte, done bool) {
	if len(b) < 80 {
		return
	}
	nonce := binary.LittleEndian.Uint32(b[76:80])
	nonce++
	if nonce == 1<<31 {
		done = true
		return
	}
	nonceBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(nonceBytes, nonce)
	for i := range b[76:80] {
		b[76+i] = nonceBytes[i]
	}
	// fmt.Println(nonce)
	return b, done
}
