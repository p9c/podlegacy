// Copyright (c) 2013-2017 The btcsuite developers

package blockchain

import (
	"math"
	"math/big"
	"time"

	"github.com/parallelcointeam/pod/chaincfg/chainhash"
)

var (
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
	// Return a work value of zero if the passed difficulty bits represent
	// a negative number. Note this should not happen in practice with valid
	// blocks, but an invalid block could trigger it.
	difficultyNum := CompactToBig(bits)
	if difficultyNum.Sign() <= 0 {
		return big.NewInt(0)
	}

	// (1 << 256) / (difficultyNum + 1)
	denominator := new(big.Int).Add(difficultyNum, bigOne)
	return new(big.Int).Div(oneLsh256, denominator)
}

// calcEasiestDifficulty calculates the easiest possible difficulty that a block
// can have given starting difficulty bits and a duration.  It is mainly used to
// verify that claimed proof of work by a block is sane as compared to a
// known good checkpoint.
func (b *BlockChain) calcEasiestDifficulty(bits uint32, duration time.Duration) uint32 {
	// Convert types used in the calculations below.
	durationVal := int64(duration / time.Second)
	adjustmentFactor := big.NewInt(b.chainParams.RetargetAdjustmentFactor)

	// The test network rules allow minimum difficulty blocks after more
	// than twice the desired amount of time needed to generate a block has
	// elapsed.
	if b.chainParams.ReduceMinDifficulty {
		reductionTime := int64(b.chainParams.MinDiffReductionTime /
			time.Second)
		if durationVal > reductionTime {
			return b.chainParams.PowLimitBits
		}
	}

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
	var powLimit *big.Int
	var powLimitBits uint32
	switch algo {
	case 514:
		powLimit = b.chainParams.ScryptPowLimit
		powLimitBits = b.chainParams.ScryptPowLimitBits
	default:
		powLimit = b.chainParams.PowLimit
		powLimitBits = b.chainParams.PowLimitBits
	}
	if lastNode.height < 2 && lastNode.height > 0 {
		// time.Sleep(time.Second * time.Duration(b.chainParams.TargetTimePerBlock))
		return b.chainParams.PowLimitBits, nil
	}
	// Genesis block.
	if lastNode == nil {
		return powLimitBits, nil
	}

	switch b.chainParams.Name {
	case "mainnet":
		prevNode := lastNode
		if prevNode.version != algo {
			prevNode = prevNode.GetPrevWithAlgo(algo)
		}

		// Return the previous block's difficulty requirements if this block is not at a difficulty retarget interval. For networks that support it, allow special reduction of the required difficulty once too much time has elapsed without mining a block.
		if b.chainParams.ReduceMinDifficulty {
			// Return minimum difficulty when more than the desired amount of time has elapsed without mining a block.
			reductionTime := int64(b.chainParams.MinDiffReductionTime)
			allowMinTime := lastNode.timestamp + reductionTime
			if newBlockTime.Unix() > allowMinTime {
				return b.chainParams.PowLimitBits, nil
			}

			// The block was mined within the desired timeframe, so return the difficulty for the last block which did not have the special minimum difficulty rule applied.
			return b.findPrevTestNetDifficulty(lastNode), nil
		}

		// Get the block node at the previous retarget (targetTimespan days worth of blocks).
		firstNode := prevNode.GetPrevWithAlgo(algo)
		for i := int64(1); firstNode != nil && i < b.chainParams.AveragingInterval; i++ {
			firstNode = firstNode.RelativeAncestor(1).GetPrevWithAlgo(algo)
		}
		if firstNode == nil {
			return powLimitBits, nil
		}

		// Limit the amount of adjustment that can occur to the previous difficulty.
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

		// Limit new value to the proof of work limit.
		if newTarget.Cmp(powLimit) > 0 {
			newTarget.Set(powLimit)
		}

		// Log new target difficulty and return it.  The new target logging is intentionally converting the bits back to a number instead of using newTarget since conversion to the compact representation loses precision.
		newTargetBits := BigToCompact(newTarget)
		log.Debugf("Difficulty retarget at block height %d, old %08x new %08x",
			lastNode.height+1, prevNode.bits, newTargetBits)
		log.Tracef("Old %08x New %08x",
			prevNode.bits, oldTarget, newTargetBits, CompactToBig(newTargetBits))
		log.Tracef("Actual timespan %v, adjusted timespan %v, target timespan %v",
			actualTimespan, adjustedTimespan, b.chainParams.AveragingTargetTimespan)
		return newTargetBits, nil
	case "testnet":
		firstblock, _ := b.BlockByHeight(0)
		newestblock := lastNode.GetPrevWithAlgo(algo)
		var prevbits uint32
		if newestblock == nil {
			switch firstblock.MsgBlock().Header.Version {
			case 514:
				prevbits = powLimitBits
			default:
				prevbits = firstblock.MsgBlock().Header.Bits
			}
			return prevbits, nil
		}
		height := int64(lastNode.height)
		if height <= b.chainParams.Interval {
			// if we have less than the interval number of blocks, we work from block 1 time, assuming a new network starting up
			firstblock, _ = b.BlockByHeight(1)
		} else {
			// if there is enough blocks to get the prescribed interval number of blocks, we use the block at interval blocks past as our window
			firstblock, _ = b.BlockByHeight(int32(height - b.chainParams.Interval))
		}
		prevbits = newestblock.bits
		prevdiff := CompactToBig(prevbits)
		alltime := newBlockTime.Unix() - firstblock.MsgBlock().Header.Timestamp.Unix()
		prec := int64(65536)
		// the window is the number of blocks we are calculating against, to generate the average block time
		window := height - int64(firstblock.Height()) + 1
		// we now have the past ratio of divergence from the target. We are using `prec` precision to improve the accuracy of the calculation, this 2^16, 16 bits of precision
		avperblock := alltime * prec / window
		shouldtime := b.chainParams.TargetTimePerBlock * prec

		var newdiff *big.Int
		// log.Debugf("average per block %d should be %d", avperblock/prec, shouldtime/prec)
		fav := float64(avperblock)
		fav /= float64(prec)
		fsh := float64(shouldtime)
		fsh /= float64(prec)
		ratio := fav / fsh

		var smoothing float64 = 10
		var factor float64 = 2

		exponent := 1 + (smoothing*factor*ratio-smoothing*factor)*(smoothing*factor)/10*smoothing*factor*ratio

		newdiff = big.NewInt(0).Mul(prevdiff, big.NewInt(int64(math.Pow(float64(avperblock), exponent))))
		newdiff = big.NewInt(0).Div(newdiff, big.NewInt(int64(math.Pow(float64(shouldtime), exponent))))

		// We now have the ratio of divergence as a 64 bit floating point number, which will be used with the adjustment filter
		//

		// switch {
		// case ratio < 0.5:
		// newdiff = big.NewInt(0).Mul(prevdiff, big.NewInt(avperblock*avperblock*avperblock*avperblock))
		// newdiff = big.NewInt(0).Div(newdiff, big.NewInt(shouldtime*shouldtime*shouldtime*shouldtime))
		// log.Debugf("cold and low %03.2f diff: %064x", ratio, newdiff)
		// case ratio < 0.75:
		// newdiff = big.NewInt(0).Mul(prevdiff, big.NewInt(avperblock*avperblock*avperblock))
		// newdiff = big.NewInt(0).Div(newdiff, big.NewInt(shouldtime*shouldtime*shouldtime))
		// log.Debugf("warm and low %03.2f diff: %064x", ratio, newdiff)
		// case ratio < 0.8:
		// newdiff = big.NewInt(0).Mul(prevdiff, big.NewInt(avperblock*avperblock))
		// newdiff = big.NewInt(0).Div(newdiff, big.NewInt(shouldtime*shouldtime))
		// log.Debugf("hot and low %03.2f diff: %064x", ratio, newdiff)
		// case ratio < 0.9:
		// newdiff = big.NewInt(0).Mul(prevdiff, big.NewInt(avperblock*avperblock))
		// newdiff = big.NewInt(0).Div(newdiff, big.NewInt(shouldtime*shouldtime))
		// log.Debugf("boiling and low %03.2f diff: %064x", ratio, newdiff)
		// case ratio > 1.11:
		// newdiff = big.NewInt(0).Mul(prevdiff, big.NewInt(avperblock))
		// newdiff = big.NewInt(0).Div(newdiff, big.NewInt(shouldtime))
		// log.Debugf("boiling and high %03.2f diff: %064x", ratio, newdiff)
		// case ratio > 1.25:
		// newdiff = big.NewInt(0).Mul(prevdiff, big.NewInt(avperblock*avperblock))
		// newdiff = big.NewInt(0).Div(newdiff, big.NewInt(shouldtime*shouldtime))
		// log.Debugf("hot and high %03.2f diff: %064x", ratio, newdiff)
		// case ratio > 1.5:
		// newdiff = big.NewInt(0).Mul(prevdiff, big.NewInt(avperblock*avperblock*avperblock))
		// newdiff = big.NewInt(0).Div(newdiff, big.NewInt(shouldtime*shouldtime*shouldtime))
		// log.Debugf("warm and high %03.2f diff: %064x", ratio, newdiff)
		// case ratio > 2:
		// newdiff = big.NewInt(0).Mul(prevdiff, big.NewInt(avperblock*avperblock*avperblock*avperblock))
		// newdiff = big.NewInt(0).Div(newdiff, big.NewInt(shouldtime*shouldtime*shouldtime*shouldtime))
		// log.Debugf("cold and high %03.2f diff: %064x", ratio, newdiff)
		// default:
		// newdiff = prevdiff
		// log.Debugf("DINGDINGDING! %03.2f diff: %064x", ratio, newdiff)
		// }
		// if newdiff.Cmp(b.chainParams.PowLimit) > 0 {
		// newdiff = b.chainParams.PowLimit
		// }
		log.Debugf("prev: %08x %064x", prevbits, prevdiff)
		log.Debugf("next: %08x %064x", BigToCompact(newdiff), newdiff)

		return BigToCompact(newdiff), nil
	}
	return powLimitBits, nil
}

// CalcNextRequiredDifficulty calculates the required difficulty for the block after the end of the current best chain based on the difficulty retarget rules. This function is safe for concurrent access.
func (b *BlockChain) CalcNextRequiredDifficulty(timestamp time.Time, algo uint32) (uint32, error) {
	b.chainLock.Lock()
	difficulty, err := b.calcNextRequiredDifficulty(b.bestChain.Tip(), timestamp, algo)
	b.chainLock.Unlock()
	return difficulty, err
}
