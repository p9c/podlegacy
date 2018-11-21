package types_test

import (
	. "github.com/parallelcointeam/pod/gingko"
	. "github.com/parallelcointeam/pod/gomega"

	"testing"
)

func TestTypes(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Types Suite")
}
