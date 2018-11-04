package main

import (
	"container/list"
	"sync/atomic"

	"github.com/parallelcointeam/btcutil"
	"github.com/parallelcointeam/pod/blockchain"
	"github.com/parallelcointeam/pod/chaincfg/chainhash"
	"github.com/parallelcointeam/pod/netsync"
	peerpkg "github.com/parallelcointeam/pod/peer"
	"github.com/parallelcointeam/pod/wire"
)

// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

// NewPeer informs the sync manager of a newly active peer.
func (sm *SyncManager) NewPeer(peer *peerpkg.Peer) {
	// Ignore if we are shutting down.
	if atomic.LoadInt32(&sm.shutdown) != 0 {
		return
	}
	sm.msgChan <- &newPeerMsg{peer: peer}
}

// QueueTx adds the passed transaction message and peer to the block handling
// queue. Responds to the done channel argument after the tx message is
// processed.
func (sm *SyncManager) QueueTx(tx *btcutil.Tx, peer *peerpkg.Peer, done chan struct{}) {
	// Don't accept more transactions if we're shutting down.
	if atomic.LoadInt32(&sm.shutdown) != 0 {
		done <- struct{}{}
		return
	}

	sm.msgChan <- &txMsg{tx: tx, peer: peer, reply: done}
}

// QueueBlock adds the passed block message and peer to the block handling
// queue. Responds to the done channel argument after the block message is
// processed.
func (sm *SyncManager) QueueBlock(block *btcutil.Block, peer *peerpkg.Peer, done chan struct{}) {
	// Don't accept more blocks if we're shutting down.
	if atomic.LoadInt32(&sm.shutdown) != 0 {
		done <- struct{}{}
		return
	}

	sm.msgChan <- &blockMsg{block: block, peer: peer, reply: done}
}

// QueueInv adds the passed inv message and peer to the block handling queue.
func (sm *SyncManager) QueueInv(inv *wire.MsgInv, peer *peerpkg.Peer) {
	// No channel handling here because peers do not need to block on inv
	// messages.
	if atomic.LoadInt32(&sm.shutdown) != 0 {
		return
	}

	sm.msgChan <- &invMsg{inv: inv, peer: peer}
}

// QueueHeaders adds the passed headers message and peer to the block handling
// queue.
func (sm *SyncManager) QueueHeaders(headers *wire.MsgHeaders, peer *peerpkg.Peer) {
	// No channel handling here because peers do not need to block on
	// headers messages.
	if atomic.LoadInt32(&sm.shutdown) != 0 {
		return
	}

	sm.msgChan <- &headersMsg{headers: headers, peer: peer}
}

// DonePeer informs the blockmanager that a peer has disconnected.
func (sm *SyncManager) DonePeer(peer *peerpkg.Peer) {
	// Ignore if we are shutting down.
	if atomic.LoadInt32(&sm.shutdown) != 0 {
		return
	}

	sm.msgChan <- &donePeerMsg{peer: peer}
}

// Start begins the core block handler which processes block and inv messages.
func (sm *SyncManager) Start() {
	// Already started?
	if atomic.AddInt32(&sm.started, 1) != 1 {
		return
	}

	log.Trace("Starting sync manager")
	sm.wg.Add(1)
	go sm.blockHandler()
}

// Stop gracefully shuts down the sync manager by stopping all asynchronous
// handlers and waiting for them to finish.
func (sm *SyncManager) Stop() error {
	if atomic.AddInt32(&sm.shutdown, 1) != 1 {
		log.Warnf("Sync manager is already in the process of " +
			"shutting down")
		return nil
	}

	log.Infof("Sync manager shutting down")
	close(sm.quit)
	sm.wg.Wait()
	return nil
}

// SyncPeerID returns the ID of the current sync peer, or 0 if there is none.
func (sm *SyncManager) SyncPeerID() int32 {
	reply := make(chan int32)
	sm.msgChan <- getSyncPeerMsg{reply: reply}
	return <-reply
}

// ProcessBlock makes use of ProcessBlock on an internal instance of a block
// chain.
func (sm *SyncManager) ProcessBlock(block *btcutil.Block, flags blockchain.BehaviorFlags) (bool, error) {
	reply := make(chan processBlockResponse, 1)
	sm.msgChan <- processBlockMsg{block: block, flags: flags, reply: reply}
	response := <-reply
	return response.isOrphan, response.err
}

// IsCurrent returns whether or not the sync manager believes it is synced with
// the connected peers.
func (sm *SyncManager) IsCurrent() bool {
	reply := make(chan bool)
	sm.msgChan <- isCurrentMsg{reply: reply}
	return <-reply
}

// Pause pauses the sync manager until the returned channel is closed.
//
// Note that while paused, all peer and block processing is halted.  The
// message sender should avoid pausing the sync manager for long durations.
func (sm *SyncManager) Pause() chan<- struct{} {
	c := make(chan struct{})
	sm.msgChan <- pauseMsg{c}
	return c
}

// New constructs a new SyncManager. Use Start to begin processing asynchronous
// block, tx, and inv updates.
func New(config *Config) (*SyncManager, error) {
	sm := SyncManager{
		peerNotifier:    config.PeerNotifier,
		chain:           config.Chain,
		txMemPool:       config.TxMemPool,
		chainParams:     config.ChainParams,
		rejectedTxns:    make(map[chainhash.Hash]struct{}),
		requestedTxns:   make(map[chainhash.Hash]struct{}),
		requestedBlocks: make(map[chainhash.Hash]struct{}),
		peerStates:      make(map[*peerpkg.Peer]*peerSyncState),
		progressLogger:  netsync.NewBlockProgressLogger("Processed", log),
		msgChan:         make(chan interface{}, config.MaxPeers*3),
		headerList:      list.New(),
		quit:            make(chan struct{}),
		feeEstimator:    config.FeeEstimator,
	}

	best := sm.chain.BestSnapshot()
	if !config.DisableCheckpoints {
		// Initialize the next checkpoint based on the current height.
		sm.nextCheckpoint = sm.findNextHeaderCheckpoint(best.Height)
		if sm.nextCheckpoint != nil {
			sm.resetHeaderState(&best.Hash, best.Height)
		}
	} else {
		log.Info("Checkpoints are disabled")
	}

	sm.chain.Subscribe(sm.handleBlockchainNotification)

	return &sm, nil
}
