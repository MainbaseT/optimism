package interop

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/actions/interop/dsl"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/contracts/bindings/emit"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-node/rollup/interop/indexing"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/event"
	stypes "github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

func TestFullInterop(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)

	is := dsl.SetupInterop(t)
	actors := is.CreateActors()
	actors.PrepareAndVerifyInitialState(t)

	// sync the supervisor, handle initial events emitted by the nodes
	actors.ChainA.Sequencer.SyncSupervisor(t)
	actors.ChainB.Sequencer.SyncSupervisor(t)

	// sync initial chain A and B
	actors.Supervisor.ProcessFull(t)

	// full is a helper function to run the full interop flow for a given chain
	// it will create a block and process it from unsafe to finalized
	full := func(chain *dsl.Chain, l1BlocksToProcess int) {
		// No blocks yet
		status := chain.Sequencer.SyncStatus()
		require.Equal(t, uint64(0), status.UnsafeL2.Number)
		chain.Sequencer.ActL2StartBlock(t)
		chain.Sequencer.ActL2EndBlock(t)
		status = chain.Sequencer.SyncStatus()
		head := status.UnsafeL2.ID()
		require.Equal(t, uint64(1), head.Number)
		require.Equal(t, uint64(0), status.CrossUnsafeL2.Number)
		require.Equal(t, uint64(0), status.LocalSafeL2.Number)
		require.Equal(t, uint64(0), status.SafeL2.Number)
		require.Equal(t, uint64(0), status.FinalizedL2.Number)

		// Ingest the new unsafe-block event
		chain.Sequencer.SyncSupervisor(t)

		// Verify as cross-unsafe with supervisor
		actors.Supervisor.ProcessFull(t)
		chain.Sequencer.ActL2PipelineFull(t)
		status = chain.Sequencer.SyncStatus()
		require.Equal(t, head, status.UnsafeL2.ID())
		require.Equal(t, head, status.CrossUnsafeL2.ID())
		require.Equal(t, uint64(0), status.LocalSafeL2.Number)
		require.Equal(t, uint64(0), status.SafeL2.Number)
		require.Equal(t, uint64(0), status.FinalizedL2.Number)
		supervisorStatus, err := actors.Supervisor.SyncStatus(t.Ctx())
		require.NoError(t, err)
		require.Equal(t, head, supervisorStatus.Chains[chain.ChainID].LocalUnsafe.ID())

		// Submit the L2 block, sync the local-safe data
		chain.Batcher.ActSubmitAll(t)
		actors.L1Miner.ActL1StartBlock(12)(t)
		actors.L1Miner.ActL1IncludeTx(chain.BatcherAddr)(t)
		actors.L1Miner.ActL1EndBlock(t)

		// The node will exhaust L1 data,
		// it needs the supervisor to see the L1 block first,
		// and provide it to the node.
		for i := 0; i < l1BlocksToProcess; i++ {
			chain.Sequencer.ActL2EventsUntil(t, event.Is[derive.ExhaustedL1Event], 100, false)
			actors.Supervisor.SignalLatestL1(t)  // supervisor will be aware of latest L1
			chain.Sequencer.SyncSupervisor(t)    // supervisor to react to exhaust-L1
			chain.Sequencer.ActL2PipelineFull(t) // node to complete syncing to L1 head.
		}

		// Theoretically shouldn't require this ActL1HeadSignal in managed mode, but currently it is required.
		chain.Sequencer.ActL1HeadSignal(t)
		status = chain.Sequencer.SyncStatus()
		require.Equal(t, head, status.UnsafeL2.ID())
		require.Equal(t, head, status.CrossUnsafeL2.ID())
		require.Equal(t, head, status.LocalSafeL2.ID())
		require.Equal(t, uint64(0), status.SafeL2.Number)
		require.Equal(t, uint64(0), status.FinalizedL2.Number)
		supervisorStatus, err = actors.Supervisor.SyncStatus(t.Ctx())
		require.NoError(t, err)
		require.Equal(t, head, supervisorStatus.Chains[chain.ChainID].LocalUnsafe.ID())
		// Local-safe does not count as "safe" in RPC
		n := chain.SequencerEngine.L2Chain().CurrentSafeBlock().Number.Uint64()
		require.Equal(t, uint64(0), n)

		// Make the supervisor aware of the new L1 block
		actors.Supervisor.SignalLatestL1(t)

		// Ingest the new local-safe event
		chain.Sequencer.SyncSupervisor(t)

		// Cross-safe verify it
		actors.Supervisor.ProcessFull(t)
		chain.Sequencer.ActL2PipelineFull(t)
		status = chain.Sequencer.SyncStatus()
		require.Equal(t, head, status.UnsafeL2.ID())
		require.Equal(t, head, status.CrossUnsafeL2.ID())
		require.Equal(t, head, status.LocalSafeL2.ID())
		require.Equal(t, head, status.SafeL2.ID())
		require.Equal(t, uint64(0), status.FinalizedL2.Number)
		supervisorStatus, err = actors.Supervisor.SyncStatus(t.Ctx())
		require.NoError(t, err)
		require.Equal(t, head, supervisorStatus.Chains[chain.ChainID].LocalUnsafe.ID())
		h := chain.SequencerEngine.L2Chain().CurrentSafeBlock().Hash()
		require.Equal(t, head.Hash, h)

		// Finalize L1, and see if the supervisor updates the op-node finality accordingly.
		// The supervisor then determines finality, which the op-node can use.
		actors.L1Miner.ActL1SafeNext(t)
		actors.L1Miner.ActL1FinalizeNext(t)
		chain.Sequencer.ActL1SafeSignal(t) // TODO old source of finality
		chain.Sequencer.ActL1FinalizedSignal(t)
		actors.Supervisor.SignalFinalizedL1(t)
		actors.Supervisor.ProcessFull(t)
		chain.Sequencer.ActL2PipelineFull(t)
		finalizedL2BlockID, err := actors.Supervisor.Finalized(t.Ctx(), chain.ChainID)
		require.NoError(t, err)
		require.Equal(t, head, finalizedL2BlockID)

		chain.Sequencer.ActL2PipelineFull(t)
		h = chain.SequencerEngine.L2Chain().CurrentFinalBlock().Hash()
		require.Equal(t, head.Hash, h)
		status = chain.Sequencer.SyncStatus()
		require.Equal(t, head, status.UnsafeL2.ID())
		require.Equal(t, head, status.CrossUnsafeL2.ID())
		require.Equal(t, head, status.LocalSafeL2.ID())
		require.Equal(t, head, status.SafeL2.ID())
		require.Equal(t, head, status.FinalizedL2.ID())
		supervisorStatus, err = actors.Supervisor.SyncStatus(t.Ctx())
		require.NoError(t, err)
		require.Equal(t, head, supervisorStatus.Chains[chain.ChainID].LocalUnsafe.ID())
	}
	// first run Chain A, processing 1 L1 block
	full(actors.ChainA, 1)
	supervisorStatus, err := actors.Supervisor.SyncStatus(t.Ctx())
	require.NoError(t, err)
	require.Equal(t, uint64(0), supervisorStatus.MinSyncedL1.Number)

	// then run Chain B, processing 2 L1 blocks (because L1 is further ahead now)
	full(actors.ChainB, 2)
	supervisorStatus, err = actors.Supervisor.SyncStatus(t.Ctx())
	require.NoError(t, err)
	require.Equal(t, uint64(1), supervisorStatus.MinSyncedL1.Number)
}

