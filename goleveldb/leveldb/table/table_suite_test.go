package table

import (
	"testing"

	"github.com/parallelcointeam/pod/goleveldb/leveldb/testutil"
)

func TestTable(t *testing.T) {
	testutil.RunSuite(t, "Table Suite")
}
