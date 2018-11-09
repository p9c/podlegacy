package server

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/parallelcointeam/pod/addrmgr"
	"github.com/parallelcointeam/pod/chain"
	"github.com/parallelcointeam/pod/chain/indexers"
	"github.com/parallelcointeam/pod/chaincfg"
	"github.com/parallelcointeam/pod/chaincfg/chainhash"
	"github.com/parallelcointeam/pod/connmgr"
	"github.com/parallelcointeam/pod/database"
	"github.com/parallelcointeam/pod/mempool"
	"github.com/parallelcointeam/pod/mining"
	"github.com/parallelcointeam/pod/mining/cpuminer"
	"github.com/parallelcointeam/pod/netsync"
	"github.com/parallelcointeam/pod/peer"
	"github.com/parallelcointeam/pod/txscript"
	"github.com/parallelcointeam/pod/upnp"
	"github.com/parallelcointeam/pod/utils"
	"github.com/parallelcointeam/pod/utils/bloom"
	"github.com/parallelcointeam/pod/wire"
)

const (
	// DefaultServices describes the default services that are supported by
	// the server.
	DefaultServices = wire.SFNodeNetwork | wire.SFNodeBloom |
		wire.SFNodeWitness | wire.SFNodeCF

	// DefaultRequiredServices describes the default services that are
	// required to be supported by outbound peers.
	DefaultRequiredServices = wire.SFNodeNetwork

	// DefaultTargetOutbound is the default number of outbound peers to target.
	DefaultTargetOutbound = 8

	// ConnectionRetryInterval is the base amount of time to wait in between
	// retries when connecting to persistent peers.  It is adjusted by the
	// number of retries such that there is a retry backoff.
	ConnectionRetryInterval = time.Second * 5
)

var (
	// UserAgentName is the user agent name and is used to help identify
	// ourselves to other bitcoin peers.
	UserAgentName = "pod"

	// UserAgentVersion is the user agent version and is used to help
	// identify ourselves to other bitcoin peers.
	UserAgentVersion = fmt.Sprintf("%d.%d.%d", AppMajor, AppMinor, AppPatch)
)

// ZeroHash is the zero value hash (all zeros).  It is defined as a convenience.
var ZeroHash chainhash.Hash

const (
	// BlockDbNamePrefix is the prefix for the block database name.  The
	// database type is appended to this value to form the full block
	// database name.
	BlockDbNamePrefix = "blocks"
)

var (
	// Cfg is
	Cfg *Config
)

// WinServiceMain is only invoked on Windows.  It detects when btcd is running
// as a service and reacts accordingly.
var WinServiceMain func() (bool, error)

// AddRebroadcastInventory is
func (s *Server) AddRebroadcastInventory(iv *wire.InvVect, data interface{}) {
	// Ignore if shutting down.
	if atomic.LoadInt32(&s.shutdown) != 0 {
		return
	}

	s.modifyRebroadcastInv <- BroadcastInventoryAdd{invVect: iv, data: data}
}

// RemoveRebroadcastInventory removes 'iv' from the list of items to be
// rebroadcasted if present.
func (s *Server) RemoveRebroadcastInventory(iv *wire.InvVect) {
	// Ignore if shutting down.
	if atomic.LoadInt32(&s.shutdown) != 0 {
		return
	}

	s.modifyRebroadcastInv <- BroadcastInventoryDel(iv)
}

// RelayTransactions generates and relays inventory vectors for all of the
// passed transactions to all connected peers.
func (s *Server) RelayTransactions(txns []*mempool.TxDesc) {
	for _, txD := range txns {
		iv := wire.NewInvVect(wire.InvTypeTx, txD.Tx.Hash())
		s.RelayInventory(iv, txD)
	}
}

// AnnounceNewTransactions generates and relays inventory vectors and notifies
// both websocket and getblocktemplate long poll clients of the passed
// transactions.  This function should be called whenever new transactions
// are added to the mempool.
func (s *Server) AnnounceNewTransactions(txns []*mempool.TxDesc) {
	// Generate and relay inventory vectors for all newly accepted
	// transactions.
	s.RelayTransactions(txns)

	// Notify both websocket and getblocktemplate long poll clients of all
	// newly accepted transactions.
	if s.RPCServer != nil {
		s.RPCServer.NotifyNewTransactions(txns)
	}
}

// TransactionConfirmed has one confirmation on the main chain. Now we can mark it as no
// longer needing rebroadcasting.
func (s *Server) TransactionConfirmed(tx *utils.Tx) {
	// Rebroadcasting is only necessary when the RPC server is active.
	if s.RPCServer == nil {
		return
	}

	iv := wire.NewInvVect(wire.InvTypeTx, tx.Hash())
	s.RemoveRebroadcastInventory(iv)
}

// PushTxMsg sends a tx message for the provided transaction hash to the
// connected peer.  An error is returned if the transaction hash is not known.
func (s *Server) PushTxMsg(sp *Peer, hash *chainhash.Hash, doneChan chan<- struct{},
	waitChan <-chan struct{}, encoding wire.MessageEncoding) error {

	// Attempt to fetch the requested transaction from the pool.  A
	// call could be made to check for existence first, but simply trying
	// to fetch a missing transaction results in the same behavior.
	tx, err := s.txMemPool.FetchTransaction(hash)
	if err != nil {
		PeerLog.Tracef("Unable to fetch tx %v from transaction "+
			"pool: %v", hash, err)

		if doneChan != nil {
			doneChan <- struct{}{}
		}
		return err
	}

	// Once we have fetched data wait for any previous operation to finish.
	if waitChan != nil {
		<-waitChan
	}

	sp.QueueMessageWithEncoding(tx.MsgTx(), doneChan, encoding)

	return nil
}

// PushBlockMsg sends a block message for the provided block hash to the
// connected peer.  An error is returned if the block hash is not known.
func (s *Server) PushBlockMsg(sp *Peer, hash *chainhash.Hash, doneChan chan<- struct{},
	waitChan <-chan struct{}, encoding wire.MessageEncoding) error {

	// Fetch the raw block bytes from the database.
	var blockBytes []byte
	err := sp.server.db.View(func(dbTx database.Tx) error {
		var err error
		blockBytes, err = dbTx.FetchBlock(hash)
		return err
	})
	if err != nil {
		PeerLog.Tracef("Unable to fetch requested block hash %v: %v",
			hash, err)

		if doneChan != nil {
			doneChan <- struct{}{}
		}
		return err
	}

	// Deserialize the block.
	var msgBlock wire.MsgBlock
	err = msgBlock.Deserialize(bytes.NewReader(blockBytes))
	if err != nil {
		PeerLog.Tracef("Unable to deserialize requested block hash "+
			"%v: %v", hash, err)

		if doneChan != nil {
			doneChan <- struct{}{}
		}
		return err
	}

	// Once we have fetched data wait for any previous operation to finish.
	if waitChan != nil {
		<-waitChan
	}

	// We only send the channel for this message if we aren't sending
	// an inv straight after.
	var dc chan<- struct{}
	continueHash := sp.continueHash
	sendInv := continueHash != nil && continueHash.IsEqual(hash)
	if !sendInv {
		dc = doneChan
	}
	sp.QueueMessageWithEncoding(&msgBlock, dc, encoding)

	// When the peer requests the final block that was advertised in
	// response to a getblocks message which requested more blocks than
	// would fit into a single message, send it a new inventory message
	// to trigger it to issue another getblocks message for the next
	// batch of inventory.
	if sendInv {
		best := sp.server.chain.BestSnapshot()
		invMsg := wire.NewMsgInvSizeHint(1)
		iv := wire.NewInvVect(wire.InvTypeBlock, &best.Hash)
		invMsg.AddInvVect(iv)
		sp.QueueMessage(invMsg, doneChan)
		sp.continueHash = nil
	}
	return nil
}

