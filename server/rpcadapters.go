// Copyright (c) 2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package server

import (
	"sync/atomic"

	"github.com/parallelcointeam/pod/chain"
	"github.com/parallelcointeam/pod/chaincfg/chainhash"
	"github.com/parallelcointeam/pod/mempool"
	"github.com/parallelcointeam/pod/netsync"
	"github.com/parallelcointeam/pod/peer"
	"github.com/parallelcointeam/pod/utils"
	"github.com/parallelcointeam/pod/wire"
)

// RPCPeer provides a peer for use with the RPC server and implements the
// RPCServerPeer interface.
type RPCPeer Peer

// Ensure RPCPeer implements the RPCServerPeer interface.
var _ RPCServerPeer = (*RPCPeer)(nil)

// ToPeer returns the underlying peer instance.
//
// This function is safe for concurrent access and is part of the RPCServerPeer
// interface implementation.
func (p *RPCPeer) ToPeer() *peer.Peer {
	if p == nil {
		return nil
	}
	return (*Peer)(p).Peer
}

// IsTxRelayDisabled returns whether or not the peer has disabled transaction
// relay.
//
// This function is safe for concurrent access and is part of the RPCServerPeer
// interface implementation.
func (p *RPCPeer) IsTxRelayDisabled() bool {
	return (*Peer)(p).disableRelayTx
}

// BanScore returns the current integer value that represents how close the peer
// is to being banned.
//
// This function is safe for concurrent access and is part of the RPCServerPeer
// interface implementation.
func (p *RPCPeer) BanScore() uint32 {
	return (*Peer)(p).banScore.Int()
}

// FeeFilter returns the requested current minimum fee rate for which
// transactions should be announced.
//
// This function is safe for concurrent access and is part of the RPCServerPeer
// interface implementation.
func (p *RPCPeer) FeeFilter() int64 {
	return atomic.LoadInt64(&(*Peer)(p).feeFilter)
}

// Ensure RPCConnManager implements the RPCServerConnManager interface.
var _ RPCServerConnManager = &RPCConnManager{}

// Connect adds the provided address as a new outbound peer.  The permanent flag
// indicates whether or not to make the peer persistent and reconnect if the
// connection is lost.  Attempting to connect to an already existing peer will
// return an error.
//
// This function is safe for concurrent access and is part of the
// RPCServerConnManager interface implementation.
func (cm *RPCConnManager) Connect(addr string, permanent bool) error {
	replyChan := make(chan error)
	cm.server.query <- ConnectNodeMsg{
		addr:      addr,
		permanent: permanent,
		reply:     replyChan,
	}
	return <-replyChan
}

// RemoveByID removes the peer associated with the provided id from the list of
// persistent peers.  Attempting to remove an id that does not exist will return
// an error.
//
// This function is safe for concurrent access and is part of the
// RPCServerConnManager interface implementation.
func (cm *RPCConnManager) RemoveByID(id int32) error {
	replyChan := make(chan error)
	cm.server.query <- RemoveNodeMsg{
		cmp:   func(sp *Peer) bool { return sp.ID() == id },
		reply: replyChan,
	}
	return <-replyChan
}

// RemoveByAddr removes the peer associated with the provided address from the
// list of persistent peers.  Attempting to remove an address that does not
// exist will return an error.
//
// This function is safe for concurrent access and is part of the
// RPCServerConnManager interface implementation.
func (cm *RPCConnManager) RemoveByAddr(addr string) error {
	replyChan := make(chan error)
	cm.server.query <- RemoveNodeMsg{
		cmp:   func(sp *Peer) bool { return sp.Addr() == addr },
		reply: replyChan,
	}
	return <-replyChan
}

// DisconnectByID disconnects the peer associated with the provided id.  This
// applies to both inbound and outbound peers.  Attempting to remove an id that
// does not exist will return an error.
//
// This function is safe for concurrent access and is part of the
// RPCServerConnManager interface implementation.
func (cm *RPCConnManager) DisconnectByID(id int32) error {
	replyChan := make(chan error)
	cm.server.query <- DisconnectNodeMsg{
		cmp:   func(sp *Peer) bool { return sp.ID() == id },
		reply: replyChan,
	}
	return <-replyChan
}

// DisconnectByAddr disconnects the peer associated with the provided address.
// This applies to both inbound and outbound peers.  Attempting to remove an
// address that does not exist will return an error.
//
// This function is safe for concurrent access and is part of the
// RPCServerConnManager interface implementation.
func (cm *RPCConnManager) DisconnectByAddr(addr string) error {
	replyChan := make(chan error)
	cm.server.query <- DisconnectNodeMsg{
		cmp:   func(sp *Peer) bool { return sp.Addr() == addr },
		reply: replyChan,
	}
	return <-replyChan
}

