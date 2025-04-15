package interop

import (
	"context"
	"math/rand"
	"testing"

	"github.com/ethereum-optimism/optimism/devnet-sdk/contracts/bindings"
	"github.com/ethereum-optimism/optimism/devnet-sdk/contracts/constants"
	"github.com/ethereum-optimism/optimism/op-acceptance-tests/tests/interop"
	"github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/actions/interop/dsl"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum-optimism/optimism/op-service/txintent"
	"github.com/ethereum-optimism/optimism/op-service/txplan"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

// BlockBuilder helps txplan to be integrated with intra block building functionality.
type BlockBuilder struct {
	t                  helpers.Testing
	sc                 *sources.EthClient
	chain              *dsl.Chain
	signer             types.Signer
	intraBlockReceipts []*types.Receipt
	receipts           []*types.Receipt
	keepBlockOpen      bool
}

func (b *BlockBuilder) SendTransaction(ctx context.Context, tx *types.Transaction) error {
	// we need low level interaction here
	// do not submit transactions via RPC, instead directly interact with block builder
	from, err := types.Sender(b.signer, tx)
	if err != nil {
		return err
	}
	intraBlockReceipt, err := b.chain.SequencerEngine.EngineApi.IncludeTx(tx, from)
	if err == nil {
		// be aware that this receipt is not finalized...
		// which means its info may be incorrect, such as block hash
		// you must call ActL2EndBlock to seal the L2 block
		b.intraBlockReceipts = append(b.intraBlockReceipts, intraBlockReceipt)
	}
	return err
}

func (b *BlockBuilder) TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error) {
	if !b.keepBlockOpen {
		// close l2 block before fetching actual receipt
		b.chain.Sequencer.ActL2EndBlock(b.t)
		b.keepBlockOpen = false
	}
	// retrospectively fill in all resulting receipts after sealing block
	for _, intraBlockReceipt := range b.intraBlockReceipts {
		receipt, _ := b.sc.TransactionReceipt(ctx, intraBlockReceipt.TxHash)
		b.receipts = append(b.receipts, receipt)
	}
	receipt, err := b.sc.TransactionReceipt(ctx, txHash)
	if err == nil {
		b.receipts = append(b.receipts, receipt)
	}
	return receipt, err
}

func TestTxPlanDeployEventLogger(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)

	is := dsl.SetupInterop(t)
	actors := is.CreateActors()
	actors.PrepareChainState(t)

	aliceA := setupUser(t, is, actors.ChainA, 0)

	nonce := uint64(0)
	opts1, builder1 := DefaultTxOptsWithoutBlockSeal(t, aliceA, actors.ChainA, nonce)
	actors.ChainA.Sequencer.ActL2StartBlock(t)

	deployCalldata := common.FromHex(bindings.EventloggerBin)
	// tx submitted but not sealed in block
	deployTxWithoutSeal := txplan.NewPlannedTx(opts1, txplan.WithData(deployCalldata))
	_, err := deployTxWithoutSeal.Submitted.Eval(t.Ctx())
	require.NoError(t, err)
	latestBlock, err := deployTxWithoutSeal.AgainstBlock.Eval(t.Ctx())
	require.NoError(t, err)

	opts2, builder2 := DefaultTxOpts(t, aliceA, actors.ChainA)
	// manually set nonce because we cannot use the pending nonce
	opts2 = txplan.Combine(opts2, txplan.WithStaticNonce(nonce+1))

	deployTx := txplan.NewPlannedTx(opts2, txplan.WithData(deployCalldata))

	receipt, err := deployTx.Included.Eval(t.Ctx())
	require.NoError(t, err)
	// now the tx is actually included in L2 block, as well as included the tx submitted before
	// tx submitted and sealed in block

	// all intermediate receipts / finalized receipt must contain the contractAddress field
	// because they all deployed contract
	require.NotNil(t, receipt.ContractAddress)
	require.Equal(t, 1, len(builder1.intraBlockReceipts))
	require.Equal(t, 1, len(builder2.intraBlockReceipts))
	require.NotNil(t, builder1.intraBlockReceipts[0].ContractAddress)
	require.NotNil(t, builder2.intraBlockReceipts[0].ContractAddress)

	// different nonce so different contract address
	require.NotEqual(t, builder1.intraBlockReceipts[0].ContractAddress, builder2.intraBlockReceipts[0].ContractAddress)
	// second and the finalized contract address must be equal
	require.Equal(t, builder2.intraBlockReceipts[0].ContractAddress, receipt.ContractAddress)

	includedBlock, err := deployTx.IncludedBlock.Eval(t.Ctx())
	require.NoError(t, err)

	// single block advanced
	require.Equal(t, latestBlock.NumberU64()+1, includedBlock.Number)
}

func DefaultTxOpts(t helpers.Testing, user *userWithKeys, chain *dsl.Chain) (txplan.Option, *BlockBuilder) {
	sc := chain.SequencerEngine.SourceClient(t, 10)
	signer := types.LatestSignerForChainID(chain.ChainID.ToBig())
	builder := &BlockBuilder{t: t, chain: chain, sc: sc, signer: signer}
	// txplan options for tx submission and ensuring block inclusion
	return txplan.Combine(
		txplan.WithPrivateKey(user.secret),
		txplan.WithChainID(sc),
		txplan.WithAgainstLatestBlock(sc),
		txplan.WithPendingNonce(sc),
		txplan.WithEstimator(sc, false),
		txplan.WithTransactionSubmitter(builder),
		txplan.WithAssumedInclusion(builder),
		txplan.WithBlockInclusionInfo(sc),
	), builder
}

