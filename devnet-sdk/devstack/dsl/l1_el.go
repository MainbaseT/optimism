package dsl

import "github.com/ethereum-optimism/optimism/devnet-sdk/devstack/stack"

// L1ELNode wraps a stack.L1ELNode interface for DSL operations
type L1ELNode struct {
	commonImpl
	inner stack.L1ELNode
}

// NewL1ELNode creates a new L1ELNode DSL wrapper
func NewL1ELNode(inner stack.L1ELNode) *L1ELNode {
	return &L1ELNode{
		commonImpl: commonFromT(inner.T()),
		inner:      inner,
	}
}

func (el *L1ELNode) String() string {
	return el.inner.ID().String()
}

// Escape returns the underlying stack.L1ELNode
func (el *L1ELNode) Escape() stack.L1ELNode {
	return el.inner
}
