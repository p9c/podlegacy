package main

import (
	"crypto/rand"
	"encoding/binary"
	"math"

	"github.com/parallelcointeam/pod/wire"
)

// RandomUint16Number returns a random uint16 in a specified input range.  Note
// that the range is in zeroth ordering; if you pass it 1800, you will get
// values from 0 to 1800.
func RandomUint16Number(max uint16) uint16 {
	// In order to avoid modulo bias and ensure every possible outcome in
	// [0, max) has equal probability, the random number must be sampled
	// from a random source that has a range limited to a multiple of the
	// modulus.
	var randomNumber uint16
	var limitRange = (math.MaxUint16 / max) * max
	for {
		binary.Read(rand.Reader, binary.LittleEndian, &randomNumber)
		if randomNumber < limitRange {
			return (randomNumber % max)
		}
	}
}

// HasServices returns whether or not the provided advertised service flags have
// all of the provided desired service flags set.
func HasServices(advertised, desired wire.ServiceFlag) bool {
	return advertised&desired == desired
}
