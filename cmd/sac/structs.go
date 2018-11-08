package main

import (
	"crypto/sha256"
	"net"
	"sync"
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

// Server provides a bitcoin server for handling communications to and from
// bitcoin peers.
type Server struct {
	// The following variables must only be used atomically.
	// Putting the uint64s first makes them 64-bit aligned for 32-bit systems.
	bytesReceived uint64 // Total bytes received from all peers since start.
	bytesSent     uint64 // Total bytes sent by all peers since start.
	started       int32
	shutdown      int32
	shutdownSched int32
	startupTime   int64

	chainParams          *chaincfg.Params
	addrManager          *addrmgr.AddrManager
	connManager          *connmgr.ConnManager
	sigCache             *txscript.SigCache
	hashCache            *txscript.HashCache
	RPCServer            *RPCServer
	syncManager          *netsync.SyncManager
	chain                *blockchain.BlockChain
	txMemPool            *mempool.TxPool
	cpuMiner             *cpuminer.CPUMiner
	modifyRebroadcastInv chan interface{}
	newPeers             chan *Peer
	donePeers            chan *Peer
	banPeers             chan *Peer
	query                chan interface{}
	relayInv             chan RelayMsg
	broadcast            chan BroadcastMsg
	peerHeightsUpdate    chan UpdatePeerHeightsMsg
	wg                   sync.WaitGroup
	quit                 chan struct{}
	nat                  upnp.NAT
	db                   database.DB
	timeSource           blockchain.MedianTimeSource
	services             wire.ServiceFlag

	// The following fields are used for optional indexes.  They will be nil
	// if the associated index is not enabled.  These fields are set during
	// initial creation of the server and never changed afterwards, so they
	// do not need to be protected for concurrent access.
	txIndex   *indexers.TxIndex
	addrIndex *indexers.AddrIndex
	cfIndex   *indexers.CfIndex

	// The fee estimator keeps track of how long transactions are left in
	// the mempool before they are mined into blocks.
	feeEstimator *mempool.FeeEstimator

	// cfCheckptCaches stores a cached slice of filter headers for cfcheckpt
	// messages for each filter type.
	cfCheckptCaches    map[wire.FilterType][]CfHeaderKV
	cfCheckptCachesMtx sync.RWMutex
}

// Peer extends the peer to maintain state shared by the server and
// the blockmanager.
type Peer struct {
	// The following variables must only be used atomically
	feeFilter int64

	*peer.Peer

	connReq        *connmgr.ConnReq
	server         *Server
	persistent     bool
	continueHash   *chainhash.Hash
	relayMtx       sync.Mutex
	disableRelayTx bool
	sentAddrs      bool
	isWhitelisted  bool
	filter         *bloom.Filter
	knownAddresses map[string]struct{}
	banScore       connmgr.DynamicBanScore
	quit           chan struct{}
	// The following chans are used to sync blockmanager and server.
	txProcessed    chan struct{}
	blockProcessed chan struct{}
}

// RPCConnManager provides a connection manager for use with the RPC server and
// implements the RPCServerConnManager interface.
type RPCConnManager struct {
	server *Server
}

// RPCServer provides a concurrent safe RPC server to a chain server.
type RPCServer struct {
	started                int32
	shutdown               int32
	cfg                    RPCServerConfig
	authsha                [sha256.Size]byte
	limitauthsha           [sha256.Size]byte
	ntfnMgr                *WsNotificationManager
	numClients             int32
	statusLines            map[int]string
	statusLock             sync.RWMutex
	wg                     sync.WaitGroup
	GbtWorkState           *GbtWorkState
	HelpCacher             *HelpCacher
	requestProcessShutdown chan struct{}
	quit                   chan int
}

// RPCServerSyncManager represents a sync manager for use with the RPC server.
//
// The interface contract requires that all of these methods are safe for
// concurrent access.
type RPCServerSyncManager interface {
	// IsCurrent returns whether or not the sync manager believes the chain
	// is current as compared to the rest of the network.
	IsCurrent() bool

	// SubmitBlock submits the provided block to the network after
	// processing it locally.
	SubmitBlock(block *utils.Block, flags blockchain.BehaviorFlags) (bool, error)

	// Pause pauses the sync manager until the returned channel is closed.
	Pause() chan<- struct{}

	// SyncPeerID returns the ID of the peer that is currently the peer being
	// used to sync from or 0 if there is none.
	SyncPeerID() int32

	// LocateHeaders returns the headers of the blocks after the first known
	// block in the provided locators until the provided stop hash or the
	// current tip is reached, up to a max of wire.MaxBlockHeadersPerMsg
	// hashes.
	LocateHeaders(locators []*chainhash.Hash, hashStop *chainhash.Hash) []wire.BlockHeader
}

// RPCServerConfig is a descriptor containing the RPC server configuration.
type RPCServerConfig struct {
	// Listeners defines a slice of listeners for which the RPC server will
	// take ownership of and accept connections.  Since the RPC server takes
	// ownership of these listeners, they will be closed when the RPC server
	// is stopped.
	Listeners []net.Listener

	// StartupTime is the unix timestamp for when the server that is hosting
	// the RPC server started.
	StartupTime int64

	// ConnMgr defines the connection manager for the RPC server to use.  It
	// provides the RPC server with a means to do things such as add,
	// remove, connect, disconnect, and query peers as well as other
	// connection-related data and tasks.
	ConnMgr RPCServerConnManager

	// SyncMgr defines the sync manager for the RPC server to use.
	SyncMgr RPCServerSyncManager

	// These fields allow the RPC server to interface with the local block
	// chain data and state.
	TimeSource  blockchain.MedianTimeSource
	Chain       *blockchain.BlockChain
	ChainParams *chaincfg.Params
	DB          database.DB

	// TxMemPool defines the transaction memory pool to interact with.
	TxMemPool *mempool.TxPool

	// These fields allow the RPC server to interface with mining.
	//
	// Generator produces block templates and the CPUMiner solves them using
	// the CPU.  CPU mining is typically only useful for test purposes when
	// doing regression or simulation testing.
	Generator *mining.BlkTmplGenerator
	CPUMiner  *cpuminer.CPUMiner

	// These fields define any optional indexes the RPC server can make use
	// of to provide additional data when queried.
	TxIndex   *indexers.TxIndex
	AddrIndex *indexers.AddrIndex
	CfIndex   *indexers.CfIndex

	// The fee estimator keeps track of how long transactions are left in
	// the mempool before they are mined into blocks.
	FeeEstimator *mempool.FeeEstimator
}

// RPCServerPeer represents a peer for use with the RPC server.
//
// The interface contract requires that all of these methods are safe for
// concurrent access.
type RPCServerPeer interface {
	// ToPeer returns the underlying peer instance.
	ToPeer() *peer.Peer

	// IsTxRelayDisabled returns whether or not the peer has disabled
	// transaction relay.
	IsTxRelayDisabled() bool

	// BanScore returns the current integer value that represents how close
	// the peer is to being banned.
	BanScore() uint32

	// FeeFilter returns the requested current minimum fee rate for which
	// transactions should be announced.
	FeeFilter() int64
}

// RPCServerConnManager represents a connection manager for use with the RPC
// server.
//
// The interface contract requires that all of these methods are safe for
// concurrent access.
type RPCServerConnManager interface {
	// Connect adds the provided address as a new outbound peer.  The
	// permanent flag indicates whether or not to make the peer persistent
	// and reconnect if the connection is lost.  Attempting to connect to an
	// already existing peer will return an error.
	Connect(addr string, permanent bool) error

	// RemoveByID removes the peer associated with the provided id from the
	// list of persistent peers.  Attempting to remove an id that does not
	// exist will return an error.
	RemoveByID(id int32) error

	// RemoveByAddr removes the peer associated with the provided address
	// from the list of persistent peers.  Attempting to remove an address
	// that does not exist will return an error.
	RemoveByAddr(addr string) error

	// DisconnectByID disconnects the peer associated with the provided id.
	// This applies to both inbound and outbound peers.  Attempting to
	// remove an id that does not exist will return an error.
	DisconnectByID(id int32) error

	// DisconnectByAddr disconnects the peer associated with the provided
	// address.  This applies to both inbound and outbound peers.
	// Attempting to remove an address that does not exist will return an
	// error.
	DisconnectByAddr(addr string) error

	// ConnectedCount returns the number of currently connected peers.
	ConnectedCount() int32

	// NetTotals returns the sum of all bytes received and sent across the
	// network for all peers.
	NetTotals() (uint64, uint64)

	// ConnectedPeers returns an array consisting of all connected peers.
	ConnectedPeers() []RPCServerPeer

	// PersistentPeers returns an array consisting of all the persistent
	// peers.
	PersistentPeers() []RPCServerPeer

	// BroadcastMessage sends the provided message to all currently
	// connected peers.
	BroadcastMessage(msg wire.Message)

	// AddRebroadcastInventory adds the provided inventory to the list of
	// inventories to be rebroadcast at random intervals until they show up
	// in a block.
	AddRebroadcastInventory(iv *wire.InvVect, data interface{})

	// RelayTransactions generates and relays inventory vectors for all of
	// the passed transactions to all connected peers.
	RelayTransactions(txns []*mempool.TxDesc)
}

// WsNotificationManager is a connection and notification manager used for
// websockets.  It allows websocket clients to register for notifications they
// are interested in.  When an event happens elsewhere in the code such as
// transactions being added to the memory pool or block connects/disconnects,
// the notification manager is provided with the relevant details needed to
// figure out which websocket clients need to be notified based on what they
// have registered for and notifies them accordingly.  It is also used to keep
// track of all connected websocket clients.
type WsNotificationManager struct {
	// server is the RPC server the notification manager is associated with.
	server *RPCServer

	// queueNotification queues a notification for handling.
	queueNotification chan interface{}

	// notificationMsgs feeds notificationHandler with notifications
	// and client (un)registeration requests from a queue as well as
	// registeration and unregisteration requests from clients.
	notificationMsgs chan interface{}

	// Access channel for current number of connected clients.
	numClients chan int

	// Shutdown handling
	wg   sync.WaitGroup
	quit chan struct{}
}

// CfHeaderKV is a tuple of a filter header and its associated block hash. The
// struct is used to cache cfcheckpt responses.
type CfHeaderKV struct {
	blockHash    chainhash.Hash
	filterHeader chainhash.Hash
}

// GetConnCountMsg is
type GetConnCountMsg struct {
	reply chan int32
}

// GetPeersMsg is
type GetPeersMsg struct {
	reply chan []*Peer
}

// GetOutboundGroup is
type GetOutboundGroup struct {
	key   string
	reply chan int
}

// GetAddedNodesMsg is
type GetAddedNodesMsg struct {
	reply chan []*Peer
}

// DisconnectNodeMsg is
type DisconnectNodeMsg struct {
	cmp   func(*Peer) bool
	reply chan error
}

// ConnectNodeMsg is
type ConnectNodeMsg struct {
	addr      string
	permanent bool
	reply     chan error
}

// RemoveNodeMsg is
type RemoveNodeMsg struct {
	cmp   func(*Peer) bool
	reply chan error
}

// BroadcastMsg provides the ability to house a bitcoin message to be broadcast
// to all connected peers except specified excluded peers.
type BroadcastMsg struct {
	message      wire.Message
	excludePeers []*Peer
}

// BroadcastInventoryAdd is a type used to declare that the InvVect it contains
// needs to be added to the rebroadcast map
type BroadcastInventoryAdd RelayMsg

// BroadcastInventoryDel is a type used to declare that the InvVect it contains
// needs to be removed from the rebroadcast map
type BroadcastInventoryDel *wire.InvVect

// RelayMsg packages an inventory vector along with the newly discovered
// inventory so the relay has access to that information.
type RelayMsg struct {
	invVect *wire.InvVect
	data    interface{}
}

// UpdatePeerHeightsMsg is a message sent from the blockmanager to the server
// after a new block has been accepted. The purpose of the message is to update
// the heights of peers that were known to announce the block before we
// connected it to the main chain or recognized it as an orphan. With these
// updates, peer heights will be kept up to date, allowing for fresh data when
// selecting sync peer candidacy.
type UpdatePeerHeightsMsg struct {
	newHash    *chainhash.Hash
	newHeight  int32
	originPeer *peer.Peer
}

// PeerState maintains state of inbound, persistent, outbound peers as well
// as banned peers and outbound groups.
type PeerState struct {
	inboundPeers    map[int32]*Peer
	outboundPeers   map[int32]*Peer
	persistentPeers map[int32]*Peer
	banned          map[string]time.Time
	outboundGroups  map[string]int
}

// GbtWorkState houses state that is used in between multiple RPC invocations to
// getblocktemplate.
type GbtWorkState struct {
	sync.Mutex
	lastTxUpdate  time.Time
	lastGenerated time.Time
	prevHash      *chainhash.Hash
	minTimestamp  time.Time
	template      *mining.BlockTemplate
	notifyMap     map[chainhash.Hash]map[int64]chan struct{}
	timeSource    blockchain.MedianTimeSource
}

// HelpCacher provides a concurrent safe type that provides help and usage for
// the RPC server commands and caches the results for future calls.
type HelpCacher struct {
	sync.Mutex
	usage      string
	methodHelp map[string]string
}
