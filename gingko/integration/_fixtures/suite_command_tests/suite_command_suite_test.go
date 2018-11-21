package suite_command_test

import (
	. "github.com/parallelcointeam/pod/gingko"
	. "github.com/parallelcointeam/pod/gomega"

	"testing"
)

func TestSuiteCommand(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Suite Command Suite")
}