// ConnectedCount returns the number of currently connected peers.
//
// This function is safe for concurrent access and is part of the
// RPCServerConnManager interface implementation.
func (cm *RPCConnManager) ConnectedCount() int32 {
	return cm.server.ConnectedCount()
}

// NetTotals returns the sum of all bytes received and sent across the network
// for all peers.
//
// This function is safe for concurrent access and is part of the
// RPCServerConnManager interface implementation.
func (cm *RPCConnManager) NetTotals() (uint64, uint64) {
	return cm.server.NetTotals()
}

// ConnectedPeers returns an array consisting of all connected peers.
//
// This function is safe for concurrent access and is part of the
// RPCServerConnManager interface implementation.
func (cm *RPCConnManager) ConnectedPeers() []RPCServerPeer {
	replyChan := make(chan []*Peer)
	cm.server.query <- GetPeersMsg{reply: replyChan}
	serverPeers := <-replyChan

	// Convert to RPC server peers.
	peers := make([]RPCServerPeer, 0, len(serverPeers))
	for _, sp := range serverPeers {
		peers = append(peers, (*RPCPeer)(sp))
	}
	return peers
}

// PersistentPeers returns an array consisting of all the added persistent
// peers.
//
// This function is safe for concurrent access and is part of the
// RPCServerConnManager interface implementation.
func (cm *RPCConnManager) PersistentPeers() []RPCServerPeer {
	replyChan := make(chan []*Peer)
	cm.server.query <- GetAddedNodesMsg{reply: replyChan}
	serverPeers := <-replyChan

	// Convert to generic peers.
	peers := make([]RPCServerPeer, 0, len(serverPeers))
	for _, sp := range serverPeers {
		peers = append(peers, (*RPCPeer)(sp))
	}
	return peers
}

// BroadcastMessage sends the provided message to all currently connected peers.
//
// This function is safe for concurrent access and is part of the
// RPCServerConnManager interface implementation.
func (cm *RPCConnManager) BroadcastMessage(msg wire.Message) {
	cm.server.BroadcastMessage(msg)
}

// AddRebroadcastInventory adds the provided inventory to the list of
// inventories to be rebroadcast at random intervals until they show up in a
// block.
//
// This function is safe for concurrent access and is part of the
// RPCServerConnManager interface implementation.
func (cm *RPCConnManager) AddRebroadcastInventory(iv *wire.InvVect, data interface{}) {
	cm.server.AddRebroadcastInventory(iv, data)
}

// RelayTransactions generates and relays inventory vectors for all of the
// passed transactions to all connected peers.
func (cm *RPCConnManager) RelayTransactions(txns []*mempool.TxDesc) {
	cm.server.RelayTransactions(txns)
}

// rpcSyncMgr provides a block manager for use with the RPC server and
// implements the RPCServerSyncManager interface.
type rpcSyncMgr struct {
	server  *Server
	syncMgr *netsync.SyncManager
}

// Ensure rpcSyncMgr implements the RPCServerSyncManager interface.
var _ RPCServerSyncManager = (*rpcSyncMgr)(nil)

// IsCurrent returns whether or not the sync manager believes the chain is
// current as compared to the rest of the network.
//
// This function is safe for concurrent access and is part of the
// RPCServerSyncManager interface implementation.
func (b *rpcSyncMgr) IsCurrent() bool {
	return b.syncMgr.IsCurrent()
}

// SubmitBlock submits the provided block to the network after processing it
// locally.
//
// This function is safe for concurrent access and is part of the
// RPCServerSyncManager interface implementation.
func (b *rpcSyncMgr) SubmitBlock(block *utils.Block, flags blockchain.BehaviorFlags) (bool, error) {
	return b.syncMgr.ProcessBlock(block, flags)
}

// Pause pauses the sync manager until the returned channel is closed.
//
// This function is safe for concurrent access and is part of the
// RPCServerSyncManager interface implementation.
func (b *rpcSyncMgr) Pause() chan<- struct{} {
	return b.syncMgr.Pause()
}

// SyncPeerID returns the peer that is currently the peer being used to sync
// from.
//
// This function is safe for concurrent access and is part of the
// RPCServerSyncManager interface implementation.
func (b *rpcSyncMgr) SyncPeerID() int32 {
	return b.syncMgr.SyncPeerID()
}

// LocateBlocks returns the hashes of the blocks after the first known block in
// the provided locators until the provided stop hash or the current tip is
// reached, up to a max of wire.MaxBlockHeadersPerMsg hashes.
//
// This function is safe for concurrent access and is part of the
// RPCServerSyncManager interface implementation.
func (b *rpcSyncMgr) LocateHeaders(locators []*chainhash.Hash, hashStop *chainhash.Hash) []wire.BlockHeader {
	return b.server.chain.LocateHeaders(locators, hashStop)
}