// PushMerkleBlockMsg sends a merkleblock message for the provided block hash to
// the connected peer.  Since a merkle block requires the peer to have a filter
// loaded, this call will simply be ignored if there is no filter loaded.  An
// error is returned if the block hash is not known.
func (s *Server) PushMerkleBlockMsg(sp *Peer, hash *chainhash.Hash,
	doneChan chan<- struct{}, waitChan <-chan struct{}, encoding wire.MessageEncoding) error {

	// Do not send a response if the peer doesn't have a filter loaded.
	if !sp.filter.IsLoaded() {
		if doneChan != nil {
			doneChan <- struct{}{}
		}
		return nil
	}

	// Fetch the raw block bytes from the database.
	blk, err := sp.server.chain.BlockByHash(hash)
	if err != nil {
		PeerLog.Tracef("Unable to fetch requested block hash %v: %v",
			hash, err)

		if doneChan != nil {
			doneChan <- struct{}{}
		}
		return err
	}

	// Generate a merkle block by filtering the requested block according
	// to the filter for the peer.
	merkle, matchedTxIndices := bloom.NewMerkleBlock(blk, sp.filter)

	// Once we have fetched data wait for any previous operation to finish.
	if waitChan != nil {
		<-waitChan
	}

	// Send the merkleblock.  Only send the done channel with this message
	// if no transactions will be sent afterwards.
	var dc chan<- struct{}
	if len(matchedTxIndices) == 0 {
		dc = doneChan
	}
	sp.QueueMessage(merkle, dc)

	// Finally, send any matched transactions.
	blkTransactions := blk.MsgBlock().Transactions
	for i, txIndex := range matchedTxIndices {
		// Only send the done channel on the final transaction.
		var dc chan<- struct{}
		if i == len(matchedTxIndices)-1 {
			dc = doneChan
		}
		if txIndex < uint32(len(blkTransactions)) {
			sp.QueueMessageWithEncoding(blkTransactions[txIndex], dc,
				encoding)
		}
	}

	return nil
}

// HandleUpdatePeerHeights updates the heights of all peers who were known to
// announce a block we recently accepted.
func (s *Server) HandleUpdatePeerHeights(state *PeerState, umsg UpdatePeerHeightsMsg) {
	state.ForAllPeers(func(sp *Peer) {
		// The origin peer should already have the updated height.
		if sp.Peer == umsg.originPeer {
			return
		}

		// This is a pointer to the underlying memory which doesn't
		// change.
		latestBlkHash := sp.LastAnnouncedBlock()

		// Skip this peer if it hasn't recently announced any new blocks.
		if latestBlkHash == nil {
			return
		}

		// If the peer has recently announced a block, and this block
		// matches our newly accepted block, then update their block
		// height.
		if *latestBlkHash == *umsg.newHash {
			sp.UpdateLastBlockHeight(umsg.newHeight)
			sp.UpdateLastAnnouncedBlock(nil)
		}
	})
}

// HandleAddPeerMsg deals with adding new peers.  It is invoked from the
// PeerHandler goroutine.
func (s *Server) HandleAddPeerMsg(state *PeerState, sp *Peer) bool {
	if sp == nil {
		return false
	}

	// Ignore new peers if we're shutting down.
	if atomic.LoadInt32(&s.shutdown) != 0 {
		SrvrLog.Infof("New peer %s ignored - server is shutting down", sp)
		sp.Disconnect()
		return false
	}

	// Disconnect banned peers.
	host, _, err := net.SplitHostPort(sp.Addr())
	if err != nil {
		SrvrLog.Debugf("can't split hostport %v", err)
		sp.Disconnect()
		return false
	}
	if banEnd, ok := state.banned[host]; ok {
		if time.Now().Before(banEnd) {
			SrvrLog.Debugf("Peer %s is banned for another %v - disconnecting",
				host, time.Until(banEnd))
			sp.Disconnect()
			return false
		}

		SrvrLog.Infof("Peer %s is no longer banned", host)
		delete(state.banned, host)
	}

	// TODO: Check for max peers from a single IP.

	// Limit max number of total peers.
	if state.Count() >= Cfg.MaxPeers {
		SrvrLog.Infof("Max peers reached [%d] - disconnecting peer %s",
			Cfg.MaxPeers, sp)
		sp.Disconnect()
		// TODO: how to handle permanent peers here?
		// they should be rescheduled.
		return false
	}

	// Add the new peer and start it.
	SrvrLog.Debugf("New peer %s", sp)
	if sp.Inbound() {
		state.inboundPeers[sp.ID()] = sp
	} else {
		state.outboundGroups[addrmgr.GroupKey(sp.NA())]++
		if sp.persistent {
			state.persistentPeers[sp.ID()] = sp
		} else {
			state.outboundPeers[sp.ID()] = sp
		}
	}

	return true
}

// HandleDonePeerMsg deals with peers that have signalled they are done.  It is
// invoked from the PeerHandler goroutine.
func (s *Server) HandleDonePeerMsg(state *PeerState, sp *Peer) {
	var list map[int32]*Peer
	if sp.persistent {
		list = state.persistentPeers
	} else if sp.Inbound() {
		list = state.inboundPeers
	} else {
		list = state.outboundPeers
	}
	if _, ok := list[sp.ID()]; ok {
		if !sp.Inbound() && sp.VersionKnown() {
			state.outboundGroups[addrmgr.GroupKey(sp.NA())]--
		}
		if !sp.Inbound() && sp.connReq != nil {
			s.connManager.Disconnect(sp.connReq.ID())
		}
		delete(list, sp.ID())
		SrvrLog.Debugf("Removed peer %s", sp)
		return
	}

	if sp.connReq != nil {
		s.connManager.Disconnect(sp.connReq.ID())
	}

	// Update the address' last seen time if the peer has acknowledged
	// our version and has sent us its version as well.
	if sp.VerAckReceived() && sp.VersionKnown() && sp.NA() != nil {
		s.addrManager.Connected(sp.NA())
	}

	// If we get here it means that either we didn't know about the peer
	// or we purposefully deleted it.
}

// HandleBanPeerMsg deals with banning peers.  It is invoked from the
// PeerHandler goroutine.
func (s *Server) HandleBanPeerMsg(state *PeerState, sp *Peer) {
	host, _, err := net.SplitHostPort(sp.Addr())
	if err != nil {
		SrvrLog.Debugf("can't split ban peer %s %v", sp.Addr(), err)
		return
	}
	direction := DirectionString(sp.Inbound())
	SrvrLog.Infof("Banned peer %s (%s) for %v", host, direction,
		Cfg.BanDuration)
	state.banned[host] = time.Now().Add(Cfg.BanDuration)
}

// HandleRelayInvMsg deals with relaying inventory to peers that are not already
// known to have it.  It is invoked from the PeerHandler goroutine.
func (s *Server) HandleRelayInvMsg(state *PeerState, msg RelayMsg) {
	state.ForAllPeers(func(sp *Peer) {
		if !sp.Connected() {
			return
		}

		// If the inventory is a block and the peer prefers headers,
		// generate and send a headers message instead of an inventory
		// message.
		if msg.invVect.Type == wire.InvTypeBlock && sp.WantsHeaders() {
			blockHeader, ok := msg.data.(wire.BlockHeader)
			if !ok {
				PeerLog.Warnf("Underlying data for headers" +
					" is not a block header")
				return
			}
			msgHeaders := wire.NewMsgHeaders()
			if err := msgHeaders.AddBlockHeader(&blockHeader); err != nil {
				PeerLog.Errorf("Failed to add block"+
					" header: %v", err)
				return
			}
			sp.QueueMessage(msgHeaders, nil)
			return
		}

		if msg.invVect.Type == wire.InvTypeTx {
			// Don't relay the transaction to the peer when it has
			// transaction relaying disabled.
			if sp.RelayTxDisabled() {
				return
			}

			txD, ok := msg.data.(*mempool.TxDesc)
			if !ok {
				PeerLog.Warnf("Underlying data for tx inv "+
					"relay is not a *mempool.TxDesc: %T",
					msg.data)
				return
			}

			// Don't relay the transaction if the transaction fee-per-kb
			// is less than the peer's feefilter.
			feeFilter := atomic.LoadInt64(&sp.feeFilter)
			if feeFilter > 0 && txD.FeePerKB < feeFilter {
				return
			}

			// Don't relay the transaction if there is a bloom
			// filter loaded and the transaction doesn't match it.
			if sp.filter.IsLoaded() {
				if !sp.filter.MatchTxAndUpdate(txD.Tx) {
					return
				}
			}
		}

		// Queue the inventory to be relayed with the next batch.
		// It will be ignored if the peer is already known to
		// have the inventory.
		sp.QueueInventory(msg.invVect)
	})
}

