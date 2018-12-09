package wire

import (
	"bytes"
	"fmt"
	"io"
	"time"

	"github.com/aguycalled/gox13hash"
	x11 "github.com/bitbandi/go-x11"
	"github.com/bitgoin/lyra2rev2"
	"github.com/dchest/blake256"
	"github.com/ebfe/keccak"
	"github.com/farces/skein512/skein"
	"github.com/parallelcointeam/pod/chaincfg/chainhash"
	"golang.org/x/crypto/blake2b"
	"golang.org/x/crypto/scrypt"
)

// MaxBlockHeaderPayload is the maximum number of bytes a block header can be.
// Version 4 bytes + Timestamp 4 bytes + Bits 4 bytes + Nonce 4 bytes +
// PrevBlock and MerkleRoot hashes.
const MaxBlockHeaderPayload = 16 + (chainhash.HashSize * 2)

// BlockHeader defines information about a block and is used in the bitcoin
// block (MsgBlock) and headers (MsgHeaders) messages.
type BlockHeader struct {
	// Version of the block.  This is not the same as the protocol version.
	Version uint32

	// Hash of the previous block header in the block chain.
	PrevBlock chainhash.Hash

	// Merkle tree reference to hash of all transactions for the block.
	MerkleRoot chainhash.Hash

	// Time the block was created.  This is, unfortunately, encoded as a
	// uint32 on the wire and therefore is limited to 2106.
	Timestamp time.Time

	// Difficulty target for the block.
	Bits uint32

	// Nonce used to generate the block.
	Nonce uint32
}

// blockHeaderLen is a constant that represents the number of bytes for a block
// header.
const blockHeaderLen = 80

// BlockHash computes the block identifier hash for the given block header.
func (h *BlockHeader) BlockHash() (out chainhash.Hash) {
	// Encode the header and double sha256 everything prior to the number of
	// transactions.  Ignore the error returns since there is no way the
	// encode could fail except being out of memory which would cause a
	// run-time panic.
	buf := bytes.NewBuffer(make([]byte, 0, MaxBlockHeaderPayload))
	_ = writeBlockHeader(buf, 0, h)
	// switch h.Version {
	// case 2:
	out = chainhash.DoubleHashH(buf.Bytes())
	// case 514:
	// 	b := buf.Bytes()
	// 	c := make([]byte, len(b))
	// 	for i := range b {
	// 		c[i] = b[len(b)-1-i]
	// 	}
	// 	dk, err := scrypt.Key(c, c, 1024, 1, 1, 32)
	// 	if err != nil {
	// 		fmt.Println(fmt.Errorf("Unable to generate scrypt key: %s", err))
	// 		return
	// 	}

	// 	for i := range dk {
	// 		out[i] = dk[len(dk)-1-i]
	// 	}
	// }
	return
}

// BlockHashWithAlgos computes the block identifier hash for the given block header. This function is additional because the sync manager and the parallelcoin protocol only use SHA256D hashes for inventories and calculating the scrypt (or other) hash for these blocks when requested via that route causes an 'unrequested block' error.
func (h *BlockHeader) BlockHashWithAlgos(hf bool) (out chainhash.Hash) {
	// Encode the header and double sha256 everything prior to the number of
	// transactions.  Ignore the error returns since there is no way the
	// encode could fail except being out of memory which would cause a
	// run-time panic.
	// fmt.Printf("algo %d\n", h.Version)
	buf := bytes.NewBuffer(make([]byte, 0, MaxBlockHeaderPayload))
	_ = writeBlockHeader(buf, 0, h)
	switch h.Version {
	case 3: // Blake14lr (decred)
		// fmt.Printf("hashing with Blake14lr\n")
		a := blake256.New()
		a.Write(buf.Bytes())
		out.SetBytes(a.Sum(nil))
	case 6: // Blake2b (sia)
		// fmt.Printf("hashing with Blake2b\n")
		out = blake2b.Sum256(buf.Bytes())
	case 10: // Lyra2REv2 (verge)
		// fmt.Printf("hashing with Lyra2REv2\n")
		o, _ := lyra2rev2.Sum(buf.Bytes())
		out.SetBytes(o)
	case 18: // Skein (skein512 + SHA256 as myriad)
		// fmt.Printf("hashing with Skein\n")
		o := buf.Bytes()
		hasher := skein.NewSkein512()
		o2 := hasher.Hash(o)
		o3 := make([]byte, 64)
		for i := range o2 {
			o3[i] = byte(o2[i])
		}
		out = chainhash.HashH(o3)
	case 34: // X11
		// fmt.Printf("hashing with X11\n")
		o := [32]byte{}
		x := x11.New()
		x.Hash(buf.Bytes(), o[:])
		out.SetBytes(o[:])
	case 66: // X13
		// fmt.Printf("hashing with X13\n")
		out = gox13hash.Sum(buf.Bytes())
	case 130: // keccac/SHA3
		// fmt.Printf("hashing with keccac\n")
		k := keccak.New256()
		k.Reset()
		k.Write(buf.Bytes())
		out.SetBytes(k.Sum(nil))
	case 514: // scrypt
		// fmt.Printf("hashing with scrypt\n")
		// b := chainhash.DoubleHashH(buf.Bytes())
		b := buf.Bytes()
		c := make([]byte, len(b))
		// for i := range b {
		// 	c[i] = b[len(b)-1-i]
		// }
		copy(c, b[:])
		dk, err := scrypt.Key(c, c, 1024, 1, 1, 32)
		if err != nil {
			fmt.Println(fmt.Errorf("Unable to generate scrypt key: %s", err))
			return
		}
		for i := range dk {
			out[i] = dk[len(dk)-1-i]
		}
		copy(out[:], dk)
	case 2: // sha256d
		// fmt.Printf("hashing with sha256d\n")
		out.SetBytes(chainhash.DoubleHashB(buf.Bytes()))
		// fmt.Printf("%064x\n", out)
	default:
		out.SetBytes(chainhash.DoubleHashB(buf.Bytes()))
	}
	return
}

// BtcDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
// See Deserialize for decoding block headers stored to disk, such as in a
// database, as opposed to decoding block headers from the wire.
func (h *BlockHeader) BtcDecode(r io.Reader, pver uint32, enc MessageEncoding) error {
	return readBlockHeader(r, pver, h)
}

// BtcEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
// See Serialize for encoding block headers to be stored to disk, such as in a
// database, as opposed to encoding block headers for the wire.
func (h *BlockHeader) BtcEncode(w io.Writer, pver uint32, enc MessageEncoding) error {
	return writeBlockHeader(w, pver, h)
}

// Deserialize decodes a block header from r into the receiver using a format
// that is suitable for long-term storage such as a database while respecting
// the Version field.
func (h *BlockHeader) Deserialize(r io.Reader) error {
	// At the current time, there is no difference between the wire encoding
	// at protocol version 0 and the stable long-term storage format.  As
	// a result, make use of readBlockHeader.
	return readBlockHeader(r, 0, h)
}

// Serialize encodes a block header from r into the receiver using a format
// that is suitable for long-term storage such as a database while respecting
// the Version field.
func (h *BlockHeader) Serialize(w io.Writer) error {
	// At the current time, there is no difference between the wire encoding
	// at protocol version 0 and the stable long-term storage format.  As
	// a result, make use of writeBlockHeader.
	return writeBlockHeader(w, 0, h)
}

// NewBlockHeader returns a new BlockHeader using the provided version, previous
// block hash, merkle root hash, difficulty bits, and nonce used to generate the
// block with defaults for the remaining fields.
func NewBlockHeader(version uint32, prevHash, merkleRootHash *chainhash.Hash,
	bits uint32, nonce uint32) *BlockHeader {

	// Limit the timestamp to one second precision since the protocol
	// doesn't support better.
	return &BlockHeader{
		Version:    version,
		PrevBlock:  *prevHash,
		MerkleRoot: *merkleRootHash,
		Timestamp:  time.Unix(time.Now().Unix(), 0),
		Bits:       bits,
		Nonce:      nonce,
	}
}

// readBlockHeader reads a bitcoin block header from r.  See Deserialize for
// decoding block headers stored to disk, such as in a database, as opposed to
// decoding from the wire.
func readBlockHeader(r io.Reader, pver uint32, bh *BlockHeader) error {
	return readElements(r, &bh.Version, &bh.PrevBlock, &bh.MerkleRoot,
		(*uint32Time)(&bh.Timestamp), &bh.Bits, &bh.Nonce)
}

// writeBlockHeader writes a bitcoin block header to w.  See Serialize for
// encoding block headers to be stored to disk, such as in a database, as
// opposed to encoding for the wire.
func writeBlockHeader(w io.Writer, pver uint32, bh *BlockHeader) error {
	sec := uint32(bh.Timestamp.Unix())
	return writeElements(w, bh.Version, &bh.PrevBlock, &bh.MerkleRoot,
		sec, bh.Bits, bh.Nonce)
}
