// Copyright (c) 2013-2017 The btcsuite developers
// Copyright (c) 2017 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package server

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/parallelcointeam/pod/Log"
	"github.com/parallelcointeam/pod/addrmgr"
	"github.com/parallelcointeam/pod/chain"
	"github.com/parallelcointeam/pod/chain/indexers"
	"github.com/parallelcointeam/pod/connmgr"
	"github.com/parallelcointeam/pod/database"
	"github.com/parallelcointeam/pod/mempool"
	"github.com/parallelcointeam/pod/mining"
	"github.com/parallelcointeam/pod/mining/cpuminer"
	"github.com/parallelcointeam/pod/netsync"
	"github.com/parallelcointeam/pod/peer"
	"github.com/parallelcointeam/pod/txscript"

	"github.com/jrick/logrotate/rotator"
)

// LogWriter implements an io.Writer that outputs to both standard output and
// the write-end pipe of an initialized log rotator.
type LogWriter struct{}

func (LogWriter) Write(p []byte) (n int, err error) {
	os.Stdout.Write(p)
	LogRotator.Write(p)
	return len(p), nil
}

// Loggers per subsystem.  A single backend logger is created and all subsytem
// loggers created from it will write to the backend.  When adding new
// subsystems, add the subsystem logger variable here and to the
//  map.
//
// Loggers can not be used before the log rotator has been initialized with a
// log file.  This must be performed early during application startup by calling
// InitLogRotator.
var (
	// BackendLog is the logging backend used to create all subsystem loggers.
	// The backend must not be used before the log rotator has been initialized,
	// or data races and/or nil pointer dereferences will occur.
	BackendLog = Log.NewBackend(LogWriter{})

	// logRotator is one of the logging outputs.  It should be closed on
	// application shutdown.
	LogRotator *rotator.Rotator

	AdxrLog = BackendLog.Logger("ADXR")
	AmgrLog = BackendLog.Logger("AMGR")
	CmgrLog = BackendLog.Logger("CMGR")
	BcdbLog = BackendLog.Logger("BCDB")
	PodLog  = BackendLog.Logger(" POD")
	ChanLog = BackendLog.Logger("CHAN")
	DiscLog = BackendLog.Logger("DISC")
	IndxLog = BackendLog.Logger("INDX")
	MinrLog = BackendLog.Logger("MINR")
	PeerLog = BackendLog.Logger("PEER")
	RpcsLog = BackendLog.Logger("RPCS")
	ScrpLog = BackendLog.Logger("SCRP")
	SrvrLog = BackendLog.Logger("SRVR")
	SyncLog = BackendLog.Logger("SYNC")
	TxmpLog = BackendLog.Logger("TXMP")
)

// Initialize package-global logger variables.
func init() {
	addrmgr.UseLogger(AmgrLog)
	connmgr.UseLogger(CmgrLog)
	database.UseLogger(BcdbLog)
	blockchain.UseLogger(ChanLog)
	indexers.UseLogger(IndxLog)
	mining.UseLogger(MinrLog)
	cpuminer.UseLogger(MinrLog)
	peer.UseLogger(PeerLog)
	txscript.UseLogger(ScrpLog)
	netsync.UseLogger(SyncLog)
	mempool.UseLogger(TxmpLog)
}

// SubsystemLoggers maps each subsystem identifier to its associated logger.
var SubsystemLoggers = map[string]Log.Logger{
	"ADXR": AdxrLog,
	"AMGR": AmgrLog,
	"CMGR": CmgrLog,
	"BCDB": BcdbLog,
	"BTCD": PodLog,
	"CHAN": ChanLog,
	"DISC": DiscLog,
	"INDX": IndxLog,
	"MINR": MinrLog,
	"PEER": PeerLog,
	"RPCS": RpcsLog,
	"SCRP": ScrpLog,
	"SRVR": SrvrLog,
	"SYNC": SyncLog,
	"TXMP": TxmpLog,
}

// InitLogRotator initializes the logging rotater to write logs to logFile and
// create roll files in the same directory.  It must be called before the
// package-global log rotater variables are used.
func InitLogRotator(logFile string) {
	logDir, _ := filepath.Split(logFile)
	err := os.MkdirAll(logDir, 0700)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create log directory: %v\n", err)
		os.Exit(1)
	}
	r, err := rotator.New(logFile, 10*1024, false, 3)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create file rotator: %v\n", err)
		os.Exit(1)
	}

	LogRotator = r
}

// SetLogLevel sets the logging level for provided subsystem.  Invalid
// subsystems are ignored.  Uninitialized subsystems are dynamically created as
// needed.
func SetLogLevel(subsystemID string, logLevel string) {
	// Ignore invalid subsystems.
	logger, ok := SubsystemLoggers[subsystemID]
	if !ok {
		return
	}

	// Defaults to info if the log level is invalid.
	level, _ := Log.LevelFromString(logLevel)
	logger.SetLevel(level)
}

// SetLogLevels sets the log level for all subsystem loggers to the passed
// level.  It also dynamically creates the subsystem loggers as needed, so it
// can be used to initialize the logging system.
func SetLogLevels(logLevel string) {
	// Configure all sub-systems with the new logging level.  Dynamically
	// create loggers as needed.
	for subsystemID := range SubsystemLoggers {
		SetLogLevel(subsystemID, logLevel)
	}
}

// DirectionString is a helper function that returns a string that represents
// the direction of a connection (inbound or outbound).
func DirectionString(inbound bool) string {
	if inbound {
		return "inbound"
	}
	return "outbound"
}

// PickNoun returns the singular or plural form of a noun depending
// on the count n.
func PickNoun(n uint64, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}
