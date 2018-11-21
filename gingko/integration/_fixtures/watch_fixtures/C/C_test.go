package C_test

import (
	. "github.com/parallelcointeam/pod/gingko/integration/_fixtures/watch_fixtures/C"

	. "github.com/parallelcointeam/pod/gingko"
	. "github.com/parallelcointeam/pod/gomega"
)

var _ = Describe("C", func() {
	It("should do it", func() {
		Ω(DoIt()).Should(Equal("done!"))
	})
})
