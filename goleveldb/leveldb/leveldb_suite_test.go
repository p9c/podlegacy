package leveldb

import (
	"testing"

	"github.com/parallelcointeam/pod/goleveldb/leveldb/testutil"
)

func TestLevelDB(t *testing.T) {
	testutil.RunSuite(t, "LevelDB Suite")
}