// HandleBroadcastMsg deals with broadcasting messages to peers.  It is invoked
// from the PeerHandler goroutine.
func (s *Server) HandleBroadcastMsg(state *PeerState, bmsg *BroadcastMsg) {
	state.ForAllPeers(func(sp *Peer) {
		if !sp.Connected() {
			return
		}

		for _, ep := range bmsg.excludePeers {
			if sp == ep {
				return
			}
		}

		sp.QueueMessage(bmsg.message, nil)
	})
}

// HandleQuery is the central handler for all queries and commands from other
// goroutines related to peer state.
func (s *Server) HandleQuery(state *PeerState, querymsg interface{}) {
	switch msg := querymsg.(type) {
	case GetConnCountMsg:
		nconnected := int32(0)
		state.ForAllPeers(func(sp *Peer) {
			if sp.Connected() {
				nconnected++
			}
		})
		msg.reply <- nconnected

	case GetPeersMsg:
		peers := make([]*Peer, 0, state.Count())
		state.ForAllPeers(func(sp *Peer) {
			if !sp.Connected() {
				return
			}
			peers = append(peers, sp)
		})
		msg.reply <- peers

	case ConnectNodeMsg:
		// TODO: duplicate oneshots?
		// Limit max number of total peers.
		if state.Count() >= Cfg.MaxPeers {
			msg.reply <- errors.New("max peers reached")
			return
		}
		for _, peer := range state.persistentPeers {
			if peer.Addr() == msg.addr {
				if msg.permanent {
					msg.reply <- errors.New("peer already connected")
				} else {
					msg.reply <- errors.New("peer exists as a permanent peer")
				}
				return
			}
		}

		netAddr, err := AddrStringToNetAddr(msg.addr)
		if err != nil {
			msg.reply <- err
			return
		}

		// TODO: if too many, nuke a non-perm peer.
		go s.connManager.Connect(&connmgr.ConnReq{
			Addr:      netAddr,
			Permanent: msg.permanent,
		})
		msg.reply <- nil
	case RemoveNodeMsg:
		found := DisconnectPeer(state.persistentPeers, msg.cmp, func(sp *Peer) {
			// Keep group counts ok since we remove from
			// the list now.
			state.outboundGroups[addrmgr.GroupKey(sp.NA())]--
		})

		if found {
			msg.reply <- nil
		} else {
			msg.reply <- errors.New("peer not found")
		}
	case GetOutboundGroup:
		count, ok := state.outboundGroups[msg.key]
		if ok {
			msg.reply <- count
		} else {
			msg.reply <- 0
		}
	// Request a list of the persistent (added) peers.
	case GetAddedNodesMsg:
		// Respond with a slice of the relevant peers.
		peers := make([]*Peer, 0, len(state.persistentPeers))
		for _, sp := range state.persistentPeers {
			peers = append(peers, sp)
		}
		msg.reply <- peers
	case DisconnectNodeMsg:
		// Check inbound peers. We pass a nil callback since we don't
		// require any additional actions on disconnect for inbound peers.
		found := DisconnectPeer(state.inboundPeers, msg.cmp, nil)
		if found {
			msg.reply <- nil
			return
		}

		// Check outbound peers.
		found = DisconnectPeer(state.outboundPeers, msg.cmp, func(sp *Peer) {
			// Keep group counts ok since we remove from
			// the list now.
			state.outboundGroups[addrmgr.GroupKey(sp.NA())]--
		})
		if found {
			// If there are multiple outbound connections to the same
			// ip:port, continue disconnecting them all until no such
			// peers are found.
			for found {
				found = DisconnectPeer(state.outboundPeers, msg.cmp, func(sp *Peer) {
					state.outboundGroups[addrmgr.GroupKey(sp.NA())]--
				})
			}
			msg.reply <- nil
			return
		}

		msg.reply <- errors.New("peer not found")
	}
}

// DisconnectPeer attempts to drop the connection of a targeted peer in the
// passed peer list. Targets are identified via usage of the passed
// `compareFunc`, which should return `true` if the passed peer is the target
// peer. This function returns true on success and false if the peer is unable
// to be located. If the peer is found, and the passed callback: `whenFound'
// isn't nil, we call it with the peer as the argument before it is removed
// from the peerList, and is disconnected from the server.
func DisconnectPeer(peerList map[int32]*Peer, compareFunc func(*Peer) bool, whenFound func(*Peer)) bool {
	for addr, peer := range peerList {
		if compareFunc(peer) {
			if whenFound != nil {
				whenFound(peer)
			}

			// This is ok because we are not continuing
			// to iterate so won't corrupt the loop.
			delete(peerList, addr)
			peer.Disconnect()
			return true
		}
	}
	return false
}

// NewServerPeerConfig returns the configuration for the given Peer.
func NewServerPeerConfig(sp *Peer) *peer.Config {
	return &peer.Config{
		Listeners: peer.MessageListeners{
			OnVersion:      sp.OnVersion,
			OnMemPool:      sp.OnMemPool,
			OnTx:           sp.OnTx,
			OnBlock:        sp.OnBlock,
			OnInv:          sp.OnInv,
			OnHeaders:      sp.OnHeaders,
			OnGetData:      sp.OnGetData,
			OnGetBlocks:    sp.OnGetBlocks,
			OnGetHeaders:   sp.OnGetHeaders,
			OnGetCFilters:  sp.OnGetCFilters,
			OnGetCFHeaders: sp.OnGetCFHeaders,
			OnGetCFCheckpt: sp.OnGetCFCheckpt,
			OnFeeFilter:    sp.OnFeeFilter,
			OnFilterAdd:    sp.OnFilterAdd,
			OnFilterClear:  sp.OnFilterClear,
			OnFilterLoad:   sp.OnFilterLoad,
			OnGetAddr:      sp.OnGetAddr,
			OnAddr:         sp.OnAddr,
			OnRead:         sp.OnRead,
			OnWrite:        sp.OnWrite,

			// Note: The reference client currently bans peers that send alerts
			// not signed with its key.  We could verify against their key, but
			// since the reference client is currently unwilling to support
			// other implementations' alert messages, we will not relay theirs.
			OnAlert: nil,
		},
		NewestBlock:       sp.NewestBlock,
		HostToNetAddress:  sp.server.addrManager.HostToNetAddress,
		Proxy:             Cfg.Proxy,
		UserAgentName:     UserAgentName,
		UserAgentVersion:  UserAgentVersion,
		UserAgentComments: Cfg.UserAgentComments,
		ChainParams:       sp.server.chainParams,
		Services:          sp.server.services,
		DisableRelayTx:    Cfg.BlocksOnly,
		ProtocolVersion:   peer.MaxProtocolVersion,
		TrickleInterval:   Cfg.TrickleInterval,
	}
}

