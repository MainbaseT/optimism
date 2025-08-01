package interop

import (
	"context"
	"math/big"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts/metrics"
	"github.com/ethereum-optimism/optimism/op-service/dial"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-chain-ops/interopgen"
	"github.com/ethereum-optimism/optimism/op-e2e/system/helpers"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	gethCore "github.com/ethereum/go-ethereum/core"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
)

// setupAndRun is a helper function that sets up a SuperSystem
// which contains two L2 Chains, and two users on each chain.
func setupAndRun(t *testing.T, config SuperSystemConfig, fn func(*testing.T, SuperSystem)) {
	recipe := interopgen.InteropDevRecipe{
		L1ChainID:        900100,
		L2s:              []interopgen.InteropDevL2Recipe{{ChainID: 900200}, {ChainID: 900201}},
		GenesisTimestamp: uint64(time.Now().Unix() + 3), // start chain 3 seconds from now
	}
	worldResources := WorldResourcePaths{
		FoundryArtifacts: "../../packages/contracts-bedrock/forge-artifacts",
		SourceMap:        "../../packages/contracts-bedrock",
	}

	// create a super system from the recipe
	// and get the L2 IDs for use in the test
	s2 := NewSuperSystem(t, &recipe, worldResources, config)

	// create two users on all L2 chains
	s2.AddUser("Alice")
	s2.AddUser("Bob")

	// run the test
	fn(t, s2)
}

// TestInterop_IsolatedChains tests a simple interop scenario
// Chains A and B exist, but no messages are sent between them
// a transaction is sent from Alice to Bob on Chain A,
// and only Chain A is affected.
func TestInterop_IsolatedChains(t *testing.T) {
	t.Parallel()
	test := func(t *testing.T, s2 SuperSystem) {
		ids := s2.L2IDs()
		chainA := ids[0]
		chainB := ids[1]

		// check the balance of Bob
		bobAddr := s2.Address(chainA, "Bob")
		clientA := s2.L2GethClient(chainA, "sequencer")
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		bobBalance, err := clientA.BalanceAt(ctx, bobAddr, nil)
		require.NoError(t, err)
		expectedBalance, _ := big.NewInt(0).SetString("10000000000000000000000000", 10)
		require.Equal(t, expectedBalance, bobBalance)

		// send a tx from Alice to Bob
		s2.SendL2Tx(
			chainA,
			"sequencer",
			"Alice",
			func(l2Opts *helpers.TxOpts) {
				l2Opts.ToAddr = &bobAddr
				l2Opts.Value = big.NewInt(1000000)
				l2Opts.GasFeeCap = big.NewInt(1_000_000_000)
				l2Opts.GasTipCap = big.NewInt(1_000_000_000)
			},
		)

		// check the balance of Bob after the tx
		ctx, cancel = context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		bobBalance, err = clientA.BalanceAt(ctx, bobAddr, nil)
		require.NoError(t, err)
		expectedBalance, _ = big.NewInt(0).SetString("10000000000000000001000000", 10)
		require.Equal(t, expectedBalance, bobBalance)

		// check that the balance of Bob on ChainB hasn't changed
		bobAddrB := s2.Address(chainB, "Bob")
		clientB := s2.L2GethClient(chainB, "sequencer")
		ctx, cancel = context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		bobBalance, err = clientB.BalanceAt(ctx, bobAddrB, nil)
		require.NoError(t, err)
		expectedBalance, _ = big.NewInt(0).SetString("10000000000000000000000000", 10)
		require.Equal(t, expectedBalance, bobBalance)
	}
	config := SuperSystemConfig{
		mempoolFiltering: false,
	}
	setupAndRun(t, config, test)
}

// TestInterop_SupervisorFinality tests that the supervisor updates its finality
// It waits for the finalized block to advance past the genesis block.
func TestInterop_SupervisorFinality(t *testing.T) {
	t.Parallel()
	test := func(t *testing.T, s2 SuperSystem) {
		supervisor := s2.SupervisorClient()
		require.Eventually(t, func() bool {
			final, err := supervisor.FinalizedL1(context.Background())
			if err != nil && strings.Contains(err.Error(), "not initialized") {
				return false
			}
			require.NoError(t, err)
			return final.Number > 0
			// this test takes about 30 seconds, with a longer Eventually timeout for CI
		}, time.Second*60, time.Second, "wait for finalized block to be greater than 0")
	}
	config := SuperSystemConfig{
		mempoolFiltering: false,
	}
	setupAndRun(t, config, test)
}