// TestFinality confirms that when L1 finality is updated on the supervisor,
// the L2 finality signal updates to the appropriate value.
// Sub-tests control how many additional blocks might be submitted to the L1 chain,
// affecting the way Finality would be determined.
func TestFinality(gt *testing.T) {
	testFinality := func(t helpers.StatefulTesting, extraBlocks int) {
		is := dsl.SetupInterop(t)
		actors := is.CreateActors()
		actors.PrepareAndVerifyInitialState(t)
		actors.Supervisor.ProcessFull(t)

		// Build L2 block on chain A
		actors.ChainA.Sequencer.ActL2StartBlock(t)
		actors.ChainA.Sequencer.ActL2EndBlock(t)

		// Sync and process the supervisor, updating cross-unsafe
		actors.ChainA.Sequencer.SyncSupervisor(t)
		actors.Supervisor.ProcessFull(t)
		actors.ChainA.Sequencer.ActL2PipelineFull(t)

		// Submit the L2 block, sync the local-safe data
		actors.ChainA.Batcher.ActSubmitAll(t)
		actors.L1Miner.ActL1StartBlock(12)(t)
		actors.L1Miner.ActL1IncludeTx(actors.ChainA.BatcherAddr)(t)
		actors.L1Miner.ActL1EndBlock(t)
		actors.L1Miner.ActL1SafeNext(t)

		// Run the node until the L1 is exhausted
		// and have the supervisor provide the latest L1 block
		actors.ChainA.Sequencer.ActL2EventsUntil(t, event.Is[derive.ExhaustedL1Event], 100, false)
		actors.Supervisor.SignalLatestL1(t)
		actors.ChainA.Sequencer.SyncSupervisor(t)
		actors.ChainA.Sequencer.ActL2PipelineFull(t)
		actors.ChainA.Sequencer.ActL1HeadSignal(t)
		// Make the supervisor aware of the new L1 block
		actors.Supervisor.SignalLatestL1(t)
		// Ingest the new local-safe event
		actors.ChainA.Sequencer.SyncSupervisor(t)
		// Cross-safe verify it
		actors.Supervisor.ProcessFull(t)
		actors.ChainA.Sequencer.ActL2PipelineFull(t)

		// Submit more blocks to the L1, to bury the L2 block
		for i := 0; i < extraBlocks; i++ {
			actors.L1Miner.ActL1StartBlock(12)(t)
			actors.L1Miner.ActL1EndBlock(t)
			actors.L1Miner.ActL1SafeNext(t)
			actors.Supervisor.SignalLatestL1(t)
			actors.Supervisor.ProcessFull(t)
		}

		tip := actors.L1Miner.SafeNum()

		// Update finality on the supervisor to the latest block
		actors.L1Miner.ActL1Finalize(t, tip)
		actors.Supervisor.SignalFinalizedL1(t)

		// Process the supervisor to update the finality, and pull L1, L2 finality
		actors.Supervisor.ProcessFull(t)
		l1Finalized, err := actors.Supervisor.FinalizedL1(t.Ctx())
		require.NoError(t, err)
		l2Finalized, err := actors.Supervisor.Finalized(t.Ctx(), actors.ChainA.ChainID)
		require.NoError(t, err)
		require.Equal(t, uint64(tip), l1Finalized.Number)
		// the L2 finality is the latest L2 block, because L1 finality is beyond anything the L2 used to derive
		require.Equal(t, uint64(1), l2Finalized.Number)

		// confirm the node also sees the finality
		actors.ChainA.Sequencer.ActL2PipelineFull(t)
		status := actors.ChainA.Sequencer.SyncStatus()
		require.Equal(t, uint64(1), status.FinalizedL2.Number)
	}
	statefulT := helpers.NewDefaultTesting(gt)
	gt.Run("FinalizeBeyondDerived", func(t *testing.T) {
		testFinality(statefulT, 10)
	})
	gt.Run("Finalize", func(t *testing.T) {
		testFinality(statefulT, 0)
	})
}

