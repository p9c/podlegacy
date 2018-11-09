package headerlist

import (
	"github.com/parallelcointeam/pod/wire"
)

// Chain is an interface that stores a list of Nodes. Each node represents a
// header in the main chain and also includes a height along with it. This is
// meant to serve as a replacement to list.List which provides similar
// functionality, but allows implementations to use custom storage backends and
// semantics.
type Chain interface {
	// ResetHeaderState resets the state of all nodes. After this method, it will
	// be as if the chain was just newly created.
	ResetHeaderState(Node)

	// Back returns the end of the chain. If the chain is empty, then this
	// return a pointer to a nil node.
	Back() *Node

	// Front returns the head of the chain. If the chain is empty, then
	// this returns a  pointer to a nil node.
	Front() *Node

	// PushBack will push a new entry to the end of the chain. The entry
	// added to the chain is also returned in place.
	PushBack(Node) *Node
}

// Node is a node within the Chain. Each node stores a header as well as a
// height. Nodes can also be used to traverse the chain backwards via their
// Prev() method.
type Node struct {
	// Height is the height of this node within the main chain.
	Height int32

	// Header is the header that this node represents.
	Header wire.BlockHeader

	prev *Node
}

// Prev attempts to access the prior node within the header chain relative to
// this node. If this is the start of the chain, then this method will return
// nil.
func (n *Node) Prev() *Node {
	return n.prev
}

// GetAlgo returns the algorithm of a block node
func (n *Node) GetAlgo() int32 {
	return n.Header.Version
}

// GetPrevWithAlgo returns the previous block from the current with the same algorithm
func (n *Node) GetPrevWithAlgo(algo int32) (prev *Node) {
	if n == nil {
		return nil
	}
	if n.GetAlgo() == algo {
		return n
	}
	prev = n.Prev()
	if prev == nil {
		return
	}
	for algo != n.Header.Version {
		prev = prev.Prev()

		if prev == nil {
			return
		}
	}
	return
}