func DefaultTxOptsWithoutBlockSeal(t helpers.Testing, user *userWithKeys, chain *dsl.Chain, nonce uint64) (txplan.Option, *BlockBuilder) {
	sc := chain.SequencerEngine.SourceClient(t, 10)
	signer := types.LatestSignerForChainID(chain.ChainID.ToBig())
	builder := &BlockBuilder{t: t, chain: chain, sc: sc, keepBlockOpen: true, signer: signer}
	return txplan.Combine(
		txplan.WithPrivateKey(user.secret),
		txplan.WithChainID(sc),
		txplan.WithAgainstLatestBlock(sc),
		// nonce must be manually set since pending nonce may be incorrect
		txplan.WithNonce(nonce),
		txplan.WithEstimator(sc, false),
		txplan.WithTransactionSubmitter(builder),
		txplan.WithAssumedInclusion(builder),
		txplan.WithBlockInclusionInfo(sc),
	), builder
}

func DeployEventLogger(t helpers.Testing, opts txplan.Option) common.Address {
	deployCalldata := common.FromHex(bindings.EventloggerBin)
	deployTx := txplan.NewPlannedTx(opts, txplan.WithData(deployCalldata))
	receipt, err := deployTx.Included.Eval(t.Ctx())
	require.NoError(t, err)
	require.NotNil(t, receipt.ContractAddress)
	eventLoggerAddress := receipt.ContractAddress
	return eventLoggerAddress
}

func consolidateToSafe(t helpers.Testing, actors *dsl.InteropActors, startA, startB, endA, endB uint64) {
	// Batch L2 blocks of chain A, B and submit to L1 to ensure safe head advances without a reorg.
	// Checking cross-unsafe consolidation is sufficient for sanity check but lets add safe check as well.
	actors.ChainA.Batcher.ActSubmitAll(t)
	actors.ChainB.Batcher.ActSubmitAll(t)
	actors.L1Miner.ActL1StartBlock(12)(t)
	actors.L1Miner.ActL1IncludeTx(actors.ChainA.BatcherAddr)(t)
	actors.L1Miner.ActL1IncludeTx(actors.ChainB.BatcherAddr)(t)
	actors.L1Miner.ActL1EndBlock(t)

	actors.Supervisor.SignalLatestL1(t)

	t.Log("awaiting L1-exhaust event")
	actors.ChainA.Sequencer.ActL2PipelineFull(t)
	actors.ChainB.Sequencer.ActL2PipelineFull(t)
	assertHeads(t, actors.ChainA, endA, startA, startA, startA)
	assertHeads(t, actors.ChainB, endB, startB, startB, startB)

	t.Log("awaiting supervisor to provide L1 data")
	actors.ChainA.Sequencer.SyncSupervisor(t)
	actors.ChainB.Sequencer.SyncSupervisor(t)
	assertHeads(t, actors.ChainA, endA, startA, startA, startA)
	assertHeads(t, actors.ChainB, endB, startB, startB, startB)

	t.Log("awaiting node to sync: unsafe to local-safe")
	actors.ChainA.Sequencer.ActL2PipelineFull(t)
	actors.ChainB.Sequencer.ActL2PipelineFull(t)
	assertHeads(t, actors.ChainA, endA, endA, startA, startA)
	assertHeads(t, actors.ChainB, endB, endB, startB, startB)

	t.Log("expecting supervisor to sync")
	actors.ChainA.Sequencer.SyncSupervisor(t)
	actors.ChainB.Sequencer.SyncSupervisor(t)
	assertHeads(t, actors.ChainA, endA, endA, startA, startA)
	assertHeads(t, actors.ChainB, endB, endB, startB, startB)

	t.Log("supervisor promotes cross-unsafe and safe")
	actors.Supervisor.ProcessFull(t)

	t.Log("awaiting nodes to sync: local-safe to safe")
	actors.ChainA.Sequencer.ActL2PipelineFull(t)
	actors.ChainB.Sequencer.ActL2PipelineFull(t)

	assertHeads(t, actors.ChainA, endA, endA, endA, endA)
	assertHeads(t, actors.ChainB, endB, endB, endB, endB)
}