// TestInterop_EmitLogs tests a simple interop scenario
// Chains A and B exist, but no messages are sent between them.
// A contract is deployed on each chain, and logs are emitted repeatedly.
func TestInterop_EmitLogs(t *testing.T) {
	t.Parallel()
	test := func(t *testing.T, s2 SuperSystem) {
		ids := s2.L2IDs()
		chainA := ids[0]
		chainB := ids[1]

		// Deploy emitter to chain A
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		EmitterA := s2.DeployEmitterContract(ctx, chainA, "Alice")

		// Deploy emitter to chain B
		ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		EmitterB := s2.DeployEmitterContract(ctx, chainB, "Alice")

		payload1 := "SUPER JACKPOT!"
		numEmits := 10
		// emit logs on both chains in parallel
		var emitParallel sync.WaitGroup
		emitOn := func(chainID string) {
			for i := 0; i < numEmits; i++ {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				s2.EmitData(ctx, chainID, "sequencer", "Alice", payload1)
				cancel()
			}
			emitParallel.Done()
		}
		emitParallel.Add(2)
		go emitOn(chainA)
		go emitOn(chainB)
		emitParallel.Wait()

		clientA := s2.L2GethClient(chainA, "sequencer")
		clientB := s2.L2GethClient(chainB, "sequencer")
		// check that the logs are emitted on chain A
		qA := ethereum.FilterQuery{
			Addresses: []common.Address{EmitterA},
		}
		logsA, err := clientA.FilterLogs(context.Background(), qA)
		require.NoError(t, err)
		require.Len(t, logsA, numEmits)

		// check that the logs are emitted on chain B
		qB := ethereum.FilterQuery{
			Addresses: []common.Address{EmitterB},
		}
		logsB, err := clientB.FilterLogs(context.Background(), qB)
		require.NoError(t, err)
		require.Len(t, logsB, numEmits)

		// wait for cross-safety to settle
		// I've tried 30s but not all logs are cross-safe by then
		time.Sleep(60 * time.Second)

		supervisor := s2.SupervisorClient()

		// helper function to turn a log into an access-list object
		logToAccess := func(chainID string, log gethTypes.Log) types.Access {
			client := s2.L2GethClient(chainID, "sequencer")
			// construct the expected hash of the log's payload
			// (topics concatenated with data)
			msgPayload := make([]byte, 0)
			for _, topic := range log.Topics {
				msgPayload = append(msgPayload, topic.Bytes()...)
			}
			msgPayload = append(msgPayload, log.Data...)
			msgHash := crypto.Keccak256Hash(msgPayload)

			// get block for the log (for timestamp)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			block, err := client.BlockByHash(ctx, log.BlockHash)
			require.NoError(t, err)

			args := types.ChecksumArgs{
				BlockNumber: log.BlockNumber,
				Timestamp:   block.Time(),
				LogIndex:    uint32(log.Index),
				ChainID:     eth.ChainIDFromBig(s2.ChainID(chainID)),
				LogHash:     types.PayloadHashToLogHash(msgHash, log.Address),
			}
			return args.Access()
		}

		var accessEntries []types.Access
		for _, evLog := range logsA {
			accessEntries = append(accessEntries, logToAccess(chainA, evLog))
		}
		for _, evLog := range logsB {
			accessEntries = append(accessEntries, logToAccess(chainB, evLog))
		}
		accessList := types.EncodeAccessList(accessEntries)

		timestamp := uint64(time.Now().Unix())
		ed := types.ExecutingDescriptor{Timestamp: timestamp, ChainID: eth.ChainIDFromBig(s2.ChainID(chainB))}
		ctx = context.Background()
		err = supervisor.CheckAccessList(ctx, accessList, types.CrossSafe, ed)
		require.NoError(t, err, "logsA must all be cross-safe")

		// a log should be invalid if the timestamp is incorrect
		accessEntries[0].Timestamp = 333
		accessList = types.EncodeAccessList(accessEntries)
		err = supervisor.CheckAccessList(ctx, accessList, types.CrossSafe, ed)
		require.ErrorContains(t, err, "conflict")
	}
	config := SuperSystemConfig{
		mempoolFiltering: false,
	}
	setupAndRun(t, config, test)
}

