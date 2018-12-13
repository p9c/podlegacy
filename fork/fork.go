// Package fork handles tracking the hard fork status and is used to determine which consensus rules apply on a block
package fork

// HardForks is the details related to a hard fork, number, name and activation height
type HardForks struct {
	Number           uint32
	Name             string
	ActivationHeight uint64
}

var (
	// List is the list of existing hard forks and when they activate
	List = []HardForks{
		HardForks{
			Number:           0,
			Name:             "Halcyon days",
			ActivationHeight: 0, // Approximately 18 Jan 2019
		},
		HardForks{
			Number:           1,
			Name:             "Plan 9 from Crypto Space",
			ActivationHeight: 184158, // Approximately 18 Jan 2019
		},
	}
)

// GetCurrent returns the number of the hard fork active at the given height
func GetCurrent(height uint64, testnet bool) (activeFork uint32) {
	for i := range List {
		// If testnet is active,
		if height > List[i].ActivationHeight || testnet {
			activeFork = List[i].Number
		}
	}
	return
}