func TestInteropLocalSafeInvalidation(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)

	is := dsl.SetupInterop(t)
	actors := is.CreateActors()
	actors.PrepareAndVerifyInitialState(t)
	genesisB := actors.ChainB.Sequencer.SyncStatus()

	// build L2 block on chain B with invalid executing message pointing to A.
	fakeMessage := []byte("this message was never emitted")
	aliceB := setupUser(t, is, actors.ChainB, 0)
	id := stypes.Identifier{
		Origin:      common.Address{0x42},
		BlockNumber: genesisB.UnsafeL2.Number,
		LogIndex:    0,
		Timestamp:   genesisB.UnsafeL2.Time,
		ChainID:     eth.ChainIDFromBig(actors.ChainA.RollupCfg.L2ChainID),
	}
	msgHash := crypto.Keccak256Hash(fakeMessage)
	tx := newExecuteMessageTxFromIDAndHash(t, aliceB, actors.ChainB, id, msgHash)

	actors.ChainB.Sequencer.ActL2StartBlock(t)
	_, err := actors.ChainB.SequencerEngine.EngineApi.IncludeTx(tx, aliceB.address)
	require.NoError(t, err)
	actors.ChainB.Sequencer.ActL2EndBlock(t)
	actors.ChainB.Sequencer.ActL2PipelineFull(t)
	originalBlock := actors.ChainB.Sequencer.SyncStatus().UnsafeL2
	require.Equal(t, uint64(1), originalBlock.Number)
	originalOutput, err := actors.ChainB.Sequencer.RollupClient().OutputAtBlock(t.Ctx(), originalBlock.Number)
	require.NoError(t, err)

	// build another empty L2 block, that will get reorged out
	actors.ChainB.Sequencer.ActL2StartBlock(t)
	actors.ChainB.Sequencer.ActL2EndBlock(t)
	actors.ChainB.Sequencer.ActL2PipelineFull(t)
	extraBlock := actors.ChainB.Sequencer.SyncStatus().UnsafeL2
	require.Equal(t, uint64(2), extraBlock.Number)

	// batch-submit the L2 block to L1
	actors.ChainB.Batcher.ActSubmitAll(t)
	actors.L1Miner.ActL1StartBlock(12)(t)
	actors.L1Miner.ActL1IncludeTx(actors.ChainB.BatcherAddr)(t)
	actors.L1Miner.ActL1EndBlock(t)

	// Signal the supervisor there is a new L1 block
	actors.Supervisor.SignalLatestL1(t)
	// sync the op-node, to signal that derivation needs the new L1 block
	t.Log("awaiting L1-exhaust event")
	actors.ChainB.Sequencer.ActL2PipelineFull(t)
	// sync the supervisor, so it can pass the L1 block to op-node
	t.Log("awaiting supervisor to provide L1 data")
	actors.ChainB.Sequencer.SyncSupervisor(t)
	actors.Supervisor.ProcessFull(t)
	// sync the op-node, so it derives the local-safe head
	t.Log("awaiting node to sync")
	actors.ChainB.Sequencer.ActL2PipelineFull(t)
	// Both L2 blocks were derived from the same L1 batch, and should have been processed into local-safe updates
	require.Equal(t, uint64(2), actors.ChainB.Sequencer.SyncStatus().LocalSafeL2.Number)
	// Make the supervisor process the derivation work from the op-node.
	// It should determine that the local-safe block needs replacement.
	t.Log("Expecting supervisor to sync and catch local-safe dependency issue")
	actors.ChainB.Sequencer.SyncSupervisor(t)
	actors.Supervisor.ProcessFull(t)
	// check supervisor head, expect it to be rewound
	localUnsafe, err := actors.Supervisor.LocalUnsafe(t.Ctx(), actors.ChainB.ChainID)
	require.NoError(t, err)
	require.Equal(t, uint64(0), localUnsafe.Number, "unsafe chain needs to be rewound")

	// Make the op-node do the processing to build the replacement
	t.Log("Expecting op-node to build replacement block")
	actors.ChainB.Sequencer.ActL2PipelineFull(t)
	// Make the supervisor pick up the replacement
	actors.ChainB.Sequencer.SyncSupervisor(t)
	actors.Supervisor.ProcessFull(t)
	// Check that the replacement is recognized as cross-safe
	crossSafe, err := actors.Supervisor.CrossSafe(t.Ctx(), actors.ChainB.ChainID)
	require.NoError(t, err)
	require.NotEqual(t, originalBlock.ID(), crossSafe.Derived)
	require.NotEqual(t, extraBlock.ID(), crossSafe.Derived)
	require.Equal(t, uint64(1), crossSafe.Derived.Number)

	actors.ChainB.Sequencer.ActL2PipelineFull(t)
	// check op-node head matches replacement block
	status := actors.ChainB.Sequencer.SyncStatus()
	require.Equal(t, crossSafe.Derived, status.SafeL2.ID())

	// Parse system tx from replacement block, assert it matches the original block
	replacementBlock, err := actors.ChainB.SequencerEngine.EthClient().BlockByHash(t.Ctx(), status.SafeL2.Hash)
	require.NoError(t, err)
	txs := replacementBlock.Transactions()
	out, err := indexing.DecodeInvalidatedBlockTx(txs[len(txs)-1])
	require.NoError(t, err)
	require.Equal(t, originalOutput.OutputRoot, eth.OutputRoot(out))

	// supervisor should have the new chain indexed now
	actors.ChainB.Sequencer.SyncSupervisor(t)
	actors.Supervisor.ProcessFull(t)
	localUnsafe, err = actors.Supervisor.LocalUnsafe(t.Ctx(), actors.ChainB.ChainID)
	require.NoError(t, err)
	require.Equal(t, crossSafe.Derived, localUnsafe)
	actors.ChainB.Sequencer.ActL2PipelineFull(t)
	status = actors.ChainB.Sequencer.SyncStatus()
	require.Equal(t, status.LocalSafeL2, status.UnsafeL2, "block follows on replacement block, derived to deconflict from previous empty block on L1")
	require.Equal(t, crossSafe.Derived, status.UnsafeL2.ParentID(), "builds on the new post-replacement chain")

	// Now check if we can continue to build L2 blocks on top of the new chain.
	// Build a new L2 block
	actors.ChainB.Sequencer.ActL2StartBlock(t)
	actors.ChainB.Sequencer.ActL2EndBlock(t)
	actors.ChainB.Sequencer.ActL2PipelineFull(t)
	// Batch submit the L2 block to L1
	actors.ChainB.Batcher.ActSubmitAll(t)
	actors.L1Miner.ActL1StartBlock(12)(t)
	actors.L1Miner.ActL1IncludeTx(actors.ChainB.BatcherAddr)(t)
	actors.L1Miner.ActL1EndBlock(t)

	// Takes a while to become local safe
	actors.Supervisor.SignalLatestL1(t)
	actors.ChainB.Sequencer.ActL2PipelineFull(t)
	actors.ChainB.Sequencer.SyncSupervisor(t)
	actors.Supervisor.ProcessFull(t)
	actors.ChainB.Sequencer.ActL2PipelineFull(t)
	status = actors.ChainB.Sequencer.SyncStatus()
	require.Equal(t, uint64(3), status.LocalSafeL2.Number)

	// And now cross-safe
	actors.ChainB.Sequencer.SyncSupervisor(t)
	actors.Supervisor.ProcessFull(t)
	actors.ChainB.Sequencer.ActL2PipelineFull(t)
	status = actors.ChainB.Sequencer.SyncStatus()
	require.Equal(t, uint64(3), status.SafeL2.Number)
}