// reorgOutUnsafeAndConsolidateToSafe assume that chainY is reorged, but chainX is not.
// chainY is expected to experience cross-unsafe invalidation and reorging unsafe blocks.
// Consolidate with steps: unsafe -> cross-unsafe -> local-safe -> safe
func reorgOutUnsafeAndConsolidateToSafe(t helpers.Testing, actors *dsl.InteropActors, chainX, chainY *dsl.Chain, startX, startY, endX, endY, unsafeHeadNumAfterReorg uint64) {
	require.GreaterOrEqual(t, endY, unsafeHeadNumAfterReorg)
	// Check to make batcher happy
	require.Positive(t, endY-startY)
	require.Positive(t, endX-startX)

	chainX.Sequencer.ActL2PipelineFull(t)
	chainY.Sequencer.ActL2PipelineFull(t)
	chainX.Sequencer.SyncSupervisor(t)
	chainY.Sequencer.SyncSupervisor(t)
	actors.Supervisor.ProcessFull(t)
	chainX.Sequencer.ActL2PipelineFull(t)
	chainY.Sequencer.ActL2PipelineFull(t)

	assertHeads(t, chainX, endX, startX, endX, startX)
	assertHeads(t, chainY, endY, startY, unsafeHeadNumAfterReorg, startY)

	// check chain Y and and supervisor view of chain Y is consistent
	reorgedOutBlock := chainY.Sequencer.SyncStatus().UnsafeL2
	require.Equal(t, unsafeHeadNumAfterReorg+1, reorgedOutBlock.Number)
	localUnsafe, err := actors.Supervisor.LocalUnsafe(t.Ctx(), chainY.ChainID)
	require.NoError(t, err)
	require.Equal(t, reorgedOutBlock.ID(), localUnsafe)

	// now try to advance safe heads
	chainX.Batcher.ActSubmitAll(t)
	chainY.Batcher.ActSubmitAll(t)
	actors.L1Miner.ActL1StartBlock(12)(t)
	actors.L1Miner.ActL1IncludeTx(chainX.BatcherAddr)(t)
	actors.L1Miner.ActL1IncludeTx(chainY.BatcherAddr)(t)
	actors.L1Miner.ActL1EndBlock(t)

	actors.Supervisor.SignalLatestL1(t)

	t.Log("awaiting L1-exhaust event")
	chainX.Sequencer.ActL2PipelineFull(t)
	chainY.Sequencer.ActL2PipelineFull(t)
	assertHeads(t, chainX, endX, startX, endX, startX)
	assertHeads(t, chainY, endY, startY, unsafeHeadNumAfterReorg, startY)

	t.Log("awaiting supervisor to provide L1 data")
	chainX.Sequencer.SyncSupervisor(t)
	chainY.Sequencer.SyncSupervisor(t)
	assertHeads(t, chainX, endX, startX, endX, startX)
	assertHeads(t, chainY, endY, startY, unsafeHeadNumAfterReorg, startY)

	t.Log("awaiting node to sync: unsafe to local-safe")
	chainX.Sequencer.ActL2PipelineFull(t)
	chainY.Sequencer.ActL2PipelineFull(t)
	assertHeads(t, chainX, endX, endX, endX, startX)
	assertHeads(t, chainY, endY, endY, unsafeHeadNumAfterReorg, startY)

	t.Log("expecting supervisor to sync")
	chainX.Sequencer.SyncSupervisor(t)
	chainY.Sequencer.SyncSupervisor(t)
	assertHeads(t, chainX, endX, endX, endX, startX)
	assertHeads(t, chainY, endY, endY, unsafeHeadNumAfterReorg, startY)

	t.Log("supervisor promotes cross-unsafe and safe")
	actors.Supervisor.ProcessFull(t)

	// check supervisor head, expect it to be rewound
	localUnsafe, err = actors.Supervisor.LocalUnsafe(t.Ctx(), chainY.ChainID)
	require.NoError(t, err)
	require.Equal(t, unsafeHeadNumAfterReorg, localUnsafe.Number, "unsafe chain needs to be rewound")

	t.Log("awaiting nodes to sync: local-safe to safe")
	chainX.Sequencer.ActL2PipelineFull(t)
	chainY.Sequencer.ActL2PipelineFull(t)

	assertHeads(t, chainX, endX, endX, endX, endX)
	assertHeads(t, chainY, endY, endY, endY, endY)

	// Make sure the replaced block has different blockhash
	replacedBlock := chainY.Sequencer.SyncStatus().LocalSafeL2
	require.NotEqual(t, reorgedOutBlock.Hash, replacedBlock.Hash)
	require.Equal(t, reorgedOutBlock.Number, replacedBlock.Number)
	require.Equal(t, unsafeHeadNumAfterReorg+1, replacedBlock.Number)
}

func TestInitAndExecMsgSameTimestamp(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)
	rng := rand.New(rand.NewSource(1234))
	is := dsl.SetupInterop(t)
	actors := is.CreateActors()
	actors.PrepareChainState(t)
	alice := setupUser(t, is, actors.ChainA, 0)
	bob := setupUser(t, is, actors.ChainB, 0)

	optsA, _ := DefaultTxOpts(t, alice, actors.ChainA)
	optsB, _ := DefaultTxOpts(t, bob, actors.ChainB)
	actors.ChainA.Sequencer.ActL2StartBlock(t)

	// chain A progressed single unsafe block
	eventLoggerAddress := DeployEventLogger(t, optsA)
	// Also match chain B
	actors.ChainB.Sequencer.ActL2EmptyBlock(t)

	// Intent to initiate message(or emit event) on chain A
	txA := txintent.NewIntent[*txintent.InitTrigger, *txintent.InteropOutput](optsA)
	randomInitTrigger := interop.RandomInitTrigger(rng, eventLoggerAddress, 3, 10)
	txA.Content.Set(randomInitTrigger)

	// Trigger single event
	actors.ChainA.Sequencer.ActL2StartBlock(t)
	_, err := txA.PlannedTx.Included.Eval(t.Ctx())
	require.NoError(t, err)

	assertHeads(t, actors.ChainA, 2, 0, 0, 0)

	// Ingest the new unsafe-block event
	actors.ChainA.Sequencer.SyncSupervisor(t)
	// Verify as cross-unsafe with supervisor
	actors.Supervisor.ProcessFull(t)
	actors.ChainA.Sequencer.ActL2PipelineFull(t)
	assertHeads(t, actors.ChainA, 2, 0, 2, 0)
	assertHeads(t, actors.ChainB, 1, 0, 0, 0)

	// Ingest the new unsafe-block event
	actors.ChainB.Sequencer.SyncSupervisor(t)
	// Verify as cross-unsafe with supervisor
	actors.Supervisor.ProcessFull(t)
	actors.ChainB.Sequencer.ActL2PipelineFull(t)
	assertHeads(t, actors.ChainB, 1, 0, 1, 0)

	// Intent to validate message on chain B
	txB := txintent.NewIntent[*txintent.ExecTrigger, *txintent.InteropOutput](optsB)
	txB.Content.DependOn(&txA.Result)

	// Single event in tx so index is 0
	txB.Content.Fn(txintent.ExecuteIndexed(constants.CrossL2Inbox, &txA.Result, 0))

	actors.ChainB.Sequencer.ActL2StartBlock(t)
	_, err = txB.PlannedTx.Included.Eval(t.Ctx())
	require.NoError(t, err)

	includedA, err := txA.PlannedTx.IncludedBlock.Eval(t.Ctx())
	require.NoError(t, err)
	includedB, err := txB.PlannedTx.IncludedBlock.Eval(t.Ctx())
	require.NoError(t, err)

	// initiating messages time <= executing message time
	require.Equal(t, includedA.Time, includedB.Time)

	assertHeads(t, actors.ChainB, 2, 0, 1, 0)

	// Ingest the new unsafe-block event
	actors.ChainB.Sequencer.SyncSupervisor(t)
	// Verify as cross-unsafe with supervisor
	actors.Supervisor.ProcessFull(t)
	actors.ChainB.Sequencer.ActL2PipelineFull(t)

	assertHeads(t, actors.ChainB, 2, 0, 2, 0)
}

