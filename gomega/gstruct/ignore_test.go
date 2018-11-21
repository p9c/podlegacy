package gstruct_test

import (
	. "github.com/parallelcointeam/pod/gingko"
	. "github.com/parallelcointeam/pod/gomega"
	. "github.com/parallelcointeam/pod/gomega/gstruct"
)

var _ = Describe("Ignore", func() {
	It("should always succeed", func() {
		Expect(nil).Should(Ignore())
		Expect(struct{}{}).Should(Ignore())
		Expect(0).Should(Ignore())
		Expect(false).Should(Ignore())
	})

	It("should always fail", func() {
		Expect(nil).ShouldNot(Reject())
		Expect(struct{}{}).ShouldNot(Reject())
		Expect(1).ShouldNot(Reject())
		Expect(true).ShouldNot(Reject())
	})
})