func TestInteropCrossSafeDependencyDelay(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)

	is := dsl.SetupInterop(t)
	actors := is.CreateActors()
	actors.PrepareAndVerifyInitialState(t)
	// We create a batch with some empty blocks before and after the cross-chain message,
	// so multiple L2 blocks are all derived from the same L1 block.
	actors.ChainA.Sequencer.ActL2EmptyBlock(t)
	actors.ChainA.Sequencer.ActL2EmptyBlock(t)
	actors.ChainA.Sequencer.ActL2EmptyBlock(t)

	actors.ChainB.Sequencer.ActL2EmptyBlock(t)

	aliceA := setupUser(t, is, actors.ChainA, 0)
	aliceB := setupUser(t, is, actors.ChainB, 0)

	// create a log event in chain B
	auth := newL2TxOpts(t, aliceB.secret, actors.ChainB)
	emitContractAddr, deployTx, _, err := emit.DeployEmit(auth, actors.ChainB.SequencerEngine.EthClient())
	require.NoError(t, err)
	includeTxOnChainBasic(t, actors.ChainB, deployTx, aliceB.address)
	emitTx := newEmitMessageTx(t, actors.ChainB, aliceB, emitContractAddr, []byte("hello from B"))
	includeTxOnChainBasic(t, actors.ChainB, emitTx, aliceB.address)

	// consume the log event in chain A
	execTx := newExecuteMessageTx(t, actors.ChainA, aliceA, actors.ChainB, emitTx)
	includeTxOnChainBasic(t, actors.ChainA, execTx, aliceA.address)
	execTxIncludedIn := actors.ChainA.Sequencer.SyncStatus().UnsafeL2

	actors.ChainA.Sequencer.ActL2EmptyBlock(t)
	actors.ChainB.Sequencer.ActL2EmptyBlock(t)

	chainAHead := actors.ChainA.Sequencer.SyncStatus().UnsafeL2
	chainBHead := actors.ChainB.Sequencer.SyncStatus().UnsafeL2

	// Now submit the data for chain A, and submit the data of chain B late,
	// so the scope has to be bumped even though we know of the event in the unsafe chain already.

	actors.ChainA.Batcher.ActSubmitAll(t)
	actors.L1Miner.ActL1StartBlock(12)(t)
	actors.L1Miner.ActL1IncludeTx(actors.ChainA.BatcherAddr)(t)
	actors.L1Miner.ActL1EndBlock(t)

	actors.L1Miner.ActEmptyBlock(t)
	actors.L1Miner.ActEmptyBlock(t)

	actors.ChainB.Batcher.ActSubmitAll(t)
	actors.L1Miner.ActL1StartBlock(12)(t)
	actors.L1Miner.ActL1IncludeTx(actors.ChainB.BatcherAddr)(t)
	actors.L1Miner.ActL1EndBlock(t)
	chainBSubmittedIn, err := actors.L1Miner.EthClient().BlockByNumber(t.Ctx(), nil)
	require.NoError(t, err)

	actors.Supervisor.SignalLatestL1(t)
	actors.Supervisor.ProcessFull(t)

	actors.ChainA.Sequencer.ActL1HeadSignal(t)
	actors.ChainB.Sequencer.ActL1HeadSignal(t)
	actors.ChainA.Sequencer.ActL2PipelineFull(t)
	actors.ChainB.Sequencer.ActL2PipelineFull(t)

	// it takes a few round trips to sync the L1 blocks
	for i := 0; i < 5; i++ {
		actors.ChainA.Sequencer.SyncSupervisor(t)
		actors.ChainB.Sequencer.SyncSupervisor(t)
		actors.Supervisor.ProcessFull(t)
		actors.ChainA.Sequencer.ActL2PipelineFull(t)
		actors.ChainB.Sequencer.ActL2PipelineFull(t)
	}

	// Assert the blocks are now cross-safe
	require.Equal(t, chainAHead, actors.ChainA.Sequencer.SyncStatus().SafeL2)
	require.Equal(t, chainBHead, actors.ChainB.Sequencer.SyncStatus().SafeL2)

	// Assert that the executing message in chain A only
	// became cross-safe when the dependency of chain B became cross safe later.
	source, err := actors.Supervisor.CrossDerivedToSource(t.Ctx(), actors.ChainA.ChainID, execTxIncludedIn.ID())
	require.NoError(t, err)
	require.Equal(t, chainBSubmittedIn.NumberU64(), source.Number)
}

