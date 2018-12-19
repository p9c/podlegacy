// Package fork handles tracking the hard fork status and is used to determine which consensus rules apply on a block
package fork

import (
	"encoding/hex"
	"math/big"
)

// HardForks is the details related to a hard fork, number, name and activation height
type HardForks struct {
	Number           uint32
	Name             string
	ActivationHeight int32
	Algos            map[string]AlgoParams
	AlgoVers         map[int32]string
}

// AlgoParams are the identifying block version number and their minimum target bits
type AlgoParams struct {
	Version int32
	MinBits uint32
	AlgoID  uint32
}

var (
	// IsTestnet is set at startup here to be accessible to all other libraries
	IsTestnet bool
	// List is the list of existing hard forks and when they activate
	List = []HardForks{
		HardForks{
			Number:           0,
			Name:             "Halcyon days",
			ActivationHeight: 0, // Approximately 18 Jan 2019
			Algos:            Algos,
			AlgoVers:         AlgoVers,
		},
		HardForks{
			Number:           1,
			Name:             "Plan 9 from Crypto Space",
			ActivationHeight: 185463,
			Algos:            P9Algos,
			AlgoVers:         P9AlgoVers,
		},
	}
	// mainPowLimit is the highest proof of work value a Parallelcoin block can
	// have for the main network.  It is the maximum target / 2^160
	mainPowLimit = func() big.Int {
		mplb, _ := hex.DecodeString("00000fffff000000000000000000000000000000000000000000000000000000")
		return *big.NewInt(0).SetBytes(mplb) //AllOnes.Rsh(&AllOnes, 0)
	}()
	mainPowLimitBits = BigToCompact(&mainPowLimit)

	// Algos are the specifications identifying the algorithm used in the block proof
	Algos = map[string]AlgoParams{
		"sha256d": AlgoParams{2, mainPowLimitBits, 0},
		"scrypt":  AlgoParams{514, mainPowLimitBits, 1},
	}

	// P9Algos is the algorithm specifications after the hard fork
	P9Algos = map[string]AlgoParams{
		"sha256d":   AlgoParams{1, mainPowLimitBits, 0},
		"scrypt":    AlgoParams{2, mainPowLimitBits, 1},
		"blake14lr": AlgoParams{3, 0x1d1089f6, 2},
		"gost":      AlgoParams{4, 0x1e00b629, 7},
		"keccak":    AlgoParams{5, 0x1e050502, 8},
		"lyra2rev2": AlgoParams{6, 0x1d5c89d1, 4},
		"skein":     AlgoParams{7, 0x1d332839, 5},
		"whirlpool": AlgoParams{8, 0x1d1c0eea, 3},
		"x11":       AlgoParams{9, 0x1d5c89d1, 6},
	}

	// AlgoVers is the lookup for pre hardfork
	AlgoVers = map[int32]string{
		2:   "sha256d",
		514: "scrypt",
	}

	// P9AlgoVers is the lookup for after 1st hardfork
	P9AlgoVers = map[int32]string{
		2:   "sha256d",
		514: "scrypt",
		3:   "blake14lr",
		4:   "gost",
		5:   "keccak",
		6:   "lyra2rev2",
		7:   "skein",
		8:   "whirlpool",
		9:   "x11",
	}
)

// GetAlgoVer returns the version number for a given algorithm (by string name) at a given height
func GetAlgoVer(name string, height int32) (version int32) {
	if IsTestnet {
		return P9Algos[name].Version
	}
	for i := range List {
		if height > List[i].ActivationHeight {
			version = List[i].Algos[name].Version
		}
	}
	return
}

// GetAlgoName returns the string identifier of an algorithm depending on hard fork activation status
func GetAlgoName(algoVer int32, height int32) (name string) {
	if IsTestnet {
		name = P9AlgoVers[algoVer]
	}
	for i := range List {
		if height > List[i].ActivationHeight {
			name = List[i].AlgoVers[algoVer]
		}
	}
	return
}

// CompactToBig converts a compact representation of a whole number N to an
// unsigned 32-bit number.  The representation is similar to IEEE754 floating
// point numbers.
//
// Like IEEE754 floating point, there are three basic components: the sign,
// the exponent, and the mantissa.  They are broken out as follows:
//
//	* the most significant 8 bits represent the unsigned base 256 exponent
// 	* bit 23 (the 24th bit) represents the sign bit
//	* the least significant 23 bits represent the mantissa
//
//	-------------------------------------------------
//	|   Exponent     |    Sign    |    Mantissa     |
//	-------------------------------------------------
//	| 8 bits [31-24] | 1 bit [23] | 23 bits [22-00] |
//	-------------------------------------------------
//
// The formula to calculate N is:
// 	N = (-1^sign) * mantissa * 256^(exponent-3)
//
// This compact form is only used in bitcoin to encode unsigned 256-bit numbers
// which represent difficulty targets, thus there really is not a need for a
// sign bit, but it is implemented here to stay consistent with bitcoind.
func CompactToBig(compact uint32) *big.Int {
	// Extract the mantissa, sign bit, and exponent.
	mantissa := compact & 0x007fffff
	isNegative := compact&0x00800000 != 0
	exponent := uint(compact >> 24)

	// Since the base for the exponent is 256, the exponent can be treated
	// as the number of bytes to represent the full 256-bit number.  So,
	// treat the exponent as the number of bytes and shift the mantissa
	// right or left accordingly.  This is equivalent to:
	// N = mantissa * 256^(exponent-3)
	var bn *big.Int
	if exponent <= 3 {
		mantissa >>= 8 * (3 - exponent)
		bn = big.NewInt(int64(mantissa))
	} else {
		bn = big.NewInt(int64(mantissa))
		bn.Lsh(bn, 8*(exponent-3))
	}

	// Make it negative if the sign bit is set.
	if isNegative {
		bn = bn.Neg(bn)
	}

	return bn
}

// BigToCompact converts a whole number N to a compact representation using
// an unsigned 32-bit number.  The compact representation only provides 23 bits
// of precision, so values larger than (2^23 - 1) only encode the most
// significant digits of the number.  See CompactToBig for details.
func BigToCompact(n *big.Int) uint32 {
	// No need to do any work if it's zero.
	if n.Sign() == 0 {
		return 0
	}

	// Since the base for the exponent is 256, the exponent can be treated
	// as the number of bytes.  So, shift the number right or left
	// accordingly.  This is equivalent to:
	// mantissa = mantissa / 256^(exponent-3)
	var mantissa uint32
	exponent := uint(len(n.Bytes()))
	if exponent <= 3 {
		mantissa = uint32(n.Bits()[0])
		mantissa <<= 8 * (3 - exponent)
	} else {
		// Use a copy to avoid modifying the caller's original number.
		tn := new(big.Int).Set(n)
		mantissa = uint32(tn.Rsh(tn, 8*(exponent-3)).Bits()[0])
	}

	// When the mantissa already has the sign bit set, the number is too
	// large to fit into the available 23-bits, so divide the number by 256
	// and increment the exponent accordingly.
	if mantissa&0x00800000 != 0 {
		mantissa >>= 8
		exponent++
	}

	// Pack the exponent, sign bit, and mantissa into an unsigned 32-bit
	// int and return it.
	compact := uint32(exponent<<24) | mantissa
	if n.Sign() < 0 {
		compact |= 0x00800000
	}
	return compact
}