// InboundPeerConnected is invoked by the connection manager when a new inbound
// connection is established.  It initializes a new inbound server peer
// instance, associates it with the connection, and starts a goroutine to wait
// for disconnection.
func (s *Server) InboundPeerConnected(conn net.Conn) {
	sp := NewServerPeer(s, false)
	sp.isWhitelisted = IsWhitelisted(conn.RemoteAddr())
	sp.Peer = peer.NewInboundPeer(NewServerPeerConfig(sp))
	sp.AssociateConnection(conn)
	go s.PeerDoneHandler(sp)
}

// OutboundPeerConnected is invoked by the connection manager when a new
// outbound connection is established.  It initializes a new outbound server
// peer instance, associates it with the relevant state such as the connection
// request instance and the connection itself, and finally notifies the address
// manager of the attempt.
func (s *Server) OutboundPeerConnected(c *connmgr.ConnReq, conn net.Conn) {
	sp := NewServerPeer(s, c.Permanent)
	p, err := peer.NewOutboundPeer(NewServerPeerConfig(sp), c.Addr.String())
	if err != nil {
		SrvrLog.Debugf("Cannot create outbound peer %s: %v", c.Addr, err)
		s.connManager.Disconnect(c.ID())
	}
	sp.Peer = p
	sp.connReq = c
	sp.isWhitelisted = IsWhitelisted(conn.RemoteAddr())
	sp.AssociateConnection(conn)
	go s.PeerDoneHandler(sp)
	s.addrManager.Attempt(sp.NA())
}

// PeerDoneHandler handles peer disconnects by notifiying the server that it's
// done along with other performing other desirable cleanup.
func (s *Server) PeerDoneHandler(sp *Peer) {
	sp.WaitForDisconnect()
	s.donePeers <- sp

	// Only tell sync manager we are gone if we ever told it we existed.
	if sp.VersionKnown() {
		s.syncManager.DonePeer(sp.Peer)

		// Evict any remaining orphans that were sent by the peer.
		numEvicted := s.txMemPool.RemoveOrphansByTag(mempool.Tag(sp.ID()))
		if numEvicted > 0 {
			TxmpLog.Debugf("Evicted %d %s from peer %v (id %d)",
				numEvicted, PickNoun(numEvicted, "orphan",
					"orphans"), sp, sp.ID())
		}
	}
	close(sp.quit)
}

// PeerHandler is used to handle peer operations such as adding and removing
// peers to and from the server, banning peers, and broadcasting messages to
// peers.  It must be run in a goroutine.
func (s *Server) PeerHandler() {
	// Start the address manager and sync manager, both of which are needed
	// by peers.  This is done here since their lifecycle is closely tied
	// to this handler and rather than adding more channels to sychronize
	// things, it's easier and slightly faster to simply start and stop them
	// in this handler.
	s.addrManager.Start()
	s.syncManager.Start()

	SrvrLog.Tracef("Starting peer handler")

	state := &PeerState{
		inboundPeers:    make(map[int32]*Peer),
		persistentPeers: make(map[int32]*Peer),
		outboundPeers:   make(map[int32]*Peer),
		banned:          make(map[string]time.Time),
		outboundGroups:  make(map[string]int),
	}

	if !Cfg.DisableDNSSeed {
		// Add peers discovered through DNS to the address manager.
		connmgr.SeedFromDNS(ActiveNetParams.Params, DefaultRequiredServices,
			PodLookup, func(addrs []*wire.NetAddress) {
				// Bitcoind uses a lookup of the dns seeder here. This
				// is rather strange since the values looked up by the
				// DNS seed lookups will vary quite a lot.
				// to replicate this behaviour we put all addresses as
				// having come from the first one.
				s.addrManager.AddAddresses(addrs, addrs[0])
			})
	}
	go s.connManager.Start()

out:
	for {
		select {
		// New peers connected to the server.
		case p := <-s.newPeers:
			s.HandleAddPeerMsg(state, p)

		// Disconnected peers.
		case p := <-s.donePeers:
			s.HandleDonePeerMsg(state, p)

		// Block accepted in mainchain or orphan, update peer height.
		case umsg := <-s.peerHeightsUpdate:
			s.HandleUpdatePeerHeights(state, umsg)

		// Peer to ban.
		case p := <-s.banPeers:
			s.HandleBanPeerMsg(state, p)

		// New inventory to potentially be relayed to other peers.
		case invMsg := <-s.relayInv:
			s.HandleRelayInvMsg(state, invMsg)

		// Message to broadcast to all connected peers except those
		// which are excluded by the message.
		case bmsg := <-s.broadcast:
			s.HandleBroadcastMsg(state, &bmsg)

		case qmsg := <-s.query:
			s.HandleQuery(state, qmsg)

		case <-s.quit:
			// Disconnect all peers on server shutdown.
			state.ForAllPeers(func(sp *Peer) {
				SrvrLog.Tracef("Shutdown peer %s", sp)
				sp.Disconnect()
			})
			break out
		}
	}

	s.connManager.Stop()
	s.syncManager.Stop()
	s.addrManager.Stop()

	// Drain channels before exiting so nothing is left waiting around
	// to send.
cleanup:
	for {
		select {
		case <-s.newPeers:
		case <-s.donePeers:
		case <-s.peerHeightsUpdate:
		case <-s.relayInv:
		case <-s.broadcast:
		case <-s.query:
		default:
			break cleanup
		}
	}
	s.wg.Done()
	SrvrLog.Tracef("Peer handler done")
}

// AddPeer adds a new peer that has already been connected to the server.
func (s *Server) AddPeer(sp *Peer) {
	s.newPeers <- sp
}

// BanPeer bans a peer that has already been connected to the server by ip.
func (s *Server) BanPeer(sp *Peer) {
	s.banPeers <- sp
}

// RelayInventory relays the passed inventory vector to all connected peers
// that are not already known to have it.
func (s *Server) RelayInventory(invVect *wire.InvVect, data interface{}) {
	s.relayInv <- RelayMsg{invVect: invVect, data: data}
}

// BroadcastMessage sends msg to all peers currently connected to the server
// except those in the passed peers to exclude.
func (s *Server) BroadcastMessage(msg wire.Message, exclPeers ...*Peer) {
	// XXX: Need to determine if this is an alert that has already been
	// broadcast and refrain from broadcasting again.
	bmsg := BroadcastMsg{message: msg, excludePeers: exclPeers}
	s.broadcast <- bmsg
}

// ConnectedCount returns the number of currently connected peers.
func (s *Server) ConnectedCount() int32 {
	replyChan := make(chan int32)

	s.query <- GetConnCountMsg{reply: replyChan}

	return <-replyChan
}

// OutboundGroupCount returns the number of peers connected to the given
// outbound group key.
func (s *Server) OutboundGroupCount(key string) int {
	replyChan := make(chan int)
	s.query <- GetOutboundGroup{key: key, reply: replyChan}
	return <-replyChan
}

// AddBytesSent adds the passed number of bytes to the total bytes sent counter
// for the server.  It is safe for concurrent access.
func (s *Server) AddBytesSent(bytesSent uint64) {
	atomic.AddUint64(&s.bytesSent, bytesSent)
}

// AddBytesReceived adds the passed number of bytes to the total bytes received
// counter for the server.  It is safe for concurrent access.
func (s *Server) AddBytesReceived(bytesReceived uint64) {
	atomic.AddUint64(&s.bytesReceived, bytesReceived)
}

// NetTotals returns the sum of all bytes received and sent across the network
// for all peers.  It is safe for concurrent access.
func (s *Server) NetTotals() (uint64, uint64) {
	return atomic.LoadUint64(&s.bytesReceived),
		atomic.LoadUint64(&s.bytesSent)
}