func TestInteropExecutingMessageOutOfRangeLogIndex(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)
	is := dsl.SetupInterop(t)
	actors := is.CreateActors()
	actors.PrepareAndVerifyInitialState(t)
	aliceA := setupUser(t, is, actors.ChainA, 0)

	// Execute a fake log on chain A
	chainBHead := actors.ChainB.Sequencer.SyncStatus().UnsafeL2
	nonExistentID := stypes.Identifier{
		Origin:      aliceA.address,
		BlockNumber: chainBHead.Number,
		LogIndex:    0,
		Timestamp:   chainBHead.Time,
		ChainID:     eth.ChainIDFromBig(actors.ChainB.RollupCfg.L2ChainID),
	}
	nonExistentHash := crypto.Keccak256Hash([]byte("fake message"))
	tx := newExecuteMessageTxFromIDAndHash(t, aliceA, actors.ChainA, nonExistentID, nonExistentHash)
	includeTxOnChainBasic(t, actors.ChainA, tx, aliceA.address)
	actors.ChainB.Sequencer.ActL2EmptyBlock(t)

	// Sync the system
	actors.ChainA.Sequencer.SyncSupervisor(t)
	actors.ChainB.Sequencer.SyncSupervisor(t)
	actors.Supervisor.ProcessFull(t)
	actors.ChainA.Sequencer.ActL2PipelineFull(t)
	actors.ChainB.Sequencer.ActL2PipelineFull(t)

	// Assert that chainA's block is not cross-safe but chainB's is.
	assertHeads(t, actors.ChainA, 1, 0, 0, 0)
	assertHeads(t, actors.ChainB, 1, 0, 1, 0)
}

