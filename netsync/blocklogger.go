package netsync

import (
	"fmt"
	"github.com/parallelcointeam/pod/btclog"
	"github.com/parallelcointeam/pod/btcutil"
	"sync"
	"time"
)

// blockProgressLogger provides periodic logging for other services in order to show users progress of certain "actions" involving some or all current blocks. Ex: syncing to best chain, indexing all blocks, etc.
type blockProgressLogger struct {
	receivedLogBlocks int64
	receivedLogTx     int64
	lastBlockLogTime  time.Time
	subsystemLogger   btclog.Logger
	progressAction    string
	sync.Mutex
}

// newBlockProgressLogger returns a new block progress logger. The progress message is templated as follows:  {progressAction} {numProcessed} {blocks|block} in the last {timePeriod}  ({numTxs}, height {lastBlockHeight}, {lastBlockTimeStamp})
func newBlockProgressLogger(progressMessage string, logger btclog.Logger) *blockProgressLogger {
	return &blockProgressLogger{
		lastBlockLogTime: time.Now(),
		progressAction:   progressMessage,
		subsystemLogger:  logger,
	}
}

// LogBlockHeight logs a new block height as an information message to show progress to the user. In order to prevent spam, it limits logging to one message every 10 seconds with duration and totals included.
func (b *blockProgressLogger) LogBlockHeight(block *btcutil.Block) {
	b.Lock()
	defer b.Unlock()
	b.receivedLogBlocks++
	b.receivedLogTx += int64(len(block.MsgBlock().Transactions))
	now := time.Now()
	duration := now.Sub(b.lastBlockLogTime)
	if duration < time.Second*2 {
		return
	}
	// Truncate the duration to 10s of milliseconds.
	durationMillis := int64(duration / time.Millisecond)
	tDuration := 10 * time.Millisecond * time.Duration(durationMillis/10)
	// Log information about new block height.
	blockStr := "blocks"
	if b.receivedLogBlocks == 1 {
		blockStr = "block "
	}
	txStr := "transactions"
	if b.receivedLogTx == 1 {
		txStr = "transaction "
	}
	b.subsystemLogger.Infof("%s %6d %s in the last %s (%6d %s, height %8d, %s)",
		b.progressAction, b.receivedLogBlocks, blockStr, fmt.Sprintf("%0.1fs", tDuration.Seconds()), b.receivedLogTx,
		txStr, block.Height(), block.MsgBlock().Header.Timestamp)
	b.receivedLogBlocks = 0
	b.receivedLogTx = 0
	b.lastBlockLogTime = now
}
func (b *blockProgressLogger) SetLastLogTime(time time.Time) {
	b.lastBlockLogTime = time
}
