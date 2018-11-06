// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"container/list"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/parallelcointeam/btclog"
	"github.com/parallelcointeam/btcutil"
	"github.com/parallelcointeam/pod/blockchain"
	"github.com/parallelcointeam/pod/chaincfg"
	"github.com/parallelcointeam/pod/chaincfg/chainhash"
	"github.com/parallelcointeam/pod/database"
	"github.com/parallelcointeam/pod/mempool"
	peerpkg "github.com/parallelcointeam/pod/peer"
	"github.com/parallelcointeam/pod/wire"
)

const (
	// minInFlightBlocks is the minimum number of blocks that should be
	// in the request queue for headers-first mode before requesting
	// more.
	minInFlightBlocks = 10

	// maxRejectedTxns is the maximum number of rejected transactions
	// hashes to store in memory.
	maxRejectedTxns = 1000

	// maxRequestedBlocks is the maximum number of requested block
	// hashes to store in memory.
	maxRequestedBlocks = wire.MaxInvPerMsg

	// maxRequestedTxns is the maximum number of requested transactions
	// hashes to store in memory.
	maxRequestedTxns = wire.MaxInvPerMsg
)

// zeroHash is the zero value hash (all zeros).  It is defined as a convenience.
var zeroHash chainhash.Hash

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

// peerSyncState stores additional information that the BlockSyncManager tracks
// about a peer.
type peerSyncState struct {
	syncCandidate   bool
	requestQueue    []*wire.InvVect
	requestedTxns   map[chainhash.Hash]struct{}
	requestedBlocks map[chainhash.Hash]struct{}
}

