package tmp

import (
	. "github.com/parallelcointeam/pod/gingko"
	. "github.com/parallelcointeam/pod/gomega"

	"testing"
)

func TestTmp(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Tmp Suite")
}