func TestBreakTimestampInvariant(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)
	rng := rand.New(rand.NewSource(1234))
	is := dsl.SetupInterop(t)
	actors := is.CreateActors()
	actors.PrepareChainState(t)

	alice := setupUser(t, is, actors.ChainA, 0)
	bob := setupUser(t, is, actors.ChainB, 0)

	optsA, _ := DefaultTxOpts(t, alice, actors.ChainA)
	optsB, _ := DefaultTxOpts(t, bob, actors.ChainB)
	actors.ChainA.Sequencer.ActL2StartBlock(t)
	// chain A progressed single unsafe block
	eventLoggerAddress := DeployEventLogger(t, optsA)

	// Intent to initiate message(or emit event) on chain A
	txA := txintent.NewIntent[*txintent.InitTrigger, *txintent.InteropOutput](optsA)
	randomInitTrigger := interop.RandomInitTrigger(rng, eventLoggerAddress, 3, 10)
	txA.Content.Set(randomInitTrigger)

	// Trigger single event
	actors.ChainA.Sequencer.ActL2StartBlock(t)
	_, err := txA.PlannedTx.Included.Eval(t.Ctx())
	require.NoError(t, err)
	actors.ChainA.Sequencer.ActL2PipelineFull(t)
	assertHeads(t, actors.ChainA, 2, 0, 0, 0)

	// make supervisor know chainA's unsafe blocks
	actors.ChainA.Sequencer.SyncSupervisor(t)

	// Intent to validate message on chain B
	txB := txintent.NewIntent[*txintent.ExecTrigger, *txintent.InteropOutput](optsB)
	txB.Content.DependOn(&txA.Result)

	// Single event in tx so index is 0
	txB.Content.Fn(txintent.ExecuteIndexed(constants.CrossL2Inbox, &txA.Result, 0))

	actors.ChainB.Sequencer.ActL2StartBlock(t)
	_, err = txB.PlannedTx.Included.Eval(t.Ctx())
	require.NoError(t, err)
	actors.ChainB.Sequencer.ActL2PipelineFull(t)
	assertHeads(t, actors.ChainB, 1, 0, 0, 0)

	includedA, err := txA.PlannedTx.IncludedBlock.Eval(t.Ctx())
	require.NoError(t, err)
	includedB, err := txB.PlannedTx.IncludedBlock.Eval(t.Ctx())
	require.NoError(t, err)

	// initiating messages time <= executing message time
	// BUT we intentionally break the timestamp invariant
	require.Greater(t, includedA.Time, includedB.Time)

	assertHeads(t, actors.ChainB, 1, 0, 0, 0)

	actors.ChainB.Batcher.ActSubmitAll(t)
	actors.L1Miner.ActL1StartBlock(12)(t)
	actors.L1Miner.ActL1IncludeTx(actors.ChainB.BatcherAddr)(t)
	actors.L1Miner.ActL1EndBlock(t)

	actors.Supervisor.SignalLatestL1(t)
	t.Log("awaiting L1-exhaust event")
	actors.ChainB.Sequencer.ActL2PipelineFull(t)
	t.Log("awaiting supervisor to provide L1 data")
	actors.ChainB.Sequencer.SyncSupervisor(t)
	actors.Supervisor.ProcessFull(t)
	t.Log("awaiting node to sync")
	actors.ChainB.Sequencer.ActL2PipelineFull(t)

	reorgedOutBlock := actors.ChainB.Sequencer.SyncStatus().LocalSafeL2
	require.Equal(t, uint64(1), reorgedOutBlock.Number)

	t.Log("Expecting supervisor to sync and catch local-safe dependency issue")
	actors.ChainB.Sequencer.SyncSupervisor(t)
	actors.Supervisor.ProcessFull(t)

	assertHeads(t, actors.ChainB, 1, 1, 0, 0)

	// check supervisor head, expect it to be rewound
	localUnsafe, err := actors.Supervisor.LocalUnsafe(t.Ctx(), actors.ChainB.ChainID)
	require.NoError(t, err)
	require.Equal(t, uint64(0), localUnsafe.Number, "unsafe chain needs to be rewound")

	// Make the op-node do the processing to build the replacement
	t.Log("Expecting op-node to build replacement block")
	actors.ChainB.Sequencer.ActL2PipelineFull(t)
	actors.ChainB.Sequencer.SyncSupervisor(t)
	actors.Supervisor.ProcessFull(t)

	// Make sure the replaced block has different blockhash
	replacedBlock := actors.ChainB.Sequencer.SyncStatus().LocalSafeL2
	require.NotEqual(t, reorgedOutBlock.Hash, replacedBlock.Hash)

	// but reached block number as 1
	assertHeads(t, actors.ChainB, 1, 1, 1, 1)
}

