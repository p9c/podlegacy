// Copyright (c) 2013-2017 The btcsuite developers

package blockchain

import (
	"encoding/hex"
	"math/big"
	"time"

	"github.com/parallelcointeam/pod/chaincfg/chainhash"
	"github.com/parallelcointeam/pod/fork"
	"github.com/parallelcointeam/pod/wire"
)

var (
	scryptPowLimit = func() big.Int {
		mplb, _ := hex.DecodeString("000000039fcaa04ac30b6384471f337748ef5c87c7aeffce5e51770ce6283137,")
		return *big.NewInt(0).SetBytes(mplb) //AllOnes.Rsh(&AllOnes, 0)
	}()
	ScryptPowLimit     = scryptPowLimit
	ScryptPowLimitBits = BigToCompact(&scryptPowLimit)

	// bigOne is 1 represented as a big.Int.  It is defined here to avoid
	// the overhead of creating it multiple times.
	bigOne = big.NewInt(1)

	// oneLsh256 is 1 shifted left 256 bits.  It is defined here to avoid
	// the overhead of creating it multiple times.
	oneLsh256 = new(big.Int).Lsh(bigOne, 256)
)

// HashToBig converts a chainhash.Hash into a big.Int that can be used to
// perform math comparisons.
func HashToBig(hash *chainhash.Hash) *big.Int {
	// A Hash is in little-endian, but the big package wants the bytes in
	// big-endian, so reverse them.
	buf := *hash
	blen := len(buf)
	for i := 0; i < blen/2; i++ {
		buf[i], buf[blen-1-i] = buf[blen-1-i], buf[i]
	}

	return new(big.Int).SetBytes(buf[:])
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

// CalcWork calculates a work value from difficulty bits.  Bitcoin increases
// the difficulty for generating a block by decreasing the value which the
// generated hash must be less than.  This difficulty target is stored in each
// block header using a compact representation as described in the documentation
// for CompactToBig.  The main chain is selected by choosing the chain that has
// the most proof of work (highest difficulty).  Since a lower target difficulty
// value equates to higher actual difficulty, the work value which will be
// accumulated must be the inverse of the difficulty.  Also, in order to avoid
// potential division by zero and really small floating point numbers, the
// result adds 1 to the denominator and multiplies the numerator by 2^256.
func CalcWork(bits uint32) *big.Int {
	// log.Debugf("CalcWork %08x %064x", bits, CompactToBig(bits))

	// Return a work value of zero if the passed difficulty bits represent
	// a negative number. Note this should not happen in practice with valid
	// blocks, but an invalid block could trigger it.
	difficultyNum := CompactToBig(bits)
	if difficultyNum.Sign() <= 0 {
		// log.Debugf("negative diff")
		return big.NewInt(0)
	}

	// (1 << 256) / (difficultyNum + 1)
	denominator := new(big.Int).Add(difficultyNum, bigOne)
	// log.Debugf("denominator %064x", denominator)
	r := new(big.Int).Div(oneLsh256, denominator)
	// log.Debugf("result %064x", r)
	return r
}

// calcEasiestDifficulty calculates the easiest possible difficulty that a block
// can have given starting difficulty bits and a duration.  It is mainly used to
// verify that claimed proof of work by a block is sane as compared to a
// known good checkpoint.
func (b *BlockChain) calcEasiestDifficulty(bits uint32, duration time.Duration) uint32 {
	log.Debug("calcEasiestDifficulty")
	// Convert types used in the calculations below.
	durationVal := int64(duration / time.Second)
	adjustmentFactor := big.NewInt(b.chainParams.RetargetAdjustmentFactor)

	// The test network rules allow minimum difficulty blocks after more
	// than twice the desired amount of time needed to generate a block has
	// elapsed.
	// if b.chainParams.ReduceMinDifficulty {
	// 	reductionTime := int64(b.chainParams.MinDiffReductionTime /
	// 		time.Second)
	// 	if durationVal > reductionTime {
	// 		return b.chainParams.PowLimitBits
	// 	}
	// }

	// Since easier difficulty equates to higher numbers, the easiest
	// difficulty for a given duration is the largest value possible given
	// the number of retargets for the duration and starting difficulty
	// multiplied by the max adjustment factor.
	newTarget := CompactToBig(bits)
	for durationVal > 0 && newTarget.Cmp(b.chainParams.PowLimit) < 0 {
		newTarget.Mul(newTarget, adjustmentFactor)
		durationVal -= b.maxRetargetTimespan
	}

	// Limit new value to the proof of work limit.
	if newTarget.Cmp(b.chainParams.PowLimit) > 0 {
		// log.Debugf("Setting minimum difficulty %064x", b.chainParams.PowLimit)
		newTarget.Set(b.chainParams.PowLimit)
	}

	return BigToCompact(newTarget)
}

// findPrevTestNetDifficulty returns the difficulty of the previous block which
// did not have the special testnet minimum difficulty rule applied.
//
// This function MUST be called with the chain state lock held (for writes).
func (b *BlockChain) findPrevTestNetDifficulty(startNode *blockNode) uint32 {
	// Search backwards through the chain for the last block without
	// the special rule applied.
	iterNode := startNode
	for iterNode != nil && iterNode.height%b.blocksPerRetarget != 0 &&
		iterNode.bits == b.chainParams.PowLimitBits {

		iterNode = iterNode.parent
	}

	// Return the found difficulty or the minimum difficulty if no
	// appropriate block was found.
	lastBits := b.chainParams.PowLimitBits
	if iterNode != nil {
		lastBits = iterNode.bits
	}
	return lastBits
}

// calcNextRequiredDifficulty calculates the required difficulty for the block
// after the passed previous block node based on the difficulty retarget rules.
// This function differs from the exported CalcNextRequiredDifficulty in that
// the exported version uses the current best chain as the previous block node
// while this function accepts any block node.
func (b *BlockChain) calcNextRequiredDifficulty(lastNode *blockNode, newBlockTime time.Time, algo uint32) (uint32, error) {
	var newTargetBits uint32
	var powLimit *big.Int
	var powLimitBits uint32

	switch fork.GetCurrent(uint64(lastNode.height+1), b.chainParams.Name == "testnet") {
	case 0:
		powLimitBits = wire.Algos[wire.AlgoVers[algo]].MinBits
		powLimit = CompactToBig(powLimitBits)
		switch algo {
		case 2:
		default:
			powLimit = b.chainParams.PowLimit
			powLimitBits = b.chainParams.PowLimitBits
		}
		// Genesis block.
		if lastNode == nil {
			return powLimitBits, nil
		}
		prevNode := lastNode
		if prevNode.version != algo {
			prevNode = prevNode.GetPrevWithAlgo(algo)
		}
		firstNode := prevNode.GetPrevWithAlgo(algo) //.RelativeAncestor(1)
		for i := int64(1); firstNode != nil && i < b.chainParams.AveragingInterval; i++ {
			firstNode = firstNode.RelativeAncestor(1).GetPrevWithAlgo(algo)
		}
		if firstNode == nil {
			return powLimitBits, nil
		}
		actualTimespan := prevNode.timestamp - firstNode.timestamp
		adjustedTimespan := actualTimespan
		if actualTimespan < b.chainParams.MinActualTimespan {
			adjustedTimespan = b.chainParams.MinActualTimespan
		} else if actualTimespan > b.chainParams.MaxActualTimespan {
			adjustedTimespan = b.chainParams.MaxActualTimespan
		}
		oldTarget := CompactToBig(prevNode.bits)
		newTarget := new(big.Int).Mul(oldTarget, big.NewInt(adjustedTimespan))
		newTarget = newTarget.Div(newTarget, big.NewInt(b.chainParams.AveragingTargetTimespan))
		if newTarget.Cmp(powLimit) > 0 {
			newTarget.Set(powLimit)
		}
		newTargetBits = BigToCompact(newTarget)
		log.Debugf("Difficulty retarget at block height %d, old %08x new %08x", lastNode.height+1, prevNode.bits, newTargetBits)
		log.Tracef("Old %08x New %08x", prevNode.bits, oldTarget, newTargetBits, CompactToBig(newTargetBits))
		log.Tracef("Actual timespan %v, adjusted timespan %v, target timespan %v",
			actualTimespan,
			adjustedTimespan,
			b.chainParams.AveragingTargetTimespan)
	case 1: // Plan 9 from Crypto Space
		powLimitBits = wire.Algos[wire.AlgoVers[algo]].MinBits
		powLimit = CompactToBig(powLimitBits)

		last := lastNode
		if last.version != algo {
			last = last.GetPrevWithAlgo(algo)
			if last == nil {
				// This is the first block of the algo
				return powLimitBits, nil
				// return b.chainParams.GenesisBlock.Header.Bits, nil
			}
		}
		lastheight := last.height
		firstheight := lastheight - int32(b.chainParams.AveragingInterval)
		// Consensus rule, if an algorithm does not appear for a day its difficulty not adjusted from the previous, as the difficulty adjusts from the previous of the algorithm, but if there was only one in a day it gets a free one, and this is to also keep all 9 algorithms running and add reverberation perturbation in addition to the difficulty bit flip
		if last.GetPrevWithAlgo(algo).height < firstheight {
			return last.bits, nil
		}
		bits := last.bits
		if firstheight < 1 {
			firstheight = 1
			if lastheight == firstheight {
				// log.Debugf("second block of algo")
				return bits, nil
				// return b.chainParams.GenesisBlock.Header.Bits, nil
			}
			if firstheight == 0 {
				return bits, nil
			}
		}
		first, _ := b.BlockByHeight(firstheight)
		if first == nil {
			return bits, nil
		}
		lasttime := last.timestamp
		firsttime := first.MsgBlock().Header.Timestamp.Unix()
		numblocks := lastheight - firstheight
		interval := float64(lasttime - firsttime)
		avblocktime := interval / float64(numblocks)
		// divergence := float64(b.chainParams.TargetTimePerBlock) / float64(avblocktime)
		divergence := avblocktime / float64(b.chainParams.TargetTimePerBlock)

		// Now we have the divergence in the averaging period, we now use this formula: https://github.com/parallelcointeam/pod/raw/master/docs/parabolic-diff-adjustment-filter-formula.png
		// This is the expanded version as will be required:
		// adjustment = 1 + (20 * divergence - 20) * (20 * divergence - 20) / 200 * divergence
		// adjustment := divergence
		adjustment := 1 + (divergence-1)*(divergence-1)*(divergence-1)
		if adjustment < 0.0 {
			adjustment *= -1
		}
		log.Debugf("algo %s divergence %.7f adjustment %.7f numblocks %d interval %.0f avblocktime %.7f", wire.AlgoVers[algo], divergence, adjustment, numblocks, interval, avblocktime)
		bigadjustment := big.NewFloat(adjustment)
		bigoldtarget := big.NewFloat(0.0).SetInt(CompactToBig(last.bits))
		bigfnewtarget := big.NewFloat(0.0).Mul(bigadjustment, bigoldtarget)
		newtarget, _ := bigfnewtarget.Int(nil)
		// log.Debugf("firsttime %d lasttime %d interval %.0f avblocktime %.8f", firsttime, lasttime, interval, avblocktime)
		// log.Debugf("target %d divergence %.8f", b.chainParams.TargetTimePerBlock, divergence)
		// log.Debugf("newtarget %064x accuracy %d", newtarget, accuracy)
		if newtarget.Cmp(powLimit) > 0 {
			newTargetBits = powLimitBits
		} else {
			newTargetBits = BigToCompact(newtarget)
		}
		// basetargetbits := newTargetBits
		newTargetBits ^= 0x00000003
		// log.Warnf("height %d av %d blocks: %.8f target: %d, divergence: %.4f adjustment: %.4f dither: %08x -> %08x", lastheight, numblocks, avblocktime, b.chainParams.TargetTimePerBlock, divergence, adjustment, basetargetbits, newTargetBits)
		// log.Warnf("old: %064x", CompactToBig(last.bits))
		// log.Warnf("new: %064x", CompactToBig(BigToCompact(newtarget)))
		log.Debugf("Difficulty retarget at block height %d, old %08x new %08x", lastNode.height+1, last.bits, newTargetBits)
	}
	return newTargetBits, nil
}

// CalcNextRequiredDifficulty calculates the required difficulty for the block
// after the end of the current best chain based on the difficulty retarget
// rules.
//
// This function is safe for concurrent access.
func (b *BlockChain) CalcNextRequiredDifficulty(timestamp time.Time, algo uint32) (uint32, error) {
	// log.Debugf("CalcNextRequiredDifficulty")

	b.chainLock.Lock()
	difficulty, err := b.calcNextRequiredDifficulty(b.bestChain.Tip(), timestamp, algo)
	b.chainLock.Unlock()
	return difficulty, err
}
