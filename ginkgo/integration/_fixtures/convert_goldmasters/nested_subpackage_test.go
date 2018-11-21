package subpackage

import (
	. "github.com/parallelcointeam/pod/gingko"
)

var _ = Describe("Testing with Ginkgo", func() {
	It("nested sub packages", func() {
		GinkgoT().Fail(true)
	})
})