func TestInteropIntraBlockReferenceReplacementChainBIsInvalid(gt *testing.T) {
	testInteropIntraBlockReferenceReplacement(gt, false)
}

func TestInteropIntraBlockReferenceReplacementChainAIsInvalid(gt *testing.T) {
	testInteropIntraBlockReferenceReplacement(gt, true)
}

func testInteropIntraBlockReferenceReplacement(gt *testing.T, reverse bool) {
	t := helpers.NewDefaultTesting(gt)
	system := dsl.NewInteropDSL(t)
	actors := system.Actors
	alice := system.CreateUser()
	emitterContract := dsl.NewEmitterContract(t)
	system.AddL2Block(actors.ChainA, dsl.WithL2BlockTransactions(
		emitterContract.Deploy(alice),
	))
	system.AddL2Block(actors.ChainB, dsl.WithL2BlockTransactions(
		emitterContract.Deploy(alice),
	))
	assertHeads(t, actors.ChainA, 1, 0, 1, 0)
	assertHeads(t, actors.ChainB, 1, 0, 1, 0)

	// Determine which chain is the invalid chain and which is the other chain
	// We'll perform all operations on chains A and B in the same order but will
	// include the invalid message on one chain or the other.
	initialInvalidChain := actors.ChainB
	otherChain := actors.ChainA
	if reverse {
		initialInvalidChain = actors.ChainA
		otherChain = actors.ChainB
	}

	// Create a scenario where ChainB has a block with an invalid message and ChainA references it.
	// The blocks on both chains should be marked as invalid and replaced.
	var (
		otherChainExecTx   *dsl.GeneratedTransaction
		invalidExecTx      *dsl.GeneratedTransaction
		invalidChainInitTx *dsl.GeneratedTransaction
	)
	{
		actors.ChainA.Sequencer.ActL2StartBlock(t)
		actors.ChainB.Sequencer.ActL2StartBlock(t)

		otherChainInitTx := emitterContract.EmitMessage(alice, "valid message on other chain")(otherChain)
		otherChainInitTx.Include()

		// Create messages with a conflicting payload on chain B, while also emitting an initiating message
		invalidExecTx = system.InboxContract.Execute(alice, otherChainInitTx,
			dsl.WithPayload([]byte("this message was never emitted")))(initialInvalidChain)
		invalidExecTx.Include()
		invalidChainInitTx = emitterContract.EmitMessage(alice, "valid message on invalid chain")(initialInvalidChain)
		invalidChainInitTx.Include()

		// Create a message with a valid message on chain A, pointing to the initiating message on B from the same block
		// as an invalid message.
		otherChainExecTx = system.InboxContract.Execute(alice, invalidChainInitTx)(otherChain)
		otherChainExecTx.Include()

		actors.ChainA.Sequencer.ActL2EndBlock(t)
		actors.ChainB.Sequencer.ActL2EndBlock(t)
		assertHeads(t, actors.ChainA, 2, 0, 1, 0)
		assertHeads(t, actors.ChainB, 2, 0, 1, 0)
	}

	// Submit the data to the L1
	// Now the local-safe head of both chains is 1 but the cross-safe head is still 0
	system.SubmitBatchData(func(opts *dsl.SubmitBatchDataOpts) {
		opts.SkipCrossSafeUpdate = true
	})
	assertHeads(t, actors.ChainA, 2, 2, 1, 0)
	assertHeads(t, actors.ChainB, 2, 2, 1, 0)

	// Get the current local-safe blocks on both chains; these should be the invalid blocks
	statusA, statusB := actors.ChainA.Sequencer.SyncStatus(), actors.ChainB.Sequencer.SyncStatus()
	initialChainABlock := statusA.LocalSafeL2
	initialChainBBlock := statusB.LocalSafeL2

	// Sync the supervisor and process the full state
	actors.ChainA.Sequencer.SyncSupervisor(t)
	actors.ChainB.Sequencer.SyncSupervisor(t)
	actors.Supervisor.ProcessFull(t)
	actors.ChainA.Sequencer.ActL2PipelineFull(t)
	actors.ChainA.Sequencer.SyncSupervisor(t)
	actors.ChainB.Sequencer.ActL2PipelineFull(t)

	// Process the full state again to ensure all events have been processed
	actors.Supervisor.ProcessFull(t)
	actors.ChainB.Sequencer.SyncSupervisor(t)
	actors.ChainB.Sequencer.ActL2PipelineFull(t)

	// Assert that the local-safe blocks have been replaced on both chains and all heads are synced
	// The blocks should have the same number but different hashes
	statusA, statusB = actors.ChainA.Sequencer.SyncStatus(), actors.ChainB.Sequencer.SyncStatus()
	require.Equal(t, initialChainABlock.Number, statusA.LocalSafeL2.Number)
	require.Equal(t, initialChainBBlock.Number, statusB.LocalSafeL2.Number)
	require.NotEqual(t, initialChainABlock.Hash, statusA.LocalSafeL2.Hash)
	require.NotEqual(t, initialChainBBlock.Hash, statusB.LocalSafeL2.Hash)
	assertHeads(t, actors.ChainA, 2, 2, 2, 2)
	assertHeads(t, actors.ChainB, 2, 2, 2, 2)

	// Assert that the invalid message txs were reorged out
	invalidExecTx.CheckNotIncluded()
	invalidChainInitTx.CheckNotIncluded() // Should have been reorged out with chainBExecTx
	otherChainExecTx.CheckNotIncluded()   // Reorged out because chainBInitTx was reorged out
}
