package proofs

import (
	"context"
	"encoding/binary"
	"strings"
	"sync"
	"testing"
	"time"

	actionsHelpers "github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/actions/proofs/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/contracts/bindings/invoker"
	preimage "github.com/ethereum-optimism/optimism/op-preimage"
	"github.com/ethereum-optimism/optimism/op-program/client/l1"
	hostcommon "github.com/ethereum-optimism/optimism/op-program/host/common"
	hostconfig "github.com/ethereum-optimism/optimism/op-program/host/config"
	"github.com/ethereum-optimism/optimism/op-program/host/kvstore"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

func Test_OPProgramAction_PrecompileHint(gt *testing.T) {
	matrix := helpers.NewMatrix[any]()
	defer matrix.Run(gt)

	for _, test := range PrecompileTestFixtures {
		testCase := test
		matrix.AddTestCase(
			test.Name,
			nil,
			helpers.FaultProofForks(),
			func(t *testing.T, testCfg *helpers.TestCfg[any]) {
				runPrecompileHintTest(t, testCase, testCfg)
			},
			helpers.ExpectNoError(),
		)
	}
}

func runPrecompileHintTest(gt *testing.T, testCase PrecompileTestFixture, testCfg *helpers.TestCfg[any]) {
	t := actionsHelpers.NewDefaultTesting(gt)
	env := helpers.NewL2FaultProofEnv(t, testCfg, helpers.NewTestParams(), helpers.NewBatcherCfg())

	// Build a block on L2 with 1 tx.
	env.Alice.L2.ActResetTxOpts(t)
	env.Alice.L2.ActSetTxToAddr(&testCase.Address)(t)
	env.Alice.L2.ActSetTxCalldata(testCase.Input)(t)
	env.Alice.L2.ActMakeTx(t)

	env.Sequencer.ActL2StartBlock(t)
	env.Engine.ActL2IncludeTx(env.Alice.Address())(t)
	env.Sequencer.ActL2EndBlock(t)
	env.Alice.L2.ActCheckReceiptStatusOfLastTx(true)(t)

	// Instruct the batcher to submit the block to L1, and include the transaction.
	env.Batcher.ActSubmitAll(t)
	env.Miner.ActL1StartBlock(12)(t)
	env.Miner.ActL1IncludeTxByHash(env.Batcher.LastSubmitted.Hash())(t)
	env.Miner.ActL1EndBlock(t)

	// Finalize the block with the batch on L1.
	env.Miner.ActL1SafeNext(t)
	env.Miner.ActL1FinalizeNext(t)

	// Instruct the sequencer to derive the L2 chain from the data on L1 that the batcher just posted.
	env.Sequencer.ActL1HeadSignal(t)
	env.Sequencer.ActL2PipelineFull(t)

	l1Head := env.Miner.L1Chain().CurrentBlock()
	l2SafeHead := env.Engine.L2Chain().CurrentSafeBlock()

	// Ensure there is only 1 block on L1.
	require.Equal(t, uint64(1), l1Head.Number.Uint64())
	// Ensure the block is marked as safe before we attempt to fault prove it.
	require.Equal(t, uint64(1), l2SafeHead.Number.Uint64())

	defaultParam := helpers.WithPreInteropDefaults(t, l2SafeHead.Number.Uint64(), env.Sequencer.L2Verifier, env.Engine)
	fixtureInputParams := []helpers.FixtureInputParam{defaultParam, helpers.WithL1Head(l1Head.Hash())}
	var fixtureInputs helpers.FixtureInputs
	for _, apply := range fixtureInputParams {
		apply(&fixtureInputs)
	}
	programCfg := helpers.NewOpProgramCfg(&fixtureInputs)
	// Create an external in-memory kv store so we can inspect the precompile results.
	kv := kvstore.NewMemKV()
	var precompileHints *precompileHintCounter
	withInProcessPrefetcher := hostcommon.WithPrefetcher(func(ctx context.Context, logger log.Logger, _kv kvstore.KV, cfg *hostconfig.Config) (hostcommon.Prefetcher, error) {
		prefetcher, err := helpers.CreateInprocessPrefetcher(t, ctx, logger, env.Miner, kv, cfg, &fixtureInputs)
		precompileHints = &precompileHintCounter{Prefetcher: prefetcher}
		return precompileHints, err
	})
	ctx, cancel := context.WithTimeout(t.Ctx(), 2*time.Minute)
	defer cancel()
	require.NoError(t, hostcommon.FaultProofProgram(ctx, testlog.Logger(t, log.LevelDebug).New("role", "program"), programCfg, withInProcessPrefetcher))
	require.NotNil(t, precompileHints)

	rules := env.Engine.L2Chain().Config().Rules(l2SafeHead.Number, true, l2SafeHead.Time)
	precompile, ok := vm.ActivePrecompiledContracts(rules)[testCase.Address]
	if !ok {
		require.Zero(t, precompileHints.GetCount(), "received precompile hints for inactive precompile")
		return
	}

	gas := precompile.RequiredGas(testCase.Input)
	precompileKey := createPrecompileKey(testCase.Address, testCase.Input, gas)
	// If accelerated, make sure that the precompile was fetched from the host.
	if testCase.Accelerated {
		require.Equal(t, 1, precompileHints.GetCount(), "unexpected number of precompile hints for accelerated precompile")
		programResult, err := kv.Get(precompileKey)
		require.NoError(t, err)

		precompileSuccess := [1]byte{1}
		expected, err := precompile.Run(testCase.Input)
		expected = append(precompileSuccess[:], expected...)
		require.NoError(t, err)
		require.EqualValues(t, expected, programResult)
	} else {
		require.Zero(t, precompileHints.GetCount(), "received precompile hints for non-accelerated precompile")
		_, err := kv.Get(precompileKey)
		require.ErrorIs(t, kvstore.ErrNotFound, err)
	}
}

func createPrecompileKey(precompileAddress common.Address, input []byte, gas uint64) common.Hash {
	hintBytes := append(precompileAddress.Bytes(), binary.BigEndian.AppendUint64(nil, gas)...)
	return preimage.PrecompileKey(crypto.Keccak256Hash(append(hintBytes, input...))).PreimageKey()
}

type precompileHintCounter struct {
	sync.RWMutex
	count int
	hostcommon.Prefetcher
}

func (p *precompileHintCounter) Hint(hint string) error {
	p.RWMutex.Lock()
	defer p.RWMutex.Unlock()
	if strings.HasPrefix(hint, l1.HintL1Precompile) || strings.HasPrefix(hint, l1.HintL1PrecompileV2) {
		p.count++
	}
	return p.Prefetcher.Hint(hint)
}

func (p *precompileHintCounter) GetCount() int {
	p.RWMutex.RLock()
	defer p.RWMutex.RUnlock()
	return p.count
}

func Test_ProgramAction_Precompiles(gt *testing.T) {
	matrix := helpers.NewMatrix[PrecompileTestFixture]()
	defer matrix.Run(gt)
	for _, test := range PrecompileTestFixtures {
		testCase := test
		matrix.AddTestCase(
			testCase.Name,
			testCase,
			helpers.FaultProofForks(),
			runPrecompileTest,
			helpers.ExpectNoError(),
		)
	}
}

func runPrecompileTest(gt *testing.T, testCfg *helpers.TestCfg[PrecompileTestFixture]) {
	t := actionsHelpers.NewDefaultTesting(gt)
	env := helpers.NewL2FaultProofEnv(t, testCfg, helpers.NewTestParams(), helpers.NewBatcherCfg())
	testCase := testCfg.Custom

	// deploy invoker contract
	env.Alice.L2.ActResetTxOpts(t)
	env.Alice.L2.ActSetTxCalldata(common.FromHex(invoker.InvokerMetaData.Bin))(t)
	env.Alice.L2.ActMakeTx(t)
	env.Sequencer.ActL2StartBlock(t)
	env.Engine.ActL2IncludeTx(env.Alice.Address())(t)
	env.Sequencer.ActL2EndBlock(t)
	env.Alice.L2.ActCheckReceiptStatusOfLastTx(true)(t)

	invokerContract := env.Alice.L2.LastTxReceipt(t).ContractAddress
	require.NotZero(t, invokerContract, "invoker contract address is zero")
	abi, err := invoker.InvokerMetaData.GetAbi()
	require.NoError(t, err)
	invokeCalldata, err := abi.Pack("invokePrecompile", testCase.Address, testCase.Input)
	require.NoError(t, err)

	// call precompile via invoker
	env.Alice.L2.ActResetTxOpts(t)
	env.Alice.L2.ActSetTxToAddr(&invokerContract)(t)
	env.Alice.L2.ActSetTxCalldata(invokeCalldata)(t)
	env.Alice.L2.ActMakeTx(t)
	env.Sequencer.ActL2StartBlock(t)
	env.Engine.ActL2IncludeTx(env.Alice.Address())(t)
	env.Sequencer.ActL2EndBlock(t)
	env.Alice.L2.ActCheckReceiptStatusOfLastTx(true)(t)

	receipt := env.Alice.L2.LastTxReceipt(t)
	receiptBlockTime := env.Engine.L2Chain().GetBlockByHash(receipt.BlockHash).Time()
	rules := env.Engine.L2Chain().Config().Rules(receipt.BlockNumber, true, receiptBlockTime)
	expectedResult := make([]byte, 0)
	precompile, ok := vm.ActivePrecompiledContracts(rules)[testCase.Address]
	if ok {
		expectedResult, err = precompile.Run(testCase.Input)
		require.NoError(t, err)
	}

	// sanity check Invoker precompile call
	require.Equal(t, receipt.Status, uint64(1), "transaction should succeed")
	require.Len(t, receipt.Logs, 1)
	require.Equal(t, receipt.Logs[0].Address, invokerContract)
	require.Len(t, receipt.Logs[0].Topics, 2)
	precompileAddress := receipt.Logs[0].Topics[1]
	var out struct{ Result []byte }
	err = abi.UnpackIntoInterface(&out, "PrecompileInvoked", receipt.Logs[0].Data)
	precompileResult := out.Result
	require.NoError(t, err)
	require.Equal(t, common.HexToAddress(precompileAddress.Hex()), testCase.Address)
	require.Equal(t, expectedResult, precompileResult)

	// instruct the batcher to submit the Invoker precompile tx to l1, and include the transaction.
	env.Batcher.ActSubmitAll(t)
	env.Miner.ActL1StartBlock(12)(t)
	env.Miner.ActL1IncludeTxByHash(env.Batcher.LastSubmitted.Hash())(t)
	env.Miner.ActL1EndBlock(t)

	// Finalize the block with the batch on L1.
	env.Miner.ActL1SafeNext(t)
	env.Miner.ActL1FinalizeNext(t)

	// Instruct the sequencer to derive the L2 chain from the data on L1 that the batcher just posted.
	env.Sequencer.ActL1HeadSignal(t)
	env.Sequencer.ActL2PipelineFull(t)

	l1Head := env.Miner.L1Chain().CurrentBlock()
	l2SafeHead := env.Engine.L2Chain().CurrentSafeBlock()

	require.Equal(t, uint64(1), l1Head.Number.Uint64())
	// Ensure the block is marked as safe before we attempt to fault prove it.
	require.Equal(t, uint64(2), l2SafeHead.Number.Uint64())

	defaultParam := helpers.WithPreInteropDefaults(t, l2SafeHead.Number.Uint64(), env.Sequencer.L2Verifier, env.Engine)
	fixtureInputParams := []helpers.FixtureInputParam{defaultParam, helpers.WithL1Head(l1Head.Hash())}
	var fixtureInputs helpers.FixtureInputs
	for _, apply := range fixtureInputParams {
		apply(&fixtureInputs)
	}

	env.RunFaultProofProgram(t, l2SafeHead.Number.Uint64(), helpers.ExpectNoError(), fixtureInputParams...)
}
