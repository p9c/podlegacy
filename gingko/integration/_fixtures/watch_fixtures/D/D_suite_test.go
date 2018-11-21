package D_test

import (
	. "github.com/parallelcointeam/pod/gingko"
	. "github.com/parallelcointeam/pod/gomega"

	"testing"
)

func TestD(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "D Suite")
}