func TestInteropBlockBuilding(t *testing.T) {
	t.Parallel()

	test := func(t *testing.T, s2 SuperSystem) {
		ids := s2.L2IDs()
		chainA := ids[0]
		chainB := ids[1]

		rollupClA := s2.L2RollupClient(chainA, "sequencer")

		// We will initiate on chain A, and execute on chain B
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		s2.DeployEmitterContract(ctx, chainA, "Alice")

		// emit log on chain A
		ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
		emitRec := s2.EmitData(ctx, chainA, "sequencer", "Alice", "hello world")
		cancel()
		t.Logf("Emitted a log event in block %d", emitRec.BlockNumber.Uint64())

		// Wait for initiating side to become cross-unsafe
		require.Eventually(t, func() bool {
			status, err := rollupClA.SyncStatus(context.Background())
			require.NoError(t, err)
			return status.CrossUnsafeL2.Number >= emitRec.BlockNumber.Uint64()
		}, time.Second*60, time.Second, "wait for emitted data to become cross-unsafe")
		t.Logf("Reached cross-unsafe block %d", emitRec.BlockNumber.Uint64())

		// Identify the log
		require.Len(t, emitRec.Logs, 1)
		ev := emitRec.Logs[0]
		ethCl := s2.L2GethClient(chainA, "sequencer")
		header, err := ethCl.HeaderByHash(context.Background(), emitRec.BlockHash)
		require.NoError(t, err)
		identifier := types.Identifier{
			Origin:      ev.Address,
			BlockNumber: ev.BlockNumber,
			LogIndex:    uint32(ev.Index),
			Timestamp:   header.Time,
			ChainID:     eth.ChainIDFromBig(s2.ChainID(chainA)),
		}

		msgPayload := types.LogToMessagePayload(ev)
		payloadHash := crypto.Keccak256Hash(msgPayload)
		logHash := types.PayloadHashToLogHash(payloadHash, identifier.Origin)
		t.Logf("expected payload hash: %s", payloadHash)
		t.Logf("expected log hash: %s", logHash)

		invalidPayload := []byte("test invalid message")
		invalidPayloadHash := crypto.Keccak256Hash(invalidPayload)
		invalidLogHash := types.PayloadHashToLogHash(invalidPayloadHash, identifier.Origin)
		t.Logf("invalid payload hash: %s", invalidPayloadHash)
		t.Logf("invalid log hash: %s", invalidLogHash)

		// hack: geth ingress validates using head timestamp, but should be checking with head+blocktime timestamp,
		// Until we fix that, we need an additional block to be built, otherwise we get hit by the aggressive ingress filter.
		require.Eventually(t, func() bool {
			status, err := rollupClA.SyncStatus(context.Background())
			require.NoError(t, err)
			return status.CrossUnsafeL2.Time > identifier.Timestamp
		}, time.Second*60, time.Second, "wait for emitted data to become cross-unsafe")

		// submit executing txs on B

		t.Log("Testing invalid message")
		{
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
			defer cancel()
			// Emitting an executing message, but with different payload.
			if s2.(*interopE2ESystem).config.mempoolFiltering {
				// We expect the traqnsaction to be filtered out by the mempool if mempool filtering is enabled.
				// ValidateMessage the ErrTxFilteredOut error is checked when sending the tx.
				_, err := s2.ValidateMessage(ctx, chainB, "Alice", identifier, invalidPayloadHash, gethCore.ErrTxFilteredOut)
				require.ErrorContains(t, err, gethCore.ErrTxFilteredOut.Error())
			} else {
				// The miner will include the tx in the block if mempool filtering is disabled, because interop checks don't happen during block building
				// this includes invalid interop messages
				_, err := s2.ValidateMessage(ctx, chainB, "Alice", identifier, invalidPayloadHash, nil)
				require.NoError(t, err)
			}
		}

		t.Log("Testing valid message now")
		{
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
			defer cancel()
			// Emit an executing message with the correct identifier / payload
			rec, err := s2.ValidateMessage(ctx, chainB, "Alice", identifier, payloadHash, nil)
			require.NoError(t, err, "expecting tx to be confirmed")
			t.Logf("confirmed executing msg in block %s", rec.BlockNumber)
		}
		t.Log("Done")
	}

	t.Run("without mempool filtering", func(t *testing.T) {
		t.Parallel()
		config := SuperSystemConfig{
			mempoolFiltering: false,
		}
		setupAndRun(t, config, test)
	})

	t.Run("with mempool filtering", func(t *testing.T) {
		t.Parallel()
		config := SuperSystemConfig{
			mempoolFiltering: true,
		}
		// run again with mempool filtering to observe the behavior of the mempool filter
		setupAndRun(t, config, test)
	})
}

