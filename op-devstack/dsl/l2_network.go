package dsl

import (
	"fmt"
	"math"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-devstack/stack"
	"github.com/ethereum-optimism/optimism/op-devstack/stack/match"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/wait"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/retry"
)

// L2Network wraps a stack.L2Network interface for DSL operations
type L2Network struct {
	commonImpl
	inner stack.L2Network
}

// NewL2Network creates a new L2Network DSL wrapper
func NewL2Network(inner stack.L2Network) *L2Network {
	return &L2Network{
		commonImpl: commonFromT(inner.T()),
		inner:      inner,
	}
}

func (n *L2Network) String() string {
	return n.inner.ID().String()
}

func (n *L2Network) ChainID() eth.ChainID {
	return n.inner.ChainID()
}

// Escape returns the underlying stack.L2Network
func (n *L2Network) Escape() stack.L2Network {
	return n.inner
}

func (n *L2Network) CatchUpTo(o *L2Network) {
	this := n.inner.L2ELNode(match.FirstL2EL)
	other := o.inner.L2ELNode(match.FirstL2EL)

	err := wait.For(n.ctx, 5*time.Second, func() (bool, error) {
		a, err := this.EthClient().InfoByLabel(n.ctx, "latest")
		if err != nil {
			return false, err
		}

		b, err := other.EthClient().InfoByLabel(n.ctx, "latest")
		if err != nil {
			return false, err
		}

		eps := 6.0 // 6 seconds
		if math.Abs(float64(a.Time()-b.Time())) > eps {
			n.log.Warn("L2 networks too far off each other", n.String(), a.Time(), o.String(), b.Time())
			return false, nil
		}

		return true, nil
	})
	n.require.NoError(err, "Expected to get latest block from L2 execution clients")
}

func (n *L2Network) WaitForBlock() eth.BlockRef {
	return NewL2ELNode(n.inner.L2ELNode(match.FirstL2EL)).WaitForBlock()
}

func (n *L2Network) PublicRPC() *L2ELNode {
	if proxyds := match.Proxyd.Match(n.Escape().L2ELNodes()); len(proxyds) > 0 {
		return NewL2ELNode(proxyds[0])
	}
	// Fallback since sysgo doesn't have proxyd support at the moment, and may never get it.
	return NewL2ELNode(n.inner.L2ELNode(match.FirstL2EL))
}

// PrintChain is used for testing/debugging, it prints the blockchain hashes and parent hashes to logs, which is useful when developing reorg tests
func (n *L2Network) PrintChain() {
	l2_el := n.inner.L2ELNode(match.FirstL2EL)
	l2_cl := n.inner.L2CLNode(match.FirstL2CL)

	unsafeHeadRef := n.UnsafeHeadRef()

	var entries []string
	for i := unsafeHeadRef.Number; i > 0; i-- {
		ref, err := l2_el.EthClient().BlockRefByNumber(n.ctx, i)
		n.require.NoError(err, "Expected to get block ref by number")

		l2blockref, err := l2_el.L2EthClient().L2BlockRefByHash(n.ctx, ref.Hash)
		n.require.NoError(err, "Expected to get block ref by hash")

		entries = append(entries, fmt.Sprintln("Time: ", ref.Time, "Number: ", ref.Number, "Hash: ", ref.Hash.Hex(), "Parent: ", ref.ParentID().Hash.Hex(), "L1 Origin: ", l2blockref.L1Origin))
	}

	syncStatus, err := l2_cl.RollupAPI().SyncStatus(n.ctx)
	n.require.NoError(err, "Expected to get sync status")

	entries = append(entries, "")
	entries = append(entries, "Supervisor Sync view")
	entries = append(entries, "")
	entries = append(entries, fmt.Sprintf("Current L1:      %s", syncStatus.CurrentL1))
	entries = append(entries, fmt.Sprintf("Head L1:         %s", syncStatus.HeadL1))
	entries = append(entries, fmt.Sprintf("Safe L1:         %s", syncStatus.SafeL1))
	entries = append(entries, fmt.Sprintf("Unsafe L2:       %s", syncStatus.UnsafeL2))
	entries = append(entries, fmt.Sprintf("Local-Safe L2:   %s", syncStatus.LocalSafeL2))
	entries = append(entries, fmt.Sprintf("Cross-Unsafe L2: %s", syncStatus.CrossUnsafeL2))
	entries = append(entries, fmt.Sprintf("Cross-Safe L2:   %s", syncStatus.SafeL2))

	n.log.Info("Printing block hashes and parent hashes", "network", n.String(), "chain", n.ChainID())
	spew.Dump(entries)
}

func (n *L2Network) UnsafeHeadRef() eth.BlockRef {
	l2_el := n.inner.L2ELNode(match.FirstL2EL)

	unsafeHead, err := l2_el.EthClient().InfoByLabel(n.ctx, eth.Unsafe)
	n.require.NoError(err, "Expected to get latest block from L2 execution client")

	unsafeHeadRef, err := l2_el.EthClient().BlockRefByHash(n.ctx, unsafeHead.Hash())
	n.require.NoError(err, "Expected to get block ref by hash")

	return unsafeHeadRef
}

// LatestBlockBeforeTimestamp finds the latest block before fork activation
func (n *L2Network) LatestBlockBeforeTimestamp(t devtest.T, timestamp uint64) eth.BlockRef {
	require := t.Require()

	t.Gate().Greater(timestamp, uint64(0), "Must not start fork at genesis")

	blockNum, err := n.Escape().RollupConfig().TargetBlockNumber(timestamp)
	require.NoError(err)

	el := n.Escape().L2ELNode(match.FirstL2EL)
	head, err := el.EthClient().BlockRefByLabel(t.Ctx(), eth.Unsafe)
	require.NoError(err)

	t.Logger().Info("Preparing",
		"head", head, "head_time", head.Time,
		"target_num", blockNum, "target_time", timestamp)

	if head.Number < blockNum {
		t.Logger().Info("No block with given timestamp yet, checking head block instead")
		return head
	} else {
		t.Logger().Info("Reached block already, proceeding with last block before timestamp")
		v, err := el.EthClient().BlockRefByNumber(t.Ctx(), blockNum-1)
		require.NoError(err)
		return v
	}
}

// AwaitActivation awaits the fork activation time, and returns the activation block
func (n *L2Network) AwaitActivation(t devtest.T, forkName rollup.ForkName) eth.BlockID {
	require := t.Require()

	el := n.Escape().L2ELNode(match.FirstL2EL)

	unsafeHead, err := retry.Do(t.Ctx(), 120, &retry.FixedStrategy{Dur: 500 * time.Millisecond}, func() (eth.BlockRef, error) {
		unsafeHead, err := el.EthClient().BlockRefByLabel(t.Ctx(), eth.Unsafe)
		if err != nil {
			return eth.BlockRef{}, err
		}
		if !n.inner.RollupConfig().IsActivationBlockForFork(unsafeHead.Time, forkName) {
			return eth.BlockRef{}, fmt.Errorf("not %s activation block", forkName)
		}
		return unsafeHead, nil // success
	})
	require.NoError(err)
	t.Logger().Info("Activation block", "block", unsafeHead.ID())

	return unsafeHead.ID()
}