// BlockSyncManager is used to communicate block related messages with peers. The
// BlockSyncManager is started as by executing Start() in a goroutine. Once started,
// it selects peers to sync from and starts the initial block download. Once the
// chain is in sync, the BlockSyncManager handles incoming block and header
// notifications and relays announcements of new blocks to peers.
type BlockSyncManager struct {
	peerNotifier   PeerNotifier
	started        int32
	shutdown       int32
	chain          *blockchain.BlockChain
	txMemPool      *mempool.TxPool
	chainParams    *chaincfg.Params
	progressLogger *BlockProgressLogger
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

// // resetHeaderState sets the headers-first mode state to values appropriate for
// // syncing from a new peer.
// func (bsm *BlockSyncManager) resetHeaderState(newestHash *chainhash.Hash, newestHeight int32) {
// 	bsm.headersFirstMode = false
// 	bsm.headerList.Init()
// 	bsm.startHeader = nil

// 	// When there is a next checkpoint, add an entry for the latest known
// 	// block into the header pool.  This allows the next downloaded header
// 	// to prove it links to the chain properly.
// 	if bsm.nextCheckpoint != nil {
// 		node := headerNode{height: newestHeight, hash: newestHash}
// 		bsm.headerList.PushBack(&node)
// 	}
// }

// // findNextHeaderCheckpoint returns the next checkpoint after the passed height.
// // It returns nil when there is not one either because the height is already
// // later than the final checkpoint or some other reason such as disabled
// // checkpoints.
// func (bsm *BlockSyncManager) findNextHeaderCheckpoint(height int32) *chaincfg.Checkpoint {
// 	checkpoints := bsm.chain.Checkpoints()
// 	if len(checkpoints) == 0 {
// 		return nil
// 	}

// 	// There is no next checkpoint if the height is already after the final
// 	// checkpoint.
// 	finalCheckpoint := &checkpoints[len(checkpoints)-1]
// 	if height >= finalCheckpoint.Height {
// 		return nil
// 	}

// 	// Find the next checkpoint.
// 	nextCheckpoint := finalCheckpoint
// 	for i := len(checkpoints) - 2; i >= 0; i-- {
// 		if height >= checkpoints[i].Height {
// 			break
// 		}
// 		nextCheckpoint = &checkpoints[i]
// 	}
// 	return nextCheckpoint
// }

// startSync will choose the best peer among the available candidate peers to
// download/sync the blockchain from.  When syncing is already running, it
// simply returns.  It also examines the candidates for any which are no longer
// candidates and removes them as needed.
func (bsm *BlockSyncManager) startSync() {
	// Return now if we're already syncing.
	if bsm.syncPeer != nil {
		return
	}

	// // Once the segwit soft-fork package has activated, we only
	// // want to sync from peers which are witness enabled to ensure
	// // that we fully validate all blockchain data.
	// segwitActive, err := bsm.chain.IsDeploymentActive(chaincfg.DeploymentSegwit)
	// if err != nil {
	// 	Log.Errorf("Unable to query for segwit soft-fork state: %v", err)
	// 	return
	// }

	best := bsm.chain.BestSnapshot()
	var bestPeer *peerpkg.Peer
	for peer, state := range bsm.peerStates {
		if !state.syncCandidate {
			continue
		}

		// if segwitActive && !peer.IsWitnessEnabled() {
		// 	Log.Debugf("peer %v not witness enabled, skipping", peer)
		// 	continue
		// }

		// Remove sync candidate peers that are no longer candidates due
		// to passing their latest known block.  NOTE: The < is
		// intentional as opposed to <=.  While technically the peer
		// doesn't have a later block when it's equal, it will likely
		// have one soon so it is a reasonable choice.  It also allows
		// the case where both are at 0 such as during regression test.
		if peer.LastBlock() < best.Height {
			state.syncCandidate = false
			continue
		}

		// TODO(davec): Use a better algorithm to choose the best peer.
		// For now, just pick the first available candidate.
		bestPeer = peer
	}

	// Start syncing from the best peer if one was selected.
	if bestPeer != nil {
		// Clear the requestedBlocks if the sync peer changes, otherwise
		// we may ignore blocks we need that the last sync peer failed
		// to send.
		bsm.requestedBlocks = make(map[chainhash.Hash]struct{})

		locator, err := bsm.chain.LatestBlockLocator()
		if err != nil {
			Log.Errorf("Failed to get block locator for the "+
				"latest block: %v", err)
			return
		}

		Log.Infof("Syncing to block height %d from peer %v",
			bestPeer.LastBlock(), bestPeer.Addr())

		// When the current height is less than a known checkpoint we
		// can use block headers to learn about which blocks comprise
		// the chain up to the checkpoint and perform less validation
		// for them.  This is possible since each header contains the
		// hash of the previous header and a merkle root.  Therefore if
		// we validate all of the received headers link together
		// properly and the checkpoint hashes match, we can be sure the
		// hashes for the blocks in between are accurate.  Further, once
		// the full blocks are downloaded, the merkle root is computed
		// and compared against the value in the header which proves the
		// full block hasn't been tampered with.
		//
		// Once we have passed the final checkpoint, or checkpoints are
		// disabled, use standard inv messages learn about the blocks
		// and fully validate them.  Finally, regression test mode does
		// not support the headers-first approach so do normal block
		// downloads when in regression test mode.
		if bsm.nextCheckpoint != nil &&
			best.Height < bsm.nextCheckpoint.Height &&
			bsm.chainParams != &chaincfg.RegressionNetParams {

			bestPeer.PushGetHeadersMsg(locator, bsm.nextCheckpoint.Hash)
			bsm.headersFirstMode = true
			Log.Infof("Downloading headers for blocks %d to "+
				"%d from peer %s", best.Height+1,
				bsm.nextCheckpoint.Height, bestPeer.Addr())
		} else {
			bestPeer.PushGetBlocksMsg(locator, &zeroHash)
		}
		bsm.syncPeer = bestPeer
	} else {
		Log.Warnf("No sync peer candidates available")
	}
}

// isSyncCandidate returns whether or not the peer is a candidate to consider
// syncing from.
func (bsm *BlockSyncManager) isSyncCandidate(peer *peerpkg.Peer) bool {
	// Typically a peer is not a candidate for sync if it's not a full node,
	// however regression test is special in that the regression tool is
	// not a full node and still needs to be considered a sync candidate.
	if bsm.chainParams == &chaincfg.RegressionNetParams {
		// The peer is not a candidate if it's not coming from localhost
		// or the hostname can't be determined for some reason.
		host, _, err := net.SplitHostPort(peer.Addr())
		if err != nil {
			return false
		}

		if host != "127.0.0.1" && host != "localhost" {
			return false
		}
	} else {
		// // The peer is not a candidate for sync if it's not a full
		// // node. Additionally, if the segwit soft-fork package has
		// // activated, then the peer must also be upgraded.
		// segwitActive, err := bsm.chain.IsDeploymentActive(chaincfg.DeploymentSegwit)
		// if err != nil {
		// 	Log.Errorf("Unable to query for segwit "+
		// 		"soft-fork state: %v", err)
		// }
		// nodeServices := peer.Services()
		// if nodeServices&wire.SFNodeNetwork != wire.SFNodeNetwork ||
		// 	(segwitActive && !peer.IsWitnessEnabled()) {
		// 	return false
		// }
	}

	// Candidate if all checks passed.
	return true
}

// handleNewPeerMsg deals with new peers that have signalled they may
// be considered as a sync peer (they have already successfully negotiated).  It
// also starts syncing if needed.  It is invoked from the syncHandler goroutine.
func (bsm *BlockSyncManager) handleNewPeerMsg(peer *peerpkg.Peer) {
	// Ignore if in the process of shutting down.
	if atomic.LoadInt32(&bsm.shutdown) != 0 {
		return
	}

	Log.Infof("New valid peer %s (%s)", peer, peer.UserAgent())

	// Initialize the peer state
	isSyncCandidate := bsm.isSyncCandidate(peer)
	bsm.peerStates[peer] = &peerSyncState{
		syncCandidate:   isSyncCandidate,
		requestedTxns:   make(map[chainhash.Hash]struct{}),
		requestedBlocks: make(map[chainhash.Hash]struct{}),
	}

	// Start syncing by choosing the best candidate if needed.
	if isSyncCandidate && bsm.syncPeer == nil {
		bsm.startSync()
	}
}

// handleDonePeerMsg deals with peers that have signalled they are done.  It
// removes the peer as a candidate for syncing and in the case where it was
// the current sync peer, attempts to select a new best peer to sync from.  It
// is invoked from the syncHandler goroutine.
func (bsm *BlockSyncManager) handleDonePeerMsg(peer *peerpkg.Peer) {
	state, exists := bsm.peerStates[peer]
	if !exists {
		Log.Warnf("Received done peer message for unknown peer %s", peer)
		return
	}

	// Remove the peer from the list of candidate peers.
	delete(bsm.peerStates, peer)

	Log.Infof("Lost peer %s", peer)

	// Remove requested transactions from the global map so that they will
	// be fetched from elsewhere next time we get an inv.
	for txHash := range state.requestedTxns {
		delete(bsm.requestedTxns, txHash)
	}

	// Remove requested blocks from the global map so that they will be
	// fetched from elsewhere next time we get an inv.
	// TODO: we could possibly here check which peers have these blocks
	// and request them now to speed things up a little.
	for blockHash := range state.requestedBlocks {
		delete(bsm.requestedBlocks, blockHash)
	}

	// Attempt to find a new peer to sync from if the quitting peer is the
	// sync peer.  Also, reset the headers-first state if in headers-first
	// mode so
	if bsm.syncPeer == peer {
		bsm.syncPeer = nil
		// if bsm.headersFirstMode {
		// 	best := bsm.chain.BestSnapshot()
		// 	// bsm.resetHeaderState(&best.Hash, best.Height)
		// }
		bsm.startSync()
	}
}

// handleTxMsg handles transaction messages from all peers.
func (bsm *BlockSyncManager) handleTxMsg(tmsg *txMsg) {
	peer := tmsg.peer
	state, exists := bsm.peerStates[peer]
	if !exists {
		Log.Warnf("Received tx message from unknown peer %s", peer)
		return
	}

	// NOTE:  BitcoinJ, and possibly other wallets, don't follow the spec of
	// sending an inventory message and allowing the remote peer to decide
	// whether or not they want to request the transaction via a getdata
	// message.  Unfortunately, the reference implementation permits
	// unrequested data, so it has allowed wallets that don't follow the
	// spec to proliferate.  While this is not ideal, there is no check here
	// to disconnect peers for sending unsolicited transactions to provide
	// interoperability.
	txHash := tmsg.tx.Hash()

	// Ignore transactions that we have already rejected.  Do not
	// send a reject message here because if the transaction was already
	// rejected, the transaction was unsolicited.
	if _, exists = bsm.rejectedTxns[*txHash]; exists {
		Log.Debugf("Ignoring unsolicited previously rejected "+
			"transaction %v from %s", txHash, peer)
		return
	}

	// Process the transaction to include validation, insertion in the
	// memory pool, orphan handling, etc.
	acceptedTxs, err := bsm.txMemPool.ProcessTransaction(tmsg.tx,
		true, true, mempool.Tag(peer.ID()))

	// Remove transaction from request maps. Either the mempool/chain
	// already knows about it and as such we shouldn't have any more
	// instances of trying to fetch it, or we failed to insert and thus
	// we'll retry next time we get an inv.
	delete(state.requestedTxns, *txHash)
	delete(bsm.requestedTxns, *txHash)

	if err != nil {
		// Do not request this transaction again until a new block
		// has been processed.
		bsm.rejectedTxns[*txHash] = struct{}{}
		bsm.limitMap(bsm.rejectedTxns, maxRejectedTxns)

		// When the error is a rule error, it means the transaction was
		// simply rejected as opposed to something actually going wrong,
		// so log it as such.  Otherwise, something really did go wrong,
		// so log it as an actual error.
		if _, ok := err.(mempool.RuleError); ok {
			Log.Debugf("Rejected transaction %v from %s: %v",
				txHash, peer, err)
		} else {
			Log.Errorf("Failed to process transaction %v: %v",
				txHash, err)
		}

		// Convert the error into an appropriate reject message and
		// send it.
		code, reason := mempool.ErrToRejectErr(err)
		peer.PushRejectMsg(wire.CmdTx, code, reason, txHash, false)
		return
	}

	bsm.peerNotifier.AnnounceNewTransactions(acceptedTxs)
}

// current returns true if we believe we are synced with our peers, false if we
// still have blocks to check
func (bsm *BlockSyncManager) current() bool {
	if !bsm.chain.IsCurrent() {
		return false
	}

	// if blockChain thinks we are current and we have no syncPeer it
	// is probably right.
	if bsm.syncPeer == nil {
		return true
	}

	// No matter what chain thinks, if we are below the block we are syncing
	// to we are not current.
	if bsm.chain.BestSnapshot().Height < bsm.syncPeer.LastBlock() {
		return false
	}
	return true
}

// handleBlockMsg handles block messages from all peers.
func (bsm *BlockSyncManager) handleBlockMsg(bmsg *blockMsg) {
	Log.Debug("handleBlockMsg")
	peer := bmsg.peer
	state, exists := bsm.peerStates[peer]
	if !exists {
		Log.Warnf("Received block message from unknown peer %s", peer)
		return
	}

	// If we didn't ask for this block then the peer is misbehaving.
	blockHash := bmsg.block.Hash()
	if _, exists = state.requestedBlocks[*blockHash]; !exists {
		// The regression test intentionally sends some blocks twice
		// to test duplicate block insertion fails.  Don't disconnect
		// the peer or ignore the block when we're in regression test
		// mode in this case so the chain code is actually fed the
		// duplicate blocks.
		if bsm.chainParams != &chaincfg.RegressionNetParams {
			Log.Warnf("Got unrequested block %v from %s -- "+
				"disconnecting", blockHash, peer.Addr())
			peer.Disconnect()
			return
		}
	}

	// When in headers-first mode, if the block matches the hash of the
	// first header in the list of headers that are being fetched, it's
	// eligible for less validation since the headers have already been
	// verified to link together and are valid up to the next checkpoint.
	// Also, remove the list entry for all blocks except the checkpoint
	// since it is needed to verify the next round of headers links
	// properly.
	// isCheckpointBlock := false
	behaviorFlags := blockchain.BFNone
	// if bsm.headersFirstMode {
	// 	firstNodeEl := bsm.headerList.Front()
	// 	if firstNodeEl != nil {
	// 		firstNode := firstNodeEl.Value.(*headerNode)
	// 		if blockHash.IsEqual(firstNode.hash) {
	// 			behaviorFlags |= blockchain.BFFastAdd
	// 			if firstNode.hash.IsEqual(bsm.nextCheckpoint.Hash) {
	// 				isCheckpointBlock = true
	// 			} else {
	// 				bsm.headerList.Remove(firstNodeEl)
	// 			}
	// 		}
	// 	}
	// }

	// Remove block from request maps. Either chain will know about it and
	// so we shouldn't have any more instances of trying to fetch it, or we
	// will fail the insert and thus we'll retry next time we get an inv.
	delete(state.requestedBlocks, *blockHash)
	delete(bsm.requestedBlocks, *blockHash)

	// Process the block to include validation, best chain selection, orphan
	// handling, etc.
	_, isOrphan, err := bsm.chain.ProcessBlock(bmsg.block, behaviorFlags)
	if err != nil {
		// When the error is a rule error, it means the block was simply
		// rejected as opposed to something actually going wrong, so log
		// it as such.  Otherwise, something really did go wrong, so log
		// it as an actual error.
		if _, ok := err.(blockchain.RuleError); ok {
			Log.Infof("Rejected block %v from %s: %v", blockHash,
				peer, err)
			fmt.Println("height", bmsg.block.Height())
		} else {
			Log.Errorf("Failed to process block %v: %v",
				blockHash, err)
		}
		if dbErr, ok := err.(database.Error); ok && dbErr.ErrorCode ==
			database.ErrCorruption {
			panic(dbErr)
		}

		// Convert the error into an appropriate reject message and
		// send it.
		code, reason := mempool.ErrToRejectErr(err)
		peer.PushRejectMsg(wire.CmdBlock, code, reason, blockHash, false)
		return
	}

	// Meta-data about the new block this peer is reporting. We use this
	// below to update this peer's lastest block height and the heights of
	// other peers based on their last announced block hash. This allows us
	// to dynamically update the block heights of peers, avoiding stale
	// heights when looking for a new sync peer. Upon acceptance of a block
	// or recognition of an orphan, we also use this information to update
	// the block heights over other peers who's invs may have been ignored
	// if we are actively syncing while the chain is not yet current or
	// who may have lost the lock announcment race.
	var heightUpdate int32
	var blkHashUpdate *chainhash.Hash

	// Request the parents for the orphan block from the peer that sent it.
	if isOrphan {
		// We've just received an orphan block from a peer. In order
		// to update the height of the peer, we try to extract the
		// block height from the scriptSig of the coinbase transaction.
		// Extraction is only attempted if the block's version is
		// high enough (ver 2+).
		header := &bmsg.block.MsgBlock().Header
		if blockchain.ShouldHaveSerializedBlockHeight(header) {
			coinbaseTx := bmsg.block.Transactions()[0]
			cbHeight, err := blockchain.ExtractCoinbaseHeight(coinbaseTx)
			if err != nil {
				Log.Warnf("Unable to extract height from "+
					"coinbase tx: %v", err)
			} else {
				Log.Debugf("Extracted height of %v from "+
					"orphan block", cbHeight)
				heightUpdate = cbHeight
				blkHashUpdate = blockHash
			}
		}

		orphanRoot := bsm.chain.GetOrphanRoot(blockHash)
		locator, err := bsm.chain.LatestBlockLocator()
		if err != nil {
			Log.Warnf("Failed to get block locator for the "+
				"latest block: %v", err)
		} else {
			peer.PushGetBlocksMsg(locator, orphanRoot)
		}
	} else {
		// When the block is not an orphan, log information about it and
		// update the chain state.
		bsm.progressLogger.LogBlockHeight(bmsg.block)

		// Update this peer's latest block height, for future
		// potential sync node candidacy.
		best := bsm.chain.BestSnapshot()
		heightUpdate = best.Height
		blkHashUpdate = &best.Hash

		// Clear the rejected transactions.
		bsm.rejectedTxns = make(map[chainhash.Hash]struct{})
	}

	// Update the block height for this peer. But only send a message to
	// the server for updating peer heights if this is an orphan or our
	// chain is "current". This avoids sending a spammy amount of messages
	// if we're syncing the chain from scratch.
	if blkHashUpdate != nil && heightUpdate != 0 {
		peer.UpdateLastBlockHeight(heightUpdate)
		if isOrphan || bsm.current() {
			go bsm.peerNotifier.UpdatePeerHeights(blkHashUpdate, heightUpdate,
				peer)
		}
	}

	// // Nothing more to do if we aren't in headers-first mode.
	// if !bsm.headersFirstMode {
	// 	return
	// }

	// // This is headers-first mode, so if the block is not a checkpoint
	// // request more blocks using the header list when the request queue is
	// // getting short.
	// if !isCheckpointBlock {
	// 	if bsm.startHeader != nil &&
	// 		len(state.requestedBlocks) < minInFlightBlocks {
	// 		bsm.fetchHeaderBlocks()
	// 	}
	// 	return
	// }

	// // This is headers-first mode and the block is a checkpoint.  When
	// // there is a next checkpoint, get the next round of headers by asking
	// // for headers starting from the block after this one up to the next
	// // checkpoint.
	// prevHeight := bsm.nextCheckpoint.Height
	// prevHash := bsm.nextCheckpoint.Hash
	// bsm.nextCheckpoint = bsm.findNextHeaderCheckpoint(prevHeight)
	// if bsm.nextCheckpoint != nil {
	// 	locator := blockchain.BlockLocator([]*chainhash.Hash{prevHash})
	// 	err := peer.PushGetHeadersMsg(locator, bsm.nextCheckpoint.Hash)
	// 	if err != nil {
	// 		Log.Warnf("Failed to send getheaders message to "+
	// 			"peer %s: %v", peer.Addr(), err)
	// 		return
	// 	}
	// 	Log.Infof("Downloading headers for blocks %d to %d from "+
	// 		"peer %s", prevHeight+1, bsm.nextCheckpoint.Height,
	// 		bsm.syncPeer.Addr())
	// 	return
	// }

	// // This is headers-first mode, the block is a checkpoint, and there are
	// // no more checkpoints, so switch to normal mode by requesting blocks
	// // from the block after this one up to the end of the chain (zero hash).
	// bsm.headersFirstMode = false
	// bsm.headerList.Init()
	// Log.Infof("Reached the final checkpoint -- switching to normal mode")
	// locator := blockchain.BlockLocator([]*chainhash.Hash{blockHash})
	// err = peer.PushGetBlocksMsg(locator, &zeroHash)
	// if err != nil {
	// 	Log.Warnf("Failed to send getblocks message to peer %s: %v",
	// 		peer.Addr(), err)
	// 	return
	// }
}

// // fetchHeaderBlocks creates and sends a request to the syncPeer for the next
// // list of blocks to be downloaded based on the current list of headers.
// func (bsm *BlockSyncManager) fetchHeaderBlocks() {
// 	// Nothing to do if there is no start header.
// 	if bsm.startHeader == nil {
// 		Log.Warnf("fetchHeaderBlocks called with no start header")
// 		return
// 	}

// 	// Build up a getdata request for the list of blocks the headers
// 	// describe.  The size hint will be limited to wire.MaxInvPerMsg by
// 	// the function, so no need to double check it here.
// 	gdmsg := wire.NewMsgGetDataSizeHint(uint(bsm.headerList.Len()))
// 	numRequested := 0
// 	for e := bsm.startHeader; e != nil; e = e.Next() {
// 		node, ok := e.Value.(*headerNode)
// 		if !ok {
// 			Log.Warn("Header list node type is not a headerNode")
// 			continue
// 		}

// 		iv := wire.NewInvVect(wire.InvTypeBlock, node.hash)
// 		haveInv, err := bsm.haveInventory(iv)
// 		if err != nil {
// 			Log.Warnf("Unexpected failure when checking for "+
// 				"existing inventory during header block "+
// 				"fetch: %v", err)
// 		}
// 		if !haveInv {
// 			syncPeerState := bsm.peerStates[bsm.syncPeer]

// 			bsm.requestedBlocks[*node.hash] = struct{}{}
// 			syncPeerState.requestedBlocks[*node.hash] = struct{}{}

// 			// If we're fetching from a witness enabled peer
// 			// post-fork, then ensure that we receive all the
// 			// witness data in the blocks.
// 			if bsm.syncPeer.IsWitnessEnabled() {
// 				iv.Type = wire.InvTypeWitnessBlock
// 			}

// 			gdmsg.AddInvVect(iv)
// 			numRequested++
// 		}
// 		bsm.startHeader = e.Next()
// 		if numRequested >= wire.MaxInvPerMsg {
// 			break
// 		}
// 	}
// 	if len(gdmsg.InvList) > 0 {
// 		bsm.syncPeer.QueueMessage(gdmsg, nil)
// 	}
// }

// // handleHeadersMsg handles block header messages from all peers.  Headers are
// // requested when performing a headers-first sync.
// func (bsm *BlockSyncManager) handleHeadersMsg(hmsg *headersMsg) {
// 	peer := hmsg.peer
// 	_, exists := bsm.peerStates[peer]
// 	if !exists {
// 		Log.Warnf("Received headers message from unknown peer %s", peer)
// 		return
// 	}

// 	// The remote peer is misbehaving if we didn't request headers.
// 	msg := hmsg.headers
// 	numHeaders := len(msg.Headers)
// 	if !bsm.headersFirstMode {
// 		Log.Warnf("Got %d unrequested headers from %s -- "+
// 			"disconnecting", numHeaders, peer.Addr())
// 		peer.Disconnect()
// 		return
// 	}

// 	// Nothing to do for an empty headers message.
// 	if numHeaders == 0 {
// 		return
// 	}

// 	// Process all of the received headers ensuring each one connects to the
// 	// previous and that checkpoints match.
// 	receivedCheckpoint := false
// 	var finalHash *chainhash.Hash
// 	for _, blockHeader := range msg.Headers {
// 		blockHash := blockHeader.BlockHash()
// 		finalHash = &blockHash

// 		// Ensure there is a previous header to compare against.
// 		prevNodeEl := bsm.headerList.Back()
// 		if prevNodeEl == nil {
// 			Log.Warnf("Header list does not contain a previous" +
// 				"element as expected -- disconnecting peer")
// 			peer.Disconnect()
// 			return
// 		}

// 		// Ensure the header properly connects to the previous one and
// 		// add it to the list of headers.
// 		node := headerNode{hash: &blockHash}
// 		prevNode := prevNodeEl.Value.(*headerNode)
// 		if prevNode.hash.IsEqual(&blockHeader.PrevBlock) {
// 			node.height = prevNode.height + 1
// 			e := bsm.headerList.PushBack(&node)
// 			if bsm.startHeader == nil {
// 				bsm.startHeader = e
// 			}
// 		} else {
// 			Log.Warnf("Received block header that does not "+
// 				"properly connect to the chain from peer %s "+
// 				"-- disconnecting", peer.Addr())
// 			peer.Disconnect()
// 			return
// 		}

// 		// Verify the header at the next checkpoint height matches.
// 		if node.height == bsm.nextCheckpoint.Height {
// 			if node.hash.IsEqual(bsm.nextCheckpoint.Hash) {
// 				receivedCheckpoint = true
// 				Log.Infof("Verified downloaded block "+
// 					"header against checkpoint at height "+
// 					"%d/hash %s", node.height, node.hash)
// 			} else {
// 				Log.Warnf("Block header at height %d/hash "+
// 					"%s from peer %s does NOT match "+
// 					"expected checkpoint hash of %s -- "+
// 					"disconnecting", node.height,
// 					node.hash, peer.Addr(),
// 					bsm.nextCheckpoint.Hash)
// 				peer.Disconnect()
// 				return
// 			}
// 			break
// 		}
// 	}

// 	// When this header is a checkpoint, switch to fetching the blocks for
// 	// all of the headers since the last checkpoint.
// 	if receivedCheckpoint {
// 		// Since the first entry of the list is always the final block
// 		// that is already in the database and is only used to ensure
// 		// the next header links properly, it must be removed before
// 		// fetching the blocks.
// 		bsm.headerList.Remove(bsm.headerList.Front())
// 		Log.Infof("Received %v block headers: Fetching blocks",
// 			bsm.headerList.Len())
// 		bsm.progressLogger.SetLastLogTime(time.Now())
// 		bsm.fetchHeaderBlocks()
// 		return
// 	}

// 	// This header is not a checkpoint, so request the next batch of
// 	// headers starting from the latest known header and ending with the
// 	// next checkpoint.
// 	locator := blockchain.BlockLocator([]*chainhash.Hash{finalHash})
// 	err := peer.PushGetHeadersMsg(locator, bsm.nextCheckpoint.Hash)
// 	if err != nil {
// 		Log.Warnf("Failed to send getheaders message to "+
// 			"peer %s: %v", peer.Addr(), err)
// 		return
// 	}
// }

// haveInventory returns whether or not the inventory represented by the passed
// inventory vector is known.  This includes checking all of the various places
// inventory can be when it is in different states such as blocks that are part
// of the main chain, on a side chain, in the orphan pool, and transactions that
// are in the memory pool (either the main pool or orphan pool).
func (bsm *BlockSyncManager) haveInventory(invVect *wire.InvVect) (bool, error) {
	switch invVect.Type {
	case wire.InvTypeWitnessBlock:
		fallthrough
	case wire.InvTypeBlock:
		// Ask chain if the block is known to it in any form (main
		// chain, side chain, or orphan).
		return bsm.chain.HaveBlock(&invVect.Hash)

	case wire.InvTypeWitnessTx:
		fallthrough
	case wire.InvTypeTx:
		// Ask the transaction memory pool if the transaction is known
		// to it in any form (main pool or orphan).
		if bsm.txMemPool.HaveTransaction(&invVect.Hash) {
			return true, nil
		}

		// Check if the transaction exists from the point of view of the
		// end of the main chain.  Note that this is only a best effort
		// since it is expensive to check existence of every output and
		// the only purpose of this check is to avoid downloading
		// already known transactions.  Only the first two outputs are
		// checked because the vast majority of transactions consist of
		// two outputs where one is some form of "pay-to-somebody-else"
		// and the other is a change output.
		prevOut := wire.OutPoint{Hash: invVect.Hash}
		for i := uint32(0); i < 2; i++ {
			prevOut.Index = i
			entry, err := bsm.chain.FetchUtxoEntry(prevOut)
			if err != nil {
				return false, err
			}
			if entry != nil && !entry.IsSpent() {
				return true, nil
			}
		}

		return false, nil
	}

	// The requested inventory is is an unsupported type, so just claim
	// it is known to avoid requesting it.
	return true, nil
}

// handleInvMsg handles inv messages from all peers.
// We examine the inventory advertised by the remote peer and act accordingly.
func (bsm *BlockSyncManager) handleInvMsg(imsg *invMsg) {
	Log.Debug("handleInvMsg")
	peer := imsg.peer
	state, exists := bsm.peerStates[peer]
	if !exists {
		Log.Warnf("Received inv message from unknown peer %s", peer)
		return
	}

	// Attempt to find the final block in the inventory list.  There may
	// not be one.
	lastBlock := -1
	invVects := imsg.inv.InvList
	for i := len(invVects) - 1; i >= 0; i-- {
		if invVects[i].Type == wire.InvTypeBlock {
			lastBlock = i
			break
		}
	}

	// If this inv contains a block announcement, and this isn't coming from
	// our current sync peer or we're current, then update the last
	// announced block for this peer. We'll use this information later to
	// update the heights of peers based on blocks we've accepted that they
	// previously announced.
	if lastBlock != -1 && (peer != bsm.syncPeer || bsm.current()) {
		peer.UpdateLastAnnouncedBlock(&invVects[lastBlock].Hash)
	}

	// Ignore invs from peers that aren't the sync if we are not current.
	// Helps prevent fetching a mass of orphans.
	if peer != bsm.syncPeer && !bsm.current() {
		return
	}

	// If our chain is current and a peer announces a block we already
	// know of, then update their current block height.
	if lastBlock != -1 && bsm.current() {
		blkHeight, err := bsm.chain.BlockHeightByHash(&invVects[lastBlock].Hash)
		if err == nil {
			peer.UpdateLastBlockHeight(blkHeight)
		}
	}

	// Request the advertised inventory if we don't already have it.  Also,
	// request parent blocks of orphans if we receive one we already have.
	// Finally, attempt to detect potential stalls due to long side chains
	// we already have and request more blocks to prevent them.
	for i, iv := range invVects {
		// Ignore unsupported inventory types.
		switch iv.Type {
		case wire.InvTypeBlock:
		case wire.InvTypeTx:
		case wire.InvTypeWitnessBlock:
		case wire.InvTypeWitnessTx:
		default:
			continue
		}

		// Add the inventory to the cache of known inventory
		// for the peer.
		peer.AddKnownInventory(iv)

		// Ignore inventory when we're in headers-first mode.
		if bsm.headersFirstMode {
			continue
		}

		// Request the inventory if we don't already have it.
		haveInv, err := bsm.haveInventory(iv)
		if err != nil {
			Log.Warnf("Unexpected failure when checking for "+
				"existing inventory during inv message "+
				"processing: %v", err)
			continue
		}
		if !haveInv {
			if iv.Type == wire.InvTypeTx {
				// Skip the transaction if it has already been
				// rejected.
				if _, exists := bsm.rejectedTxns[iv.Hash]; exists {
					continue
				}
			}

			// Ignore invs block invs from non-witness enabled
			// peers, as after segwit activation we only want to
			// download from peers that can provide us full witness
			// data for blocks.

			// PARALLELCOIN HAS NO WITNESS STUFF
			// if !peer.IsWitnessEnabled() && iv.Type == wire.InvTypeBlock {
			// 	continue
			// }

			// Add it to the request queue.
			state.requestQueue = append(state.requestQueue, iv)
			continue
		}

		if iv.Type == wire.InvTypeBlock {
			// The block is an orphan block that we already have.
			// When the existing orphan was processed, it requested
			// the missing parent blocks.  When this scenario
			// happens, it means there were more blocks missing
			// than are allowed into a single inventory message.  As
			// a result, once this peer requested the final
			// advertised block, the remote peer noticed and is now
			// resending the orphan block as an available block
			// to signal there are more missing blocks that need to
			// be requested.
			if bsm.chain.IsKnownOrphan(&iv.Hash) {
				// Request blocks starting at the latest known
				// up to the root of the orphan that just came
				// in.
				orphanRoot := bsm.chain.GetOrphanRoot(&iv.Hash)
				locator, err := bsm.chain.LatestBlockLocator()
				if err != nil {
					Log.Errorf("PEER: Failed to get block "+
						"locator for the latest block: "+
						"%v", err)
					continue
				}
				peer.PushGetBlocksMsg(locator, orphanRoot)
				continue
			}

			// We already have the final block advertised by this
			// inventory message, so force a request for more.  This
			// should only happen if we're on a really long side
			// chain.
			if i == lastBlock {
				// Request blocks after this one up to the
				// final one the remote peer knows about (zero
				// stop hash).
				locator := bsm.chain.BlockLocatorFromHash(&iv.Hash)
				peer.PushGetBlocksMsg(locator, &zeroHash)
			}
		}
	}

	// Request as much as possible at once.  Anything that won't fit into
	// the request will be requested on the next inv message.
	numRequested := 0
	gdmsg := wire.NewMsgGetData()
	requestQueue := state.requestQueue
	for len(requestQueue) != 0 {
		iv := requestQueue[0]
		requestQueue[0] = nil
		requestQueue = requestQueue[1:]

		switch iv.Type {
		case wire.InvTypeWitnessBlock:
			fallthrough
		case wire.InvTypeBlock:
			// Request the block if there is not already a pending
			// request.
			if _, exists := bsm.requestedBlocks[iv.Hash]; !exists {
				bsm.requestedBlocks[iv.Hash] = struct{}{}
				bsm.limitMap(bsm.requestedBlocks, maxRequestedBlocks)
				state.requestedBlocks[iv.Hash] = struct{}{}

				if peer.IsWitnessEnabled() {
					iv.Type = wire.InvTypeWitnessBlock
				}

				gdmsg.AddInvVect(iv)
				numRequested++
			}

		case wire.InvTypeWitnessTx:
			fallthrough
		case wire.InvTypeTx:
			// Request the transaction if there is not already a
			// pending request.
			if _, exists := bsm.requestedTxns[iv.Hash]; !exists {
				bsm.requestedTxns[iv.Hash] = struct{}{}
				bsm.limitMap(bsm.requestedTxns, maxRequestedTxns)
				state.requestedTxns[iv.Hash] = struct{}{}

				// If the peer is capable, request the txn
				// including all witness data.
				if peer.IsWitnessEnabled() {
					iv.Type = wire.InvTypeWitnessTx
				}

				gdmsg.AddInvVect(iv)
				numRequested++
			}
		}

		if numRequested >= wire.MaxInvPerMsg {
			break
		}
	}
	state.requestQueue = requestQueue
	if len(gdmsg.InvList) > 0 {
		peer.QueueMessage(gdmsg, nil)
	}
}

// limitMap is a helper function for maps that require a maximum limit by
// evicting a random transaction if adding a new value would cause it to
// overflow the maximum allowed.
func (bsm *BlockSyncManager) limitMap(m map[chainhash.Hash]struct{}, limit int) {
	if len(m)+1 > limit {
		// Remove a random entry from the map.  For most compilers, Go's
		// range statement iterates starting at a random item although
		// that is not 100% guaranteed by the spec.  The iteration order
		// is not important here because an adversary would have to be
		// able to pull off preimage attacks on the hashing function in
		// order to target eviction of specific entries anyways.
		for txHash := range m {
			delete(m, txHash)
			return
		}
	}
}

// blockHandler is the main handler for the sync manager.  It must be run as a
// goroutine.  It processes block and inv messages in a separate goroutine
// from the peer handlers so the block (MsgBlock) messages are handled by a
// single thread without needing to lock memory data structures.  This is
// important because the sync manager controls which blocks are needed and how
// the fetching should proceed.
func (bsm *BlockSyncManager) blockHandler() {
out:
	for {
		select {
		case m := <-bsm.msgChan:
			switch msg := m.(type) {
			case *newPeerMsg:
				bsm.handleNewPeerMsg(msg.peer)

			case *txMsg:
				bsm.handleTxMsg(msg)
				msg.reply <- struct{}{}

			case *blockMsg:
				bsm.handleBlockMsg(msg)
				msg.reply <- struct{}{}

			case *invMsg:
				bsm.handleInvMsg(msg)

			// case *headersMsg:
			// 	bsm.handleHeadersMsg(msg)

			case *donePeerMsg:
				bsm.handleDonePeerMsg(msg.peer)

			case getSyncPeerMsg:
				var peerID int32
				if bsm.syncPeer != nil {
					peerID = bsm.syncPeer.ID()
				}
				msg.reply <- peerID

			case processBlockMsg:
				_, isOrphan, err := bsm.chain.ProcessBlock(
					msg.block, msg.flags)
				if err != nil {
					msg.reply <- processBlockResponse{
						isOrphan: false,
						err:      err,
					}
				}

				msg.reply <- processBlockResponse{
					isOrphan: isOrphan,
					err:      nil,
				}

			case isCurrentMsg:
				msg.reply <- bsm.current()

			case pauseMsg:
				// Wait until the sender unpauses the manager.
				<-msg.unpause

			default:
				Log.Warnf("Invalid message type in block "+
					"handler: %T", msg)
			}

		case <-bsm.quit:
			break out
		}
	}

	bsm.wg.Done()
	Log.Trace("Block handler done")
}

// handleBlockchainNotification handles notifications from blockchain.  It does
// things such as request orphan block parents and relay accepted blocks to
// connected peers.
func (bsm *BlockSyncManager) handleBlockchainNotification(notification *blockchain.Notification) {
	switch notification.Type {
	// A block has been accepted into the block chain.  Relay it to other
	// peers.
	case blockchain.NTBlockAccepted:
		// Don't relay if we are not current. Other peers that are
		// current should already know about it.
		if !bsm.current() {
			return
		}

		block, ok := notification.Data.(*btcutil.Block)
		if !ok {
			Log.Warnf("Chain accepted notification is not a block.")
			break
		}

		// Generate the inventory vector and relay it.
		iv := wire.NewInvVect(wire.InvTypeBlock, block.Hash())
		bsm.peerNotifier.RelayInventory(iv, block.MsgBlock().Header)

	// A block has been connected to the main block chain.
	case blockchain.NTBlockConnected:
		block, ok := notification.Data.(*btcutil.Block)
		if !ok {
			Log.Warnf("Chain connected notification is not a block.")
			break
		}

		// Remove all of the transactions (except the coinbase) in the
		// connected block from the transaction pool.  Secondly, remove any
		// transactions which are now double spends as a result of these
		// new transactions.  Finally, remove any transaction that is
		// no longer an orphan. Transactions which depend on a confirmed
		// transaction are NOT removed recursively because they are still
		// valid.
		for _, tx := range block.Transactions()[1:] {
			bsm.txMemPool.RemoveTransaction(tx, false)
			bsm.txMemPool.RemoveDoubleSpends(tx)
			bsm.txMemPool.RemoveOrphan(tx)
			bsm.peerNotifier.TransactionConfirmed(tx)
			acceptedTxs := bsm.txMemPool.ProcessOrphans(tx)
			bsm.peerNotifier.AnnounceNewTransactions(acceptedTxs)
		}

		// Register block with the fee estimator, if it exists.
		if bsm.feeEstimator != nil {
			err := bsm.feeEstimator.RegisterBlock(block)

			// If an error is somehow generated then the fee estimator
			// has entered an invalid state. Since it doesn't know how
			// to recover, create a new one.
			if err != nil {
				bsm.feeEstimator = mempool.NewFeeEstimator(
					mempool.DefaultEstimateFeeMaxRollback,
					mempool.DefaultEstimateFeeMinRegisteredBlocks)
			}
		}

	// A block has been disconnected from the main block chain.
	case blockchain.NTBlockDisconnected:
		block, ok := notification.Data.(*btcutil.Block)
		if !ok {
			Log.Warnf("Chain disconnected notification is not a block.")
			break
		}

		// Reinsert all of the transactions (except the coinbase) into
		// the transaction pool.
		for _, tx := range block.Transactions()[1:] {
			_, _, err := bsm.txMemPool.MaybeAcceptTransaction(tx,
				false, false)
			if err != nil {
				// Remove the transaction and all transactions
				// that depend on it if it wasn't accepted into
				// the transaction pool.
				bsm.txMemPool.RemoveTransaction(tx, true)
			}
		}

		// Rollback previous block recorded by the fee estimator.
		if bsm.feeEstimator != nil {
			bsm.feeEstimator.Rollback(block.Hash())
		}
	}
}

// NewPeer informs the sync manager of a newly active peer.
func (bsm *BlockSyncManager) NewPeer(peer *peerpkg.Peer) {
	// Ignore if we are shutting down.
	if atomic.LoadInt32(&bsm.shutdown) != 0 {
		return
	}
	bsm.msgChan <- &newPeerMsg{peer: peer}
}

// QueueTx adds the passed transaction message and peer to the block handling
// queue. Responds to the done channel argument after the tx message is
// processed.
func (bsm *BlockSyncManager) QueueTx(tx *btcutil.Tx, peer *peerpkg.Peer, done chan struct{}) {
	// Don't accept more transactions if we're shutting down.
	if atomic.LoadInt32(&bsm.shutdown) != 0 {
		done <- struct{}{}
		return
	}

	bsm.msgChan <- &txMsg{tx: tx, peer: peer, reply: done}
}

// QueueBlock adds the passed block message and peer to the block handling
// queue. Responds to the done channel argument after the block message is
// processed.
func (bsm *BlockSyncManager) QueueBlock(block *btcutil.Block, peer *peerpkg.Peer, done chan struct{}) {
	// Don't accept more blocks if we're shutting down.
	if atomic.LoadInt32(&bsm.shutdown) != 0 {
		done <- struct{}{}
		return
	}

	bsm.msgChan <- &blockMsg{block: block, peer: peer, reply: done}
}

// QueueInv adds the passed inv message and peer to the block handling queue.
func (bsm *BlockSyncManager) QueueInv(inv *wire.MsgInv, peer *peerpkg.Peer) {
	// No channel handling here because peers do not need to block on inv
	// messages.
	if atomic.LoadInt32(&bsm.shutdown) != 0 {
		return
	}

	bsm.msgChan <- &invMsg{inv: inv, peer: peer}
}

// QueueHeaders adds the passed headers message and peer to the block handling
// queue.
func (bsm *BlockSyncManager) QueueHeaders(headers *wire.MsgHeaders, peer *peerpkg.Peer) {
	// No channel handling here because peers do not need to block on
	// headers messages.
	if atomic.LoadInt32(&bsm.shutdown) != 0 {
		return
	}

	bsm.msgChan <- &headersMsg{headers: headers, peer: peer}
}

// DonePeer informs the blockmanager that a peer has disconnected.
func (bsm *BlockSyncManager) DonePeer(peer *peerpkg.Peer) {
	// Ignore if we are shutting down.
	if atomic.LoadInt32(&bsm.shutdown) != 0 {
		return
	}

	bsm.msgChan <- &donePeerMsg{peer: peer}
}

// Start begins the core block handler which processes block and inv messages.
func (bsm *BlockSyncManager) Start() {
	// Already started?
	if atomic.AddInt32(&bsm.started, 1) != 1 {
		return
	}

	Log.Trace("Starting sync manager")
	bsm.wg.Add(1)
	go bsm.blockHandler()
}

// Stop gracefully shuts down the sync manager by stopping all asynchronous
// handlers and waiting for them to finish.
func (bsm *BlockSyncManager) Stop() error {
	if atomic.AddInt32(&bsm.shutdown, 1) != 1 {
		Log.Warnf("Sync manager is already in the process of " +
			"shutting down")
		return nil
	}

	Log.Infof("Sync manager shutting down")
	close(bsm.quit)
	bsm.wg.Wait()
	return nil
}

// SyncPeerID returns the ID of the current sync peer, or 0 if there is none.
func (bsm *BlockSyncManager) SyncPeerID() int32 {
	reply := make(chan int32)
	bsm.msgChan <- getSyncPeerMsg{reply: reply}
	return <-reply
}

// ProcessBlock makes use of ProcessBlock on an internal instance of a block
// chain.
func (bsm *BlockSyncManager) ProcessBlock(block *btcutil.Block, flags blockchain.BehaviorFlags) (bool, error) {
	reply := make(chan processBlockResponse, 1)
	bsm.msgChan <- processBlockMsg{block: block, flags: flags, reply: reply}
	response := <-reply
	return response.isOrphan, response.err
}

// IsCurrent returns whether or not the sync manager believes it is synced with
// the connected peers.
func (bsm *BlockSyncManager) IsCurrent() bool {
	reply := make(chan bool)
	bsm.msgChan <- isCurrentMsg{reply: reply}
	return <-reply
}

// Pause pauses the sync manager until the returned channel is closed.
//
// Note that while paused, all peer and block processing is halted.  The
// message sender should avoid pausing the sync manager for long durations.
func (bsm *BlockSyncManager) Pause() chan<- struct{} {
	c := make(chan struct{})
	bsm.msgChan <- pauseMsg{c}
	return c
}

// New constructs a new BlockSyncManager. Use Start to begin processing asynchronous
// block, tx, and inv updates.
func New(config *Config) (*BlockSyncManager, error) {
	bsm := BlockSyncManager{
		peerNotifier:    config.PeerNotifier,
		chain:           config.Chain,
		txMemPool:       config.TxMemPool,
		chainParams:     config.ChainParams,
		rejectedTxns:    make(map[chainhash.Hash]struct{}),
		requestedTxns:   make(map[chainhash.Hash]struct{}),
		requestedBlocks: make(map[chainhash.Hash]struct{}),
		peerStates:      make(map[*peerpkg.Peer]*peerSyncState),
		progressLogger:  NewBlockProgressLogger("Processed", Log),
		msgChan:         make(chan interface{}, config.MaxPeers*3),
		headerList:      list.New(),
		quit:            make(chan struct{}),
		feeEstimator:    config.FeeEstimator,
	}

	// best := bsm.chain.BestSnapshot()
	// if !config.DisableCheckpoints {
	// 	// Initialize the next checkpoint based on the current height.
	// 	bsm.nextCheckpoint = bsm.findNextHeaderCheckpoint(best.Height)
	// 	if bsm.nextCheckpoint != nil {
	// 		bsm.resetHeaderState(&best.Hash, best.Height)
	// 	}
	// } else {
	// 	Log.Info("Checkpoints are disabled")
	// }

	bsm.chain.Subscribe(bsm.handleBlockchainNotification)

	return &bsm, nil
}

// BlockProgressLogger provides periodic logging for other services in order
// to show users progress of certain "actions" involving some or all current
// blocks. Ex: syncing to best chain, indexing all blocks, etc.
type BlockProgressLogger struct {
	receivedLogBlocks int64
	receivedLogTx     int64
	lastBlockLogTime  time.Time

	subsystemLogger btclog.Logger
	progressAction  string
	sync.Mutex
}

// NewBlockProgressLogger returns a new block progress logger.
// The progress message is templated as follows:
//  {progressAction} {numProcessed} {blocks|block} in the last {timePeriod}
//  ({numTxs}, height {lastBlockHeight}, {lastBlockTimeStamp})
func NewBlockProgressLogger(progressMessage string, logger btclog.Logger) *BlockProgressLogger {
	return &BlockProgressLogger{
		lastBlockLogTime: time.Now(),
		progressAction:   progressMessage,
		subsystemLogger:  logger,
	}
}

// LogBlockHeight logs a new block height as an information message to show
// progress to the user. In order to prevent spam, it limits logging to one
// message every 10 seconds with duration and totals included.
func (b *BlockProgressLogger) LogBlockHeight(block *btcutil.Block) {
	b.Lock()
	defer b.Unlock()

	b.receivedLogBlocks++
	b.receivedLogTx += int64(len(block.MsgBlock().Transactions))

	now := time.Now()
	duration := now.Sub(b.lastBlockLogTime)
	if duration < time.Second*10 {
		return
	}

	// Truncate the duration to 10s of milliseconds.
	durationMillis := int64(duration / time.Millisecond)
	tDuration := 10 * time.Millisecond * time.Duration(durationMillis/10)

	// Log information about new block height.
	blockStr := "blocks"
	if b.receivedLogBlocks == 1 {
		blockStr = "block"
	}
	txStr := "transactions"
	if b.receivedLogTx == 1 {
		txStr = "transaction"
	}
	b.subsystemLogger.Infof("%s %d %s in the last %s (%d %s, height %d, %s)",
		b.progressAction, b.receivedLogBlocks, blockStr, tDuration, b.receivedLogTx,
		txStr, block.Height(), block.MsgBlock().Header.Timestamp)

	b.receivedLogBlocks = 0
	b.receivedLogTx = 0
	b.lastBlockLogTime = now
}

func (b *BlockProgressLogger) SetLastLogTime(time time.Time) {
	b.lastBlockLogTime = time
}

// Log is a logger that is initialized with no output filters.  This
// means the package will not perform any logging by default until the caller
// requests it.
var Log btclog.Logger

// DisableLog disables all library log output.  Logging output is disabled
// by default until either UseLogger or SetLogWriter are called.
func DisableLog() {
	Log = btclog.Disabled
}

// UseLogger uses a specified Logger to output package logging info.
// This should be used in preference to SetLogWriter if the caller is also
// using btclog.
func UseLogger(logger btclog.Logger) {
	Log = logger
}