func TestMultiNode(t *testing.T) {
	t.Skip() // TODO(#16174): Decide on future of multi-node support
	t.Parallel()
	test := func(t *testing.T, s2 SuperSystem) {
		supervisor := s2.SupervisorClient()
		require.Eventually(t, func() bool {
			final, err := supervisor.FinalizedL1(context.Background())
			if err != nil && strings.Contains(err.Error(), "not initialized") {
				return false
			}
			require.NoError(t, err)
			return final.Number > 0
			// this test takes about 30 seconds, with a longer Eventually timeout for CI
		}, time.Second*60, time.Second, "wait for finalized block to be greater than 0")

		// now that we have had some action, record the current state of chainA
		chainA := s2.L2IDs()[0]
		seqClient := s2.L2RollupClient(chainA, "sequencer")
		originalStatus, err := seqClient.SyncStatus(context.Background())
		require.NoError(t, err)

		// and then add a new node to the system
		s2.AddNode(chainA, "new-node")
		newNodeClient := s2.L2RollupClient(chainA, "new-node")

		// and check that the supervisor is still working
		// by watching that both nodes advance past the previous status
		require.Eventually(t, func() bool {
			seqStatus, err := seqClient.SyncStatus(context.Background())
			require.NoError(t, err)
			newNodeStatus, err := newNodeClient.SyncStatus(context.Background())
			require.NoError(t, err)
			// check that all heads for both nodes are greater than the original status
			return seqStatus.UnsafeL2.Number > originalStatus.UnsafeL2.Number &&
				seqStatus.CrossUnsafeL2.Number > originalStatus.CrossUnsafeL2.Number &&
				seqStatus.SafeL2.Number > originalStatus.SafeL2.Number &&
				seqStatus.SafeL1.Number > originalStatus.SafeL1.Number &&
				newNodeStatus.UnsafeL2.Number > originalStatus.UnsafeL2.Number &&
				newNodeStatus.CrossUnsafeL2.Number > originalStatus.CrossUnsafeL2.Number &&
				newNodeStatus.SafeL2.Number > originalStatus.SafeL2.Number &&
				newNodeStatus.SafeL1.Number > originalStatus.SafeL1.Number
		}, time.Second*60, time.Second, "wait for all nodes to advance past the original status")
	}
	config := SuperSystemConfig{
		mempoolFiltering: false,
	}
	setupAndRun(t, config, test)
}

func TestProposals(t *testing.T) {
	t.Parallel()
	test := func(t *testing.T, s2 SuperSystem) {
		logger := testlog.Logger(t, log.LvlInfo)
		ids := s2.L2IDs()
		chainA := ids[0]
		proposer := s2.Proposer(chainA)
		// Start the proposer as it isn't started by default.
		err := proposer.Start(context.Background())
		require.NoError(t, err)
		require.NotNil(t, proposer.DisputeGameFactoryAddr)
		gameFactoryAddr := *proposer.DisputeGameFactoryAddr

		rpcClient, err := dial.DialRPCClientWithTimeout(context.Background(), logger, s2.L1().UserRPC().RPC())
		require.NoError(t, err)
		caller := batching.NewMultiCaller(rpcClient, batching.DefaultBatchSize)
		factory := contracts.NewDisputeGameFactoryContract(metrics.NoopContractMetrics, gameFactoryAddr, caller)
		ethClient := ethclient.NewClient(rpcClient)
		require.Eventually(t, func() bool {
			head, err := ethClient.BlockByNumber(context.Background(), nil)
			require.NoError(t, err)
			count, err := factory.GetGameCount(context.Background(), head.Hash())
			require.NoError(t, err)
			t.Logf("Current game count: %v", count)
			return count > 0
		}, 5*time.Minute, time.Second)

		head, err := ethClient.BlockByNumber(context.Background(), nil)
		require.NoError(t, err)
		game, err := factory.GetGame(context.Background(), 0, head.Hash())
		require.NoError(t, err)
		require.Equal(t, uint32(4) /* super permissionless */, game.GameType)
	}
	setupAndRun(t, SuperSystemConfig{}, test)
}