// UpdatePeerHeights updates the heights of all peers who have have announced
// the latest connected main chain block, or a recognized orphan. These height
// updates allow us to dynamically refresh peer heights, ensuring sync peer
// selection has access to the latest block heights for each peer.
func (s *Server) UpdatePeerHeights(latestBlkHash *chainhash.Hash, latestHeight int32, updateSource *peer.Peer) {
	s.peerHeightsUpdate <- UpdatePeerHeightsMsg{
		newHash:    latestBlkHash,
		newHeight:  latestHeight,
		originPeer: updateSource,
	}
}

// RebroadcastHandler keeps track of user submitted inventories that we have
// sent out but have not yet made it into a block. We periodically rebroadcast
// them in case our peers restarted or otherwise lost track of them.
func (s *Server) RebroadcastHandler() {
	// Wait 5 min before first tx rebroadcast.
	timer := time.NewTimer(5 * time.Minute)
	pendingInvs := make(map[wire.InvVect]interface{})

out:
	for {
		select {
		case riv := <-s.modifyRebroadcastInv:
			switch msg := riv.(type) {
			// Incoming InvVects are added to our map of RPC txs.
			case BroadcastInventoryAdd:
				pendingInvs[*msg.invVect] = msg.data

			// When an InvVect has been added to a block, we can
			// now remove it, if it was present.
			case BroadcastInventoryDel:
				if _, ok := pendingInvs[*msg]; ok {
					delete(pendingInvs, *msg)
				}
			}

		case <-timer.C:
			// Any inventory we have has not made it into a block
			// yet. We periodically resubmit them until they have.
			for iv, data := range pendingInvs {
				ivCopy := iv
				s.RelayInventory(&ivCopy, data)
			}

			// Process at a random time up to 30mins (in seconds)
			// in the future.
			timer.Reset(time.Second *
				time.Duration(RandomUint16Number(1800)))

		case <-s.quit:
			break out
		}
	}

	timer.Stop()

	// Drain channels before exiting so nothing is left waiting around
	// to send.
cleanup:
	for {
		select {
		case <-s.modifyRebroadcastInv:
		default:
			break cleanup
		}
	}
	s.wg.Done()
}

// Start begins accepting connections from peers.
func (s *Server) Start() {
	// Already started?
	if atomic.AddInt32(&s.started, 1) != 1 {
		return
	}

	SrvrLog.Trace("Starting server")

	// Server startup time. Used for the uptime command for uptime calculation.
	s.startupTime = time.Now().Unix()

	// Start the peer handler which in turn starts the address and block
	// managers.
	s.wg.Add(1)
	go s.PeerHandler()

	if s.nat != nil {
		s.wg.Add(
			1)
		go s.UPNPUpdateThread()
	}
	// 	if is
	if !Cfg.DisableRPC {
		s.wg.Add(1)

		// Start the RebroadcastHandler, which ensures user tx received by
		// the RPC server are rebroadcast until being included in a block.
		go s.RebroadcastHandler()

		s.RPCServer.Start()
	}

	// Start the CPU miner if generation is enabled.
	if Cfg.Generate {
		s.cpuMiner.Start()
	}
}

// Stop gracefully shuts down the server by stopping and disconnecting all
// peers and the main listener.
func (s *Server) Stop() error {
	// Make sure this only happens once.
	if atomic.AddInt32(&s.shutdown, 1) != 1 {
		SrvrLog.Infof("Server is already in the process of shutting down")
		return nil
	}

	SrvrLog.Warnf("Server shutting down")

	// Stop the CPU miner if needed
	s.cpuMiner.Stop()

	// Shutdown the RPC server if it's not disabled.
	if !Cfg.DisableRPC {
		s.RPCServer.Stop()
	}

	// Save fee estimator state in the database.
	s.db.Update(func(tx database.Tx) error {
		metadata := tx.Metadata()
		metadata.Put(mempool.EstimateFeeDatabaseKey, s.feeEstimator.Save())

		return nil
	})

	// Signal the remaining goroutines to quit.
	close(s.quit)
	return nil
}

// WaitForShutdown blocks until the main listener and peer handlers are stopped.
func (s *Server) WaitForShutdown() {
	s.wg.Wait()
}

// ScheduleShutdown schedules a server shutdown after the specified duration.
// It also dynamically adjusts how often to warn the server is going down based
// on remaining duration.
func (s *Server) ScheduleShutdown(duration time.Duration) {
	// Don't schedule shutdown more than once.
	if atomic.AddInt32(&s.shutdownSched, 1) != 1 {
		return
	}
	SrvrLog.Warnf("Server shutdown in %v", duration)
	go func() {
		remaining := duration
		tickDuration := DynamicTickDuration(remaining)
		done := time.After(remaining)
		ticker := time.NewTicker(tickDuration)
	out:
		for {
			select {
			case <-done:
				ticker.Stop()
				s.Stop()
				break out
			case <-ticker.C:
				remaining = remaining - tickDuration
				if remaining < time.Second {
					continue
				}

				// Change tick duration dynamically based on remaining time.
				newDuration := DynamicTickDuration(remaining)
				if tickDuration != newDuration {
					tickDuration = newDuration
					ticker.Stop()
					ticker = time.NewTicker(tickDuration)
				}
				SrvrLog.Warnf("Server shutdown in %v", remaining)
			}
		}
	}()
}

// ParseListeners determines whether each listen address is IPv4 and IPv6 and
// returns a slice of appropriate net.Addrs to listen on with TCP. It also
// properly detects addresses which apply to "all interfaces" and adds the
// address as both IPv4 and IPv6.
func ParseListeners(addrs []string) ([]net.Addr, error) {
	netAddrs := make([]net.Addr, 0, len(addrs)*2)
	for _, addr := range addrs {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			// Shouldn't happen due to already being normalized.
			return nil, err
		}

		// Empty host or host of * on plan9 is both IPv4 and IPv6.
		if host == "" || (host == "*" && runtime.GOOS == "plan9") {
			netAddrs = append(netAddrs, SimpleAddr{Net: "tcp4", Addr: addr})
			netAddrs = append(netAddrs, SimpleAddr{Net: "tcp6", Addr: addr})
			continue
		}

		// Strip IPv6 zone id if present since net.ParseIP does not
		// handle it.
		zoneIndex := strings.LastIndex(host, "%")
		if zoneIndex > 0 {
			host = host[:zoneIndex]
		}

		// Parse the IP.
		ip := net.ParseIP(host)
		if ip == nil {
			return nil, fmt.Errorf("'%s' is not a valid IP address", host)
		}

		// To4 returns nil when the IP is not an IPv4 address, so use
		// this determine the address type.
		if ip.To4() == nil {
			netAddrs = append(netAddrs, SimpleAddr{Net: "tcp6", Addr: addr})
		} else {
			netAddrs = append(netAddrs, SimpleAddr{Net: "tcp4", Addr: addr})
		}
	}
	return netAddrs, nil
}

// UPNPUpdateThread is
func (s *Server) UPNPUpdateThread() {
	// Go off immediately to prevent code duplication, thereafter we renew
	// lease every 15 minutes.
	timer := time.NewTimer(0 * time.Second)
	lport, _ := strconv.ParseInt(ActiveNetParams.DefaultPort, 10, 16)
	first := true
out:
	for {
		select {
		case <-timer.C:
			// TODO: pick external port  more cleverly
			// TODO: know which ports we are listening to on an external net.
			// TODO: if specific listen port doesn't work then ask for wildcard
			// listen port?
			// XXX this assumes timeout is in seconds.
			listenPort, err := s.nat.AddPortMapping("tcp", int(lport), int(lport),
				"btcd listen port", 20*60)
			if err != nil {
				SrvrLog.Warnf("can't add UPnP port mapping: %v", err)
			}
			if first && err == nil {
				// TODO: look this up periodically to see if upnp domain changed
				// and so did ip.
				externalip, err := s.nat.GetExternalAddress()
				if err != nil {
					SrvrLog.Warnf("UPnP can't get external address: %v", err)
					continue out
				}
				na := wire.NewNetAddressIPPort(externalip, uint16(listenPort),
					s.services)
				err = s.addrManager.AddLocalAddress(na, addrmgr.UpnpPrio)
				if err != nil {
					// XXX DeletePortMapping?
				}
				SrvrLog.Warnf("Successfully bound via UPnP to %s", addrmgr.NetAddressKey(na))
				first = false
			}
			timer.Reset(time.Minute * 15)
		case <-s.quit:
			break out
		}
	}

	timer.Stop()

	if err := s.nat.DeletePortMapping("tcp", int(lport), int(lport)); err != nil {
		SrvrLog.Warnf("unable to remove UPnP port mapping: %v", err)
	} else {
		SrvrLog.Debugf("successfully disestablished UPnP port mapping")
	}

	s.wg.Done()
}