// TestExecMsgDifferTxIndex tests below scenario:
// Execute message that links with initiating message with: first, random or last tx of a block.
func TestExecMsgDifferTxIndex(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)
	rng := rand.New(rand.NewSource(1234))
	is := dsl.SetupInterop(t)
	actors := is.CreateActors()
	actors.PrepareChainState(t)

	// only unsafe head of each chain progresses in this code block
	var targetNum uint64
	{
		alice := setupUser(t, is, actors.ChainA, 0)
		bob := setupUser(t, is, actors.ChainB, 0)

		optsA, _ := DefaultTxOpts(t, alice, actors.ChainA)
		optsB, _ := DefaultTxOpts(t, bob, actors.ChainB)
		actors.ChainA.Sequencer.ActL2StartBlock(t)
		// chain A progressed single unsafe block
		eventLoggerAddress := DeployEventLogger(t, optsA)

		// attempt to include multiple txs in a single L2 block
		actors.ChainA.Sequencer.ActL2StartBlock(t)
		// start with nonce as 1 because alice deployed the EventLogger

		nonce := uint64(1)
		txCount := 3 + rng.Intn(15)
		initTxs := []*txintent.IntentTx[*txintent.InitTrigger, *txintent.InteropOutput]{}
		for range txCount {
			opts, _ := DefaultTxOptsWithoutBlockSeal(t, alice, actors.ChainA, nonce)

			// Intent to initiate message(or emit event) on chain A
			tx := txintent.NewIntent[*txintent.InitTrigger, *txintent.InteropOutput](opts)
			initTxs = append(initTxs, tx)
			randomInitTrigger := interop.RandomInitTrigger(rng, eventLoggerAddress, 3, 10)
			tx.Content.Set(randomInitTrigger)

			// Trigger single event
			_, err := tx.PlannedTx.Submitted.Eval(t.Ctx())
			require.NoError(t, err)

			nonce += 1
		}
		actors.ChainA.Sequencer.ActL2EndBlock(t)

		// fetch receipts since all txs are included in the block and sealed
		for _, tx := range initTxs {
			includedBlock, err := tx.PlannedTx.IncludedBlock.Eval(t.Ctx())
			require.NoError(t, err)
			// all txCount txs are included at same block
			require.Equal(t, uint64(2), includedBlock.Number)
		}
		assertHeads(t, actors.ChainA, 2, 0, 0, 0)

		// advance chain B for satisfying the timestamp invariant
		actors.ChainB.Sequencer.ActL2EmptyBlock(t)
		assertHeads(t, actors.ChainB, 1, 0, 0, 0)

		// first, random or last tx of a single L2 block.
		indexes := []int{0, 1 + rng.Intn(txCount-1), txCount - 1}
		for blockNumDelta, index := range indexes {
			actors.ChainB.Sequencer.ActL2StartBlock(t)

			initTx := initTxs[index]
			execTx := txintent.NewIntent[*txintent.ExecTrigger, *txintent.InteropOutput](optsB)

			// Single event in every tx so index is always 0
			execTx.Content.Fn(txintent.ExecuteIndexed(constants.CrossL2Inbox, &initTx.Result, 0))
			execTx.Content.DependOn(&initTx.Result)

			includedBlock, err := execTx.PlannedTx.IncludedBlock.Eval(t.Ctx())
			require.NoError(t, err)

			// each block contains single executing message
			require.Equal(t, uint64(2+blockNumDelta), includedBlock.Number)
		}
		targetNum = uint64(1 + len(indexes))
		assertHeads(t, actors.ChainB, targetNum, 0, 0, 0)
	}
	// store unsafe head of chain B to compare after consolidation
	chainBUnsafeHead := actors.ChainB.Sequencer.SyncStatus().UnsafeL2
	require.Equal(t, targetNum, chainBUnsafeHead.Number)
	require.Equal(t, uint64(4), targetNum)

	consolidateToSafe(t, actors, 0, 0, 2, 4)

	// unsafe head of chain B did not get updated
	require.Equal(t, chainBUnsafeHead, actors.ChainB.Sequencer.SyncStatus().UnsafeL2)
	// unsafe head of chain B consolidated to safe
	require.Equal(t, chainBUnsafeHead, actors.ChainB.Sequencer.SyncStatus().SafeL2)
}

// TestExpiredMessage tests below scenario:
// Execute message with current timestamp > the lower-bound expiry timestamp.
func TestExpiredMessage(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)
	rng := rand.New(rand.NewSource(1234))

	expiryTime := uint64(6)
	is := dsl.SetupInterop(t, dsl.SetMessageExpiryTime(expiryTime))
	actors := is.CreateActors()
	actors.PrepareChainState(t)

	alice := setupUser(t, is, actors.ChainA, 0)
	bob := setupUser(t, is, actors.ChainB, 0)

	optsA, _ := DefaultTxOpts(t, alice, actors.ChainA)
	optsB, _ := DefaultTxOpts(t, bob, actors.ChainB)
	actors.ChainA.Sequencer.ActL2StartBlock(t)
	// chain A progressed single unsafe block
	eventLoggerAddress := DeployEventLogger(t, optsA)

	// Intent to initiate message(or emit event) on chain A
	txA := txintent.NewIntent[*txintent.InitTrigger, *txintent.InteropOutput](optsA)
	randomInitTrigger := interop.RandomInitTrigger(rng, eventLoggerAddress, 3, 10)
	txA.Content.Set(randomInitTrigger)

	// Trigger single event
	actors.ChainA.Sequencer.ActL2StartBlock(t)
	_, err := txA.PlannedTx.Included.Eval(t.Ctx())
	require.NoError(t, err)
	actors.ChainA.Sequencer.ActL2PipelineFull(t)
	assertHeads(t, actors.ChainA, 2, 0, 0, 0)

	// make supervisor know chainA's unsafe blocks
	actors.ChainA.Sequencer.SyncSupervisor(t)

	// advance chain B to reach expiry
	targetNumblocksUntilExpiry := expiryTime / actors.ChainA.RollupCfg.BlockTime
	for range 2 + targetNumblocksUntilExpiry {
		actors.ChainB.Sequencer.ActL2EmptyBlock(t)
	}
	assertHeads(t, actors.ChainB, 2+targetNumblocksUntilExpiry, 0, 0, 0)

	// check that chain B unsafe head reached tip of expiry
	require.Equal(t, actors.ChainA.Sequencer.SyncStatus().UnsafeL2.Time+expiryTime, actors.ChainB.Sequencer.SyncStatus().UnsafeL2.Time)

	// Intent to validate message on chain B
	txB := txintent.NewIntent[*txintent.ExecTrigger, *txintent.InteropOutput](optsB)
	txB.Content.DependOn(&txA.Result)

	// Single event in tx so index is 0
	txB.Content.Fn(txintent.ExecuteIndexed(constants.CrossL2Inbox, &txA.Result, 0))

	actors.ChainB.Sequencer.ActL2StartBlock(t)
	_, err = txB.PlannedTx.Included.Eval(t.Ctx())
	require.NoError(t, err)
	actors.ChainB.Sequencer.ActL2PipelineFull(t)
	expiredMsgBlockNum := 2 + targetNumblocksUntilExpiry + 1
	assertHeads(t, actors.ChainB, expiredMsgBlockNum, 0, 0, 0)

	includedA, err := txA.PlannedTx.IncludedBlock.Eval(t.Ctx())
	require.NoError(t, err)
	includedB, err := txB.PlannedTx.IncludedBlock.Eval(t.Ctx())
	require.NoError(t, err)

	// initiating messages time + expiryTime >= executing message time
	// BUT we intentionally break the message expiry invariant
	require.Greater(t, includedB.Time, includedA.Time+expiryTime)

	reorgOutUnsafeAndConsolidateToSafe(t, actors, actors.ChainA, actors.ChainB, 0, 0, 2, expiredMsgBlockNum, expiredMsgBlockNum-1)
}

