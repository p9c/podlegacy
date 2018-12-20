package wire
import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"time"
	"github.com/aead/skein/skein256"
	whirl "github.com/balacode/zr-whirl"
	x11 "github.com/bitbandi/go-x11"
	"github.com/bitgoin/lyra2rev2"
	"github.com/dchest/blake256"
	"github.com/ebfe/keccak"
	// "github.com/enceve/crypto/skein/skein256"
	"github.com/jzelinskie/whirlpool"
	"github.com/parallelcointeam/pod/chaincfg/chainhash"
	"github.com/parallelcointeam/pod/fork"
	gost "github.com/programmer10110/gostreebog"
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
	Version int32
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
	// Encode the header and double sha256 everything prior to the number of transactions.  Ignore the error returns since there is no way the encode could fail except being out of memory which would cause a run-time panic.
	buf := bytes.NewBuffer(make([]byte, 0, MaxBlockHeaderPayload))
	_ = writeBlockHeader(buf, 0, h)
	out = chainhash.DoubleHashH(buf.Bytes())
	return
}
// Hash computes the hash of bytes using the named hash
func Hash(bytes []byte, name string) (out chainhash.Hash) {
	switch name {
	case "blake14lr":
		a := blake256.New()
		a.Write(bytes)
		out.SetBytes(a.Sum(nil))
	case "whirlpool":
		wp := whirlpool.New()
		io.WriteString(wp, hex.EncodeToString(bytes))
		o := whirl.Sum512(bytes)
		out.SetBytes(o[:32])
	case "lyra2rev2":
		o, _ := lyra2rev2.Sum(bytes)
		out.SetBytes(o)
	case "skein":
		in := bytes
		var o [32]byte
		skein256.Sum256(&o, in, nil)
		out.SetBytes(o[:])
	case "x11":
		o := [32]byte{}
		x := x11.New()
		x.Hash(bytes, o[:])
		out.SetBytes(o[:])
	case "gost":
		o := gost.Hash(bytes, "256")
		out.SetBytes(o)
	case "keccak":
		k := keccak.New256()
		k.Reset()
		k.Write(bytes)
		out.SetBytes(k.Sum(nil))
	case "scrypt":
		b := bytes
		c := make([]byte, len(b))
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
	case "sha256d": // sha256d
		out.SetBytes(chainhash.DoubleHashB(bytes))
	}
	return
}
// BlockHashWithAlgos computes the block identifier hash for the given block header. This function is additional because the sync manager and the parallelcoin protocol only use SHA256D hashes for inventories and calculating the scrypt (or other) hash for these blocks when requested via that route causes an 'unrequested block' error.
func (h *BlockHeader) BlockHashWithAlgos(height int32) (out chainhash.Hash) {
	// Encode the header and double sha256 everything prior to the number of transactions.  Ignore the error returns since there is no way the encode could fail except being out of memory which would cause a run-time panic.
	buf := bytes.NewBuffer(make([]byte, 0, MaxBlockHeaderPayload))
	_ = writeBlockHeader(buf, 0, h)
	vers := h.Version
	out = Hash(buf.Bytes(), fork.GetAlgoName(vers, height))
	return
}
// BtcDecode decodes r using the bitcoin protocol encoding into the receiver. This is part of the Message interface implementation. See Deserialize for decoding block headers stored to disk, such as in a database, as opposed to decoding block headers from the wire.
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
func NewBlockHeader(version int32, prevHash, merkleRootHash *chainhash.Hash,
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