// SetupRPCListeners returns a slice of listeners that are configured for use
// with the RPC server depending on the configuration settings for listen
// addresses and TLS.
func SetupRPCListeners() ([]net.Listener, error) {
	// Setup TLS if not disabled.
	listenFunc := net.Listen
	if !Cfg.DisableTLS {
		// Generate the TLS cert and key file if both don't already
		// exist.
		if !FileExists(Cfg.RPCKey) && !FileExists(Cfg.RPCCert) {
			err := GenCertPair(Cfg.RPCCert, Cfg.RPCKey)
			if err != nil {
				return nil, err
			}
		}
		keypair, err := tls.LoadX509KeyPair(Cfg.RPCCert, Cfg.RPCKey)
		if err != nil {
			return nil, err
		}

		tlsConfig := tls.Config{
			Certificates: []tls.Certificate{keypair},
			MinVersion:   tls.VersionTLS12,
		}

		// Change the standard net.Listen function to the tls one.
		listenFunc = func(net string, laddr string) (net.Listener, error) {
			return tls.Listen(net, laddr, &tlsConfig)
		}
	}

	netAddrs, err := ParseListeners(Cfg.RPCListeners)
	if err != nil {
		return nil, err
	}

	listeners := make([]net.Listener, 0, len(netAddrs))
	for _, addr := range netAddrs {
		listener, err := listenFunc(addr.Network(), addr.String())
		if err != nil {
			RPCsLog.Warnf("Can't listen on %s: %v", addr, err)
			continue
		}
		listeners = append(listeners, listener)
	}

	return listeners, nil
}

