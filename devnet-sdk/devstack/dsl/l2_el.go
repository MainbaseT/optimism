package dsl

import "github.com/ethereum-optimism/optimism/devnet-sdk/devstack/stack"

// L2ELNode wraps a stack.L2ELNode interface for DSL operations
type L2ELNode struct {
	commonImpl
	inner stack.L2ELNode
}

// NewL2ELNode creates a new L2ELNode DSL wrapper
func NewL2ELNode(inner stack.L2ELNode) *L2ELNode {
	return &L2ELNode{
		commonImpl: commonFromT(inner.T()),
		inner:      inner,
	}
}

func (el *L2ELNode) String() string {
	return el.inner.ID().String()
}

// Escape returns the underlying stack.L2ELNode
func (el *L2ELNode) Escape() stack.L2ELNode {
	return el.inner
}