// TestCrossPatternSameTimestamp tests below scenario:
// Transaction on B executes message from A, and vice-versa. Cross-pattern, within same timestamp:
// Four transactions happen in same timestamp:
// tx0: chainA: alice initiates message X
// tx1: chainB: bob executes message X
// tx2: chainB: bob initiates message Y
// tx3: chainA: alice executes message Y
func TestCrossPatternSameTimestamp(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)
	rng := rand.New(rand.NewSource(1234))
	is := dsl.SetupInterop(t)
	actors := is.CreateActors()
	actors.PrepareChainState(t)
	alice := setupUser(t, is, actors.ChainA, 0)
	bob := setupUser(t, is, actors.ChainB, 0)

	// deploy eventLogger per chain
	actors.ChainA.Sequencer.ActL2StartBlock(t)
	deployOptsA, _ := DefaultTxOpts(t, setupUser(t, is, actors.ChainA, 1), actors.ChainA)
	eventLoggerAddressA := DeployEventLogger(t, deployOptsA)
	actors.ChainB.Sequencer.ActL2StartBlock(t)
	deployOptsB, _ := DefaultTxOpts(t, setupUser(t, is, actors.ChainB, 1), actors.ChainB)
	eventLoggerAddressB := DeployEventLogger(t, deployOptsB)

	assertHeads(t, actors.ChainA, 1, 0, 0, 0)
	assertHeads(t, actors.ChainB, 1, 0, 0, 0)

	require.Equal(t, actors.ChainA.RollupCfg.Genesis.L2Time, actors.ChainB.RollupCfg.Genesis.L2Time)
	// assume all four txs land in block number 2, same time
	targetTime := actors.ChainA.RollupCfg.Genesis.L2Time + actors.ChainA.RollupCfg.BlockTime*2
	// start with nonce as 0 for both alice and bob
	nonce := uint64(0)
	optsA, builderA := DefaultTxOptsWithoutBlockSeal(t, alice, actors.ChainA, nonce)
	optsB, builderB := DefaultTxOptsWithoutBlockSeal(t, bob, actors.ChainB, nonce)

	// open blocks on both chains
	actors.ChainA.Sequencer.ActL2StartBlock(t)
	actors.ChainB.Sequencer.ActL2StartBlock(t)

	// Intent to initiate message X on chain A
	tx0 := txintent.NewIntent[*txintent.InitTrigger, *txintent.InteropOutput](optsA)
	tx0.Content.Set(interop.RandomInitTrigger(rng, eventLoggerAddressA, 3, 10))

	_, err := tx0.PlannedTx.Submitted.Eval(t.Ctx())
	require.NoError(t, err)
	// manually update the included info since block is not sealed yet
	require.NotNil(t, builderA.intraBlockReceipts[0])
	tx0.PlannedTx.Included.Set(builderA.intraBlockReceipts[0])
	tx0.PlannedTx.IncludedBlock.Set(eth.BlockRef{Time: targetTime})

	// Intent to execute message X on chain B
	tx1 := txintent.NewIntent[*txintent.ExecTrigger, *txintent.InteropOutput](optsB)
	tx1.Content.Fn(txintent.ExecuteIndexed(constants.CrossL2Inbox, &tx0.Result, 0))
	tx1.Content.DependOn(&tx0.Result)

	_, err = tx1.PlannedTx.Submitted.Eval(t.Ctx())
	require.NoError(t, err)

	// Intent to initiate message Y on chain B
	// override nonce
	optsB = txplan.Combine(optsB, txplan.WithStaticNonce(nonce+1))
	tx2 := txintent.NewIntent[*txintent.InitTrigger, *txintent.InteropOutput](optsB)
	tx2.Content.Set(interop.RandomInitTrigger(rng, eventLoggerAddressB, 4, 9))
	_, err = tx2.PlannedTx.Submitted.Eval(t.Ctx())
	require.NoError(t, err)
	// manually update the included info since block is not sealed yet
	require.NotNil(t, builderB.intraBlockReceipts[0])
	tx2.PlannedTx.Included.Set(builderB.intraBlockReceipts[0])
	tx2.PlannedTx.IncludedBlock.Set(eth.BlockRef{Time: targetTime})

	// Intent to execute message Y on chain A
	// override nonce
	optsA = txplan.Combine(optsA, txplan.WithStaticNonce(nonce+1))
	tx3 := txintent.NewIntent[*txintent.ExecTrigger, *txintent.InteropOutput](optsA)
	tx3.Content.Fn(txintent.ExecuteIndexed(constants.CrossL2Inbox, &tx2.Result, 0))
	tx3.Content.DependOn(&tx2.Result)

	_, err = tx3.PlannedTx.Submitted.Eval(t.Ctx())
	require.NoError(t, err)

	// finally seal block
	actors.ChainA.Sequencer.ActL2EndBlock(t)
	actors.ChainB.Sequencer.ActL2EndBlock(t)
	assertHeads(t, actors.ChainA, 2, 0, 0, 0)
	assertHeads(t, actors.ChainB, 2, 0, 0, 0)

	// store unsafe head of chain A, B to compare after consolidation
	chainAUnsafeHead := actors.ChainA.Sequencer.SyncStatus().UnsafeL2
	chainBUnsafeHead := actors.ChainB.Sequencer.SyncStatus().UnsafeL2

	consolidateToSafe(t, actors, 0, 0, 2, 2)

	// unsafe heads consolidated to safe
	require.Equal(t, chainAUnsafeHead, actors.ChainA.Sequencer.SyncStatus().SafeL2)
	require.Equal(t, chainBUnsafeHead, actors.ChainB.Sequencer.SyncStatus().SafeL2)

	t.Log("Check that all tx included blocks and receipts can be fetched using the RPC")
	targetBlockNum := uint64(2)

	// check tx1 first instead of tx0 not to make tx0 submitted
	receipt, err := tx1.PlannedTx.Included.Eval(t.Ctx())
	require.NoError(t, err)
	require.Equal(t, targetBlockNum, receipt.BlockNumber.Uint64())
	block, err := tx1.PlannedTx.IncludedBlock.Eval(t.Ctx())
	require.NoError(t, err)
	require.Equal(t, targetTime, block.Time)

	// invalidate receipt which is incorrect since it was fetched intra block
	tx0.PlannedTx.Included.Invalidate()
	tx0.PlannedTx.IncludedBlock.Invalidate()
	block, err = tx0.PlannedTx.IncludedBlock.Eval(t.Ctx())
	require.NoError(t, err)
	require.Equal(t, targetBlockNum, block.Number)
	require.Equal(t, targetTime, block.Time)

	// check tx3 first instead of tx2 not to make tx2 submitted
	receipt, err = tx3.PlannedTx.Included.Eval(t.Ctx())
	require.NoError(t, err)
	require.Equal(t, targetBlockNum, receipt.BlockNumber.Uint64())
	block, err = tx3.PlannedTx.IncludedBlock.Eval(t.Ctx())
	require.NoError(t, err)
	require.Equal(t, targetTime, block.Time)

	// invalidate receipt which is incorrect since it was fetched intra block
	tx2.PlannedTx.Included.Invalidate()
	tx2.PlannedTx.IncludedBlock.Invalidate()
	block, err = tx2.PlannedTx.IncludedBlock.Eval(t.Ctx())
	require.NoError(t, err)
	require.Equal(t, targetBlockNum, block.Number)
	require.Equal(t, targetTime, block.Time)
}

