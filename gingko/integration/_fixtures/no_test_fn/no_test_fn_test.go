package no_test_fn_test

import (
	. "github.com/parallelcointeam/pod/gingko"
	. "github.com/parallelcointeam/pod/gingko/integration/_fixtures/no_test_fn"
	. "github.com/parallelcointeam/pod/gomega"
)

var _ = Describe("NoTestFn", func() {
	It("should proxy strings", func() {
		Ω(StringIdentity("foo")).Should(Equal("foo"))
	})
})