// New returns a new btcd server configured to listen on addr for the
// bitcoin network type specified by chainParams.  Use start to begin accepting
// connections from peers.
func New(listenAddrs []string, db database.DB, chainParams *chaincfg.Params, interrupt <-chan struct{}) (*Server, error) {
	services := DefaultServices
	if Cfg.NoPeerBloomFilters {
		services &^= wire.SFNodeBloom
	}
	SrvrLog.Tracef("Creating new address manager...")
	if Cfg.NoCFilters {
		services &^= wire.SFNodeCF
	}
	amgr := addrmgr.New(Cfg.DataDir, PodLookup)

	var listeners []net.Listener
	var nat upnp.NAT
	if !Cfg.DisableListen {
		var err error
		listeners, nat, err = InitListeners(amgr, listenAddrs, services)
		if err != nil {
			return nil, err
		}
		if len(listeners) == 0 {
			return nil, errors.New("no valid listen address")
		}
	}

	s := Server{
		chainParams:          chainParams,
		addrManager:          amgr,
		newPeers:             make(chan *Peer, Cfg.MaxPeers),
		donePeers:            make(chan *Peer, Cfg.MaxPeers),
		banPeers:             make(chan *Peer, Cfg.MaxPeers),
		query:                make(chan interface{}),
		relayInv:             make(chan RelayMsg, Cfg.MaxPeers),
		broadcast:            make(chan BroadcastMsg, Cfg.MaxPeers),
		quit:                 make(chan struct{}),
		modifyRebroadcastInv: make(chan interface{}),
		peerHeightsUpdate:    make(chan UpdatePeerHeightsMsg),
		nat:                  nat,
		db:                   db,
		timeSource:           chain.NewMedianTime(),
		services:             services,
		sigCache:             txscript.NewSigCache(Cfg.SigCacheMaxSize),
		hashCache:            txscript.NewHashCache(Cfg.SigCacheMaxSize),
		cfCheckptCaches:      make(map[wire.FilterType][]CfHeaderKV),
	}

	// Create the transaction and address indexes if needed.
	//
	// CAUTION: the txindex needs to be first in the indexes array because
	// the addrindex uses data from the txindex during catchup.  If the
	// addrindex is run first, it may not have the transactions from the
	// current block indexed.
	var indexes []indexers.Indexer
	if Cfg.TxIndex || Cfg.AddrIndex {
		// Enable transaction index if address index is enabled since it
		// requires it.
		if !Cfg.TxIndex {
			IndxLog.Infof("Transaction index enabled because it " +
				"is required by the address index")
			Cfg.TxIndex = true
		} else {
			IndxLog.Info("Transaction index is enabled")
		}

		s.txIndex = indexers.NewTxIndex(db)
		indexes = append(indexes, s.txIndex)
	}
	if Cfg.AddrIndex {
		IndxLog.Info("Address index is enabled")
		s.addrIndex = indexers.NewAddrIndex(db, chainParams)
		indexes = append(indexes, s.addrIndex)
	}
	if !Cfg.NoCFilters {
		IndxLog.Info("Committed filter index is enabled")
		s.cfIndex = indexers.NewCfIndex(db, chainParams)
		indexes = append(indexes, s.cfIndex)
	}

	// Create an index manager if any of the optional indexes are enabled.
	var indexManager chain.IndexManager
	if len(indexes) > 0 {
		indexManager = indexers.NewManager(db, indexes)
	}

	// Merge given checkpoints with the default ones unless they are disabled.
	var checkpoints []chaincfg.Checkpoint
	if !Cfg.DisableCheckpoints {
		checkpoints = MergeCheckpoints(s.chainParams.Checkpoints, Cfg.addCheckpoints)
	}

	// Create a new block chain instance with the appropriate configuration.
	var err error
	s.chain, err = chain.New(&chain.Config{
		DB:           s.db,
		Interrupt:    interrupt,
		ChainParams:  s.chainParams,
		Checkpoints:  checkpoints,
		TimeSource:   s.timeSource,
		SigCache:     s.sigCache,
		IndexManager: indexManager,
		HashCache:    s.hashCache,
	})
	if err != nil {
		return nil, err
	}

	// Search for a FeeEstimator state in the database. If none can be found
	// or if it cannot be loaded, create a new one.
	db.Update(func(tx database.Tx) error {
		metadata := tx.Metadata()
		feeEstimationData := metadata.Get(mempool.EstimateFeeDatabaseKey)
		if feeEstimationData != nil {
			// delete it from the database so that we don't try to restore the
			// same thing again somehow.
			metadata.Delete(mempool.EstimateFeeDatabaseKey)

			// If there is an error, log it and make a new fee estimator.
			var err error
			s.feeEstimator, err = mempool.RestoreFeeEstimator(feeEstimationData)

			if err != nil {
				PeerLog.Errorf("Failed to restore fee estimator %v", err)
			}
		}

		return nil
	})

	// If no feeEstimator has been found, or if the one that has been found
	// is behind somehow, create a new one and start over.
	if s.feeEstimator == nil || s.feeEstimator.LastKnownHeight() != s.chain.BestSnapshot().Height {
		s.feeEstimator = mempool.NewFeeEstimator(
			mempool.DefaultEstimateFeeMaxRollback,
			mempool.DefaultEstimateFeeMinRegisteredBlocks)
	}

	txC := mempool.Config{
		Policy: mempool.Policy{
			DisableRelayPriority: Cfg.NoRelayPriority,
			AcceptNonStd:         Cfg.RelayNonStd,
			FreeTxRelayLimit:     Cfg.FreeTxRelayLimit,
			MaxOrphanTxs:         Cfg.MaxOrphanTxs,
			MaxOrphanTxSize:      defaultMaxOrphanTxSize,
			MaxSigOpCostPerTx:    chain.MaxBlockSigOpsCost / 4,
			MinRelayTxFee:        Cfg.minRelayTxFee,
			MaxTxVersion:         2,
		},
		ChainParams:    chainParams,
		FetchUtxoView:  s.chain.FetchUtxoView,
		BestHeight:     func() int32 { return s.chain.BestSnapshot().Height },
		MedianTimePast: func() time.Time { return s.chain.BestSnapshot().MedianTime },
		CalcSequenceLock: func(tx *utils.Tx, view *chain.UtxoViewpoint) (*chain.SequenceLock, error) {
			return s.chain.CalcSequenceLock(tx, view, true)
		},
		IsDeploymentActive: s.chain.IsDeploymentActive,
		SigCache:           s.sigCache,
		HashCache:          s.hashCache,
		AddrIndex:          s.addrIndex,
		FeeEstimator:       s.feeEstimator,
	}
	s.txMemPool = mempool.New(&txC)

	s.syncManager, err = netsync.New(&netsync.Config{
		PeerNotifier:       &s,
		Chain:              s.chain,
		TxMemPool:          s.txMemPool,
		ChainParams:        s.chainParams,
		DisableCheckpoints: Cfg.DisableCheckpoints,
		MaxPeers:           Cfg.MaxPeers,
		FeeEstimator:       s.feeEstimator,
	})
	if err != nil {
		return nil, err
	}

	// Create the mining policy and block template generator based on the
	// configuration options.
	//
	// NOTE: The CPU miner relies on the mempool, so the mempool has to be
	// created before calling the function to create the CPU miner.
	policy := mining.Policy{
		BlockMinWeight:    Cfg.BlockMinWeight,
		BlockMaxWeight:    Cfg.BlockMaxWeight,
		BlockMinSize:      Cfg.BlockMinSize,
		BlockMaxSize:      Cfg.BlockMaxSize,
		BlockPrioritySize: Cfg.BlockPrioritySize,
		TxMinFreeFee:      Cfg.minRelayTxFee,
	}
	blockTemplateGenerator := mining.NewBlkTmplGenerator(&policy,
		s.chainParams, s.txMemPool, s.chain, s.timeSource,
		s.sigCache, s.hashCache)
	s.cpuMiner = cpuminer.New(&cpuminer.Config{
		ChainParams:            chainParams,
		BlockTemplateGenerator: blockTemplateGenerator,
		MiningAddrs:            Cfg.miningAddrs,
		ProcessBlock:           s.syncManager.ProcessBlock,
		ConnectedCount:         s.ConnectedCount,
		IsCurrent:              s.syncManager.IsCurrent,
	})

	// Only setup a function to return new addresses to connect to when
	// not running in connect-only mode.  The simulation network is always
	// in connect-only mode since it is only intended to connect to
	// specified peers and actively avoid advertising and connecting to
	// discovered peers in order to prevent it from becoming a public test
	// network.
	var newAddressFunc func() (net.Addr, error)
	if !Cfg.SimNet && len(Cfg.ConnectPeers) == 0 {
		newAddressFunc = func() (net.Addr, error) {
			for tries := 0; tries < 100; tries++ {
				addr := s.addrManager.GetAddress()
				if addr == nil {
					break
				}

				// Address will not be invalid, local or unroutable
				// because addrmanager rejects those on addition.
				// Just check that we don't already have an address
				// in the same group so that we are not connecting
				// to the same network segment at the expense of
				// others.
				key := addrmgr.GroupKey(addr.NetAddress())
				if s.OutboundGroupCount(key) != 0 {
					continue
				}

				// only allow recent nodes (10mins) after we failed 30
				// times
				if tries < 30 && time.Since(addr.LastAttempt()) < 10*time.Minute {
					continue
				}

				// allow nondefault ports after 50 failed tries.
				if tries < 50 && fmt.Sprintf("%d", addr.NetAddress().Port) !=
					ActiveNetParams.DefaultPort {
					continue
				}

				addrString := addrmgr.NetAddressKey(addr.NetAddress())
				return AddrStringToNetAddr(addrString)
			}

			return nil, errors.New("no valid connect address")
		}
	}

	// Create a connection manager.
	targetOutbound := DefaultTargetOutbound
	if Cfg.MaxPeers < targetOutbound {
		targetOutbound = Cfg.MaxPeers
	}
	cmgr, err := connmgr.New(&connmgr.Config{
		Listeners:      listeners,
		OnAccept:       s.InboundPeerConnected,
		RetryDuration:  ConnectionRetryInterval,
		TargetOutbound: uint32(targetOutbound),
		Dial:           Dial,
		OnConnection:   s.OutboundPeerConnected,
		GetNewAddress:  newAddressFunc,
	})
	if err != nil {
		return nil, err
	}
	s.connManager = cmgr

	// Start up persistent peers.
	permanentPeers := Cfg.ConnectPeers
	if len(permanentPeers) == 0 {
		permanentPeers = Cfg.AddPeers
	}
	for _, addr := range permanentPeers {
		netAddr, err := AddrStringToNetAddr(addr)
		if err != nil {
			return nil, err
		}

		go s.connManager.Connect(&connmgr.ConnReq{
			Addr:      netAddr,
			Permanent: true,
		})
	}

	if !Cfg.DisableRPC {
		// Setup listeners for the configured RPC listen addresses and
		// TLS settings.
		rpcListeners, err := SetupRPCListeners()
		if err != nil {
			return nil, err
		}
		if len(rpcListeners) == 0 {
			return nil, errors.New("RPCS: No valid listen address")
		}

		s.RPCServer, err = NewRPCServer(&RPCServerConfig{
			Listeners:    rpcListeners,
			StartupTime:  s.startupTime,
			ConnMgr:      &RPCConnManager{&s},
			SyncMgr:      &rpcSyncMgr{&s, s.syncManager},
			TimeSource:   s.timeSource,
			Chain:        s.chain,
			ChainParams:  chainParams,
			DB:           db,
			TxMemPool:    s.txMemPool,
			Generator:    blockTemplateGenerator,
			CPUMiner:     s.cpuMiner,
			TxIndex:      s.txIndex,
			AddrIndex:    s.addrIndex,
			CfIndex:      s.cfIndex,
			FeeEstimator: s.feeEstimator,
		})
		if err != nil {
			return nil, err
		}

		// Signal process shutdown when the RPC server requests it.
		go func() {
			<-s.RPCServer.RequestedProcessShutdown()
			ShutdownRequestChannel <- struct{}{}
		}()
	}

	return &s, nil
}