// TestCrossPatternSameTx tests below scenario:
// Transaction on B executes message from A, and vice-versa. Cross-pattern, within same tx: inter-dependent but non-cyclic txs.
// Two transactions happen in same timestamp:
// txA: chain A: alice initiates message X and executes message Y
// txB: chain B: bob initiates message Y and executes message X
func TestCrossPatternSameTx(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)
	rng := rand.New(rand.NewSource(1234))
	is := dsl.SetupInterop(t)
	actors := is.CreateActors()
	actors.PrepareChainState(t)
	alice := setupUser(t, is, actors.ChainA, 0)
	bob := setupUser(t, is, actors.ChainB, 0)

	// deploy eventLogger per chain
	actors.ChainA.Sequencer.ActL2StartBlock(t)
	deployOptsA, _ := DefaultTxOpts(t, setupUser(t, is, actors.ChainA, 1), actors.ChainA)
	eventLoggerAddressA := DeployEventLogger(t, deployOptsA)
	actors.ChainB.Sequencer.ActL2StartBlock(t)
	deployOptsB, _ := DefaultTxOpts(t, setupUser(t, is, actors.ChainB, 1), actors.ChainB)
	eventLoggerAddressB := DeployEventLogger(t, deployOptsB)

	assertHeads(t, actors.ChainA, 1, 0, 0, 0)
	assertHeads(t, actors.ChainB, 1, 0, 0, 0)

	require.Equal(t, actors.ChainA.RollupCfg.Genesis.L2Time, actors.ChainB.RollupCfg.Genesis.L2Time)
	// assume all two txs land in block number 2, same time
	targetTime := actors.ChainA.RollupCfg.Genesis.L2Time + actors.ChainA.RollupCfg.BlockTime*2
	targetNum := uint64(2)
	optsA, _ := DefaultTxOpts(t, alice, actors.ChainA)
	optsB, _ := DefaultTxOpts(t, bob, actors.ChainB)

	// open blocks on both chains
	actors.ChainA.Sequencer.ActL2StartBlock(t)
	actors.ChainB.Sequencer.ActL2StartBlock(t)

	// speculatively build exec messages by knowing necessary info to build Message
	initX := interop.RandomInitTrigger(rng, eventLoggerAddressA, 3, 10)
	logIndexX, logIndexY := uint(0), uint(0)
	execX, err := interop.ExecTriggerFromInitTrigger(initX, logIndexX, targetNum, targetTime, actors.ChainA.ChainID)
	require.NoError(t, err)
	initY := interop.RandomInitTrigger(rng, eventLoggerAddressB, 4, 7)
	execY, err := interop.ExecTriggerFromInitTrigger(initY, logIndexY, targetNum, targetTime, actors.ChainB.ChainID)
	require.NoError(t, err)

	callsA := []txintent.Call{initX, execY}
	callsB := []txintent.Call{initY, execX}

	// Intent to initiate message X and execute message Y at chain A
	txA := txintent.NewIntent[*txintent.MultiTrigger, *txintent.InteropOutput](optsA)
	txA.Content.Set(&txintent.MultiTrigger{Emitter: constants.MultiCall3, Calls: callsA})
	// Intent to initiate message Y and execute message X at chain B
	txB := txintent.NewIntent[*txintent.MultiTrigger, *txintent.InteropOutput](optsB)
	txB.Content.Set(&txintent.MultiTrigger{Emitter: constants.MultiCall3, Calls: callsB})

	includedA, err := txA.PlannedTx.IncludedBlock.Eval(t.Ctx())
	require.NoError(t, err)
	includedB, err := txB.PlannedTx.IncludedBlock.Eval(t.Ctx())
	require.NoError(t, err)

	// Make sure two txs both sealed in block at expected time
	require.Equal(t, includedA.Time, targetTime)
	require.Equal(t, includedA.Number, targetNum)
	require.Equal(t, includedB.Time, targetTime)
	require.Equal(t, includedB.Number, targetNum)

	assertHeads(t, actors.ChainA, targetNum, 0, 0, 0)
	assertHeads(t, actors.ChainB, targetNum, 0, 0, 0)

	// confirm speculatively built exec message X by rebuilding after txA inclusion
	_, err = txA.Result.Eval(t.Ctx())
	require.NoError(t, err)
	multiTriggerA, err := txintent.ExecuteIndexeds(constants.MultiCall3, constants.CrossL2Inbox, &txA.Result, []int{int(logIndexX)})(t.Ctx())
	require.NoError(t, err)
	require.Equal(t, multiTriggerA.Calls[logIndexX], execX)

	// confirm speculatively built exec message Y by rebuilding after txB inclusion
	_, err = txB.Result.Eval(t.Ctx())
	require.NoError(t, err)
	multiTriggerB, err := txintent.ExecuteIndexeds(constants.MultiCall3, constants.CrossL2Inbox, &txB.Result, []int{int(logIndexY)})(t.Ctx())
	require.NoError(t, err)
	require.Equal(t, multiTriggerB.Calls[logIndexY], execY)

	// store unsafe head of chain A, B to compare after consolidation
	chainAUnsafeHead := actors.ChainA.Sequencer.SyncStatus().UnsafeL2
	chainBUnsafeHead := actors.ChainB.Sequencer.SyncStatus().UnsafeL2

	consolidateToSafe(t, actors, 0, 0, targetNum, targetNum)

	// unsafe heads consolidated to safe
	require.Equal(t, chainAUnsafeHead, actors.ChainA.Sequencer.SyncStatus().SafeL2)
	require.Equal(t, chainBUnsafeHead, actors.ChainB.Sequencer.SyncStatus().SafeL2)
}

