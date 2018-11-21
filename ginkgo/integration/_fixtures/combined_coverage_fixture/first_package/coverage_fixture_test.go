package first_package_test

import (
	. "github.com/parallelcointeam/pod/gingko/integration/_fixtures/combined_coverage_fixture/first_package"
	. "github.com/parallelcointeam/pod/gingko/integration/_fixtures/combined_coverage_fixture/first_package/external_coverage_fixture"

	. "github.com/parallelcointeam/pod/gingko"
	. "github.com/parallelcointeam/pod/gomega"
)

var _ = Describe("CoverageFixture", func() {
	It("should test A", func() {
		Ω(A()).Should(Equal("A"))
	})

	It("should test B", func() {
		Ω(B()).Should(Equal("B"))
	})

	It("should test C", func() {
		Ω(C()).Should(Equal("C"))
	})

	It("should test D", func() {
		Ω(D()).Should(Equal("D"))
	})

	It("should test external package", func() {
		Ω(Tested()).Should(Equal("tested"))
	})
})
