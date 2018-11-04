package main

import (
	"container/list"
	"sync"

	"github.com/parallelcointeam/btcutil"
	"github.com/parallelcointeam/pod/blockchain"
	"github.com/parallelcointeam/pod/chaincfg"
	"github.com/parallelcointeam/pod/chaincfg/chainhash"
	"github.com/parallelcointeam/pod/mempool"
	"github.com/parallelcointeam/pod/netsync"
	"github.com/parallelcointeam/pod/peer"
	peerpkg "github.com/parallelcointeam/pod/peer"
	"github.com/parallelcointeam/pod/wire"
)

// PeerNotifier exposes methods to notify peers of status changes to
// transactions, blocks, etc. Currently server (in the main package) implements
// this interface.
type PeerNotifier interface {
	AnnounceNewTransactions(newTxs []*mempool.TxDesc)

	UpdatePeerHeights(latestBlkHash *chainhash.Hash, latestHeight int32, updateSource *peer.Peer)

	RelayInventory(invVect *wire.InvVect, data interface{})

	TransactionConfirmed(tx *btcutil.Tx)
}

// Config is a configuration struct used to initialize a new SyncManager.
type Config struct {
	PeerNotifier PeerNotifier
	Chain        *blockchain.BlockChain
	TxMemPool    *mempool.TxPool
	ChainParams  *chaincfg.Params

	DisableCheckpoints bool
	MaxPeers           int

	FeeEstimator *mempool.FeeEstimator
}

// SyncManager is used to communicate block related messages with peers. The
// SyncManager is started as by executing Start() in a goroutine. Once started,
// it selects peers to sync from and starts the initial block download. Once the
// chain is in sync, the SyncManager handles incoming block and header
// notifications and relays announcements of new blocks to peers.
type SyncManager struct {
	peerNotifier   netsync.PeerNotifier
	started        int32
	shutdown       int32
	chain          *blockchain.BlockChain
	txMemPool      *mempool.TxPool
	chainParams    *chaincfg.Params
	progressLogger *netsync.BlockProgressLogger
	msgChan        chan interface{}
	wg             sync.WaitGroup
	quit           chan struct{}

	// These fields should only be accessed from the blockHandler thread
	rejectedTxns    map[chainhash.Hash]struct{}
	requestedTxns   map[chainhash.Hash]struct{}
	requestedBlocks map[chainhash.Hash]struct{}
	syncPeer        *peerpkg.Peer
	peerStates      map[*peerpkg.Peer]*peerSyncState

	// The following fields are used for headers-first mode.
	headersFirstMode bool
	headerList       *list.List
	startHeader      *list.Element
	nextCheckpoint   *chaincfg.Checkpoint

	// An optional fee estimator.
	feeEstimator *mempool.FeeEstimator
}

// newPeerMsg signifies a newly connected peer to the block handler.
type newPeerMsg struct {
	peer *peerpkg.Peer
}

// blockMsg packages a bitcoin block message and the peer it came from together
// so the block handler has access to that information.
type blockMsg struct {
	block *btcutil.Block
	peer  *peerpkg.Peer
	reply chan struct{}
}

// invMsg packages a bitcoin inv message and the peer it came from together
// so the block handler has access to that information.
type invMsg struct {
	inv  *wire.MsgInv
	peer *peerpkg.Peer
}

// headersMsg packages a bitcoin headers message and the peer it came from
// together so the block handler has access to that information.
type headersMsg struct {
	headers *wire.MsgHeaders
	peer    *peerpkg.Peer
}

// donePeerMsg signifies a newly disconnected peer to the block handler.
type donePeerMsg struct {
	peer *peerpkg.Peer
}

// txMsg packages a bitcoin tx message and the peer it came from together
// so the block handler has access to that information.
type txMsg struct {
	tx    *btcutil.Tx
	peer  *peerpkg.Peer
	reply chan struct{}
}

// getSyncPeerMsg is a message type to be sent across the message channel for
// retrieving the current sync peer.
type getSyncPeerMsg struct {
	reply chan int32
}

// processBlockResponse is a response sent to the reply channel of a
// processBlockMsg.
type processBlockResponse struct {
	isOrphan bool
	err      error
}

// processBlockMsg is a message type to be sent across the message channel
// for requested a block is processed.  Note this call differs from blockMsg
// above in that blockMsg is intended for blocks that came from peers and have
// extra handling whereas this message essentially is just a concurrent safe
// way to call ProcessBlock on the internal block chain instance.
type processBlockMsg struct {
	block *btcutil.Block
	flags blockchain.BehaviorFlags
	reply chan processBlockResponse
}

// isCurrentMsg is a message type to be sent across the message channel for
// requesting whether or not the sync manager believes it is synced with the
// currently connected peers.
type isCurrentMsg struct {
	reply chan bool
}

// pauseMsg is a message type to be sent across the message channel for
// pausing the sync manager.  This effectively provides the caller with
// exclusive access over the manager until a receive is performed on the
// unpause channel.
type pauseMsg struct {
	unpause <-chan struct{}
}

// headerNode is used as a node in a list of headers that are linked together
// between checkpoints.
type headerNode struct {
	height int32
	hash   *chainhash.Hash
}

// peerSyncState stores additional information that the SyncManager tracks
// about a peer.
type peerSyncState struct {
	syncCandidate   bool
	requestQueue    []*wire.InvVect
	requestedTxns   map[chainhash.Hash]struct{}
	requestedBlocks map[chainhash.Hash]struct{}
}