// TestCycleInTx tests below scenario:
// Transaction executes message, then initiates it: cycle with self
func TestCycleInTx(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)
	rng := rand.New(rand.NewSource(1234))
	is := dsl.SetupInterop(t)
	actors := is.CreateActors()
	actors.PrepareChainState(t)
	alice := setupUser(t, is, actors.ChainA, 0)

	actors.ChainA.Sequencer.ActL2StartBlock(t)
	deployOptsA, _ := DefaultTxOpts(t, setupUser(t, is, actors.ChainA, 1), actors.ChainA)
	eventLoggerAddressA := DeployEventLogger(t, deployOptsA)

	assertHeads(t, actors.ChainA, 1, 0, 0, 0)

	// assume tx which multicalls exec message and init message land in block number 2
	targetTime := actors.ChainA.RollupCfg.Genesis.L2Time + actors.ChainA.RollupCfg.BlockTime*2
	targetNum := uint64(2)
	optsA, _ := DefaultTxOpts(t, alice, actors.ChainA)

	// open blocks
	actors.ChainA.Sequencer.ActL2StartBlock(t)

	// speculatively build exec message by knowing necessary info to build Message
	init := interop.RandomInitTrigger(rng, eventLoggerAddressA, 3, 10)
	logIndexX := uint(0)
	exec, err := interop.ExecTriggerFromInitTrigger(init, logIndexX, targetNum, targetTime, actors.ChainA.ChainID)
	require.NoError(t, err)

	// tx includes cycle with self
	calls := []txintent.Call{exec, init}

	// Intent to execute message X and initiate message X at chain A
	tx := txintent.NewIntent[*txintent.MultiTrigger, *txintent.InteropOutput](optsA)
	tx.Content.Set(&txintent.MultiTrigger{Emitter: constants.MultiCall3, Calls: calls})

	included, err := tx.PlannedTx.IncludedBlock.Eval(t.Ctx())
	require.NoError(t, err)

	// Make sure tx in block sealed at expected time
	require.Equal(t, included.Time, targetTime)
	require.Equal(t, included.Number, targetNum)

	// Make batcher happy by advancing at least a single block
	actors.ChainB.Sequencer.ActL2EmptyBlock(t)

	unsafeHeadNumAfterReorg := targetNum - 1
	reorgOutUnsafeAndConsolidateToSafe(t, actors, actors.ChainB, actors.ChainA, 0, 0, 1, targetNum, unsafeHeadNumAfterReorg)
}