// InitListeners initializes the configured net listeners and adds any bound
// addresses to the address manager. Returns the listeners and a NAT interface,
// which is non-nil if UPnP is in use.
func InitListeners(amgr *addrmgr.AddrManager, listenAddrs []string, services wire.ServiceFlag) ([]net.Listener, upnp.NAT, error) {
	// Listen for TCP connections at the configured addresses
	netAddrs, err := ParseListeners(listenAddrs)
	if err != nil {
		return nil, nil, err
	}

	listeners := make([]net.Listener, 0, len(netAddrs))
	for _, addr := range netAddrs {
		listener, err := net.Listen(addr.Network(), addr.String())
		if err != nil {
			SrvrLog.Warnf("Can't listen on %s: %v", addr, err)
			continue
		}
		listeners = append(listeners, listener)
	}

	var nat upnp.NAT
	if len(Cfg.ExternalIPs) != 0 {
		defaultPort, err := strconv.ParseUint(ActiveNetParams.DefaultPort, 10, 16)
		if err != nil {
			SrvrLog.Errorf("Can not parse default port %s for active chain: %v",
				ActiveNetParams.DefaultPort, err)
			return nil, nil, err
		}

		for _, sip := range Cfg.ExternalIPs {
			eport := uint16(defaultPort)
			host, portstr, err := net.SplitHostPort(sip)
			if err != nil {
				// no port, use default.
				host = sip
			} else {
				port, err := strconv.ParseUint(portstr, 10, 16)
				if err != nil {
					SrvrLog.Warnf("Can not parse port from %s for "+
						"externalip: %v", sip, err)
					continue
				}
				eport = uint16(port)
			}
			na, err := amgr.HostToNetAddress(host, eport, services)
			if err != nil {
				SrvrLog.Warnf("Not adding %s as externalip: %v", sip, err)
				continue
			}

			err = amgr.AddLocalAddress(na, addrmgr.ManualPrio)
			if err != nil {
				AmgrLog.Warnf("Skipping specified external IP: %v", err)
			}
		}
	} else {
		if Cfg.Upnp {
			var err error
			nat, err = upnp.Discover()
			if err != nil {
				SrvrLog.Warnf("Can't discover upnp: %v", err)
			}
			// nil nat here is fine, just means no upnp on network.
		}

		// Add bound addresses to address manager to be advertised to peers.
		for _, listener := range listeners {
			addr := listener.Addr().String()
			err := AddLocalAddress(amgr, addr, services)
			if err != nil {
				AmgrLog.Warnf("Skipping bound address %s: %v", addr, err)
			}
		}
	}

	return listeners, nat, nil
}

// AddrStringToNetAddr takes an address in the form of 'host:port' and returns
// a net.Addr which maps to the original address with any host names resolved
// to IP addresses.  It also handles tor addresses properly by returning a
// net.Addr that encapsulates the address.
func AddrStringToNetAddr(addr string) (net.Addr, error) {
	host, strPort, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	port, err := strconv.Atoi(strPort)
	if err != nil {
		return nil, err
	}

	// Skip if host is already an IP address.
	if ip := net.ParseIP(host); ip != nil {
		return &net.TCPAddr{
			IP:   ip,
			Port: port,
		}, nil
	}

	// Tor addresses cannot be resolved to an IP, so just return an onion
	// address instead.
	if strings.HasSuffix(host, ".onion") {
		if Cfg.NoOnion {
			return nil, errors.New("tor has been disabled")
		}

		return &OnionAddr{addr: addr}, nil
	}

	// Attempt to look up an IP address associated with the parsed host.
	ips, err := PodLookup(host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("no addresses found for %s", host)
	}

	return &net.TCPAddr{
		IP:   ips[0],
		Port: port,
	}, nil
}

// AddLocalAddress adds an address that this node is listening on to the
// address manager so that it may be relayed to peers.
func AddLocalAddress(addrMgr *addrmgr.AddrManager, addr string, services wire.ServiceFlag) error {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return err
	}

	if ip := net.ParseIP(host); ip != nil && ip.IsUnspecified() {
		// If bound to unspecified address, advertise all local interfaces
		addrs, err := net.InterfaceAddrs()
		if err != nil {
			return err
		}

		for _, addr := range addrs {
			ifaceIP, _, err := net.ParseCIDR(addr.String())
			if err != nil {
				continue
			}

			// If bound to 0.0.0.0, do not add IPv6 interfaces and if bound to
			// ::, do not add IPv4 interfaces.
			if (ip.To4() == nil) != (ifaceIP.To4() == nil) {
				continue
			}

			netAddr := wire.NewNetAddressIPPort(ifaceIP, uint16(port), services)
			addrMgr.AddLocalAddress(netAddr, addrmgr.BoundPrio)
		}
	} else {
		netAddr, err := addrMgr.HostToNetAddress(host, uint16(port), services)
		if err != nil {
			return err
		}

		addrMgr.AddLocalAddress(netAddr, addrmgr.BoundPrio)
	}

	return nil
}

// DynamicTickDuration is a convenience function used to dynamically choose a
// tick duration based on remaining time.  It is primarily used during
// server shutdown to make shutdown warnings more frequent as the shutdown time
// approaches.
func DynamicTickDuration(remaining time.Duration) time.Duration {
	switch {
	case remaining <= time.Second*5:
		return time.Second
	case remaining <= time.Second*15:
		return time.Second * 5
	case remaining <= time.Minute:
		return time.Second * 15
	case remaining <= time.Minute*5:
		return time.Minute
	case remaining <= time.Minute*15:
		return time.Minute * 5
	case remaining <= time.Hour:
		return time.Minute * 15
	}
	return time.Hour
}

// IsWhitelisted returns whether the IP address is included in the whitelisted
// networks and IPs.
func IsWhitelisted(addr net.Addr) bool {
	if len(Cfg.whitelists) == 0 {
		return false
	}

	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		SrvrLog.Warnf("Unable to SplitHostPort on '%s': %v", addr, err)
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil {
		SrvrLog.Warnf("Unable to parse IP '%s'", addr)
		return false
	}

	for _, ipnet := range Cfg.whitelists {
		if ipnet.Contains(ip) {
			return true
		}
	}
	return false
}

// CheckpointSorter implements sort.Interface to allow a slice of checkpoints to
// be sorted.
type CheckpointSorter []chaincfg.Checkpoint

// Len returns the number of checkpoints in the slice.  It is part of the
// sort.Interface implementation.
func (s CheckpointSorter) Len() int {
	return len(s)
}

// Swap swaps the checkpoints at the passed indices.  It is part of the
// sort.Interface implementation.
func (s CheckpointSorter) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// Less returns whether the checkpoint with index i should sort before the
// checkpoint with index j.  It is part of the sort.Interface implementation.
func (s CheckpointSorter) Less(i, j int) bool {
	return s[i].Height < s[j].Height
}

// MergeCheckpoints returns two slices of checkpoints merged into one slice
// such that the checkpoints are sorted by height.  In the case the additional
// checkpoints contain a checkpoint with the same height as a checkpoint in the
// default checkpoints, the additional checkpoint will take precedence and
// overwrite the default one.
func MergeCheckpoints(defaultCheckpoints, additional []chaincfg.Checkpoint) []chaincfg.Checkpoint {
	// Create a map of the additional checkpoints to remove duplicates while
	// leaving the most recently-specified checkpoint.
	extra := make(map[int32]chaincfg.Checkpoint)
	for _, checkpoint := range additional {
		extra[checkpoint.Height] = checkpoint
	}

	// Add all default checkpoints that do not have an override in the
	// additional checkpoints.
	numDefault := len(defaultCheckpoints)
	checkpoints := make([]chaincfg.Checkpoint, 0, numDefault+len(extra))
	for _, checkpoint := range defaultCheckpoints {
		if _, exists := extra[checkpoint.Height]; !exists {
			checkpoints = append(checkpoints, checkpoint)
		}
	}

	// Append the additional checkpoints and return the sorted results.
	for _, checkpoint := range extra {
		checkpoints = append(checkpoints, checkpoint)
	}
	sort.Sort(CheckpointSorter(checkpoints))
	return checkpoints
}

// Count returns the count of all known peers.
func (ps *PeerState) Count() int {
	return len(ps.inboundPeers) + len(ps.outboundPeers) +
		len(ps.persistentPeers)
}

// ForAllOutboundPeers is a helper function that runs closure on all outbound
// peers known to PeerState.
func (ps *PeerState) ForAllOutboundPeers(closure func(sp *Peer)) {
	for _, e := range ps.outboundPeers {
		closure(e)
	}
	for _, e := range ps.persistentPeers {
		closure(e)
	}
}

// ForAllPeers is a helper function that runs closure on all peers known to
// PeerState.
func (ps *PeerState) ForAllPeers(closure func(sp *Peer)) {
	for _, e := range ps.inboundPeers {
		closure(e)
	}
	ps.ForAllOutboundPeers(closure)
}
