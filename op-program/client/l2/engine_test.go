package l2

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-program/client/l2/engineapi"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/stateless"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/trie"

	"github.com/ethereum-optimism/optimism/op-node/chaincfg"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-node/rollup/engine"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

// Should implement derive.Engine
var _ engine.Engine = (*OracleEngine)(nil)

func TestPayloadByHash(t *testing.T) {
	ctx := context.Background()

	t.Run("KnownBlock", func(t *testing.T) {
		engine, stub := createOracleEngine(t, false)
		block := stub.head
		payload, err := engine.PayloadByHash(ctx, block.Hash())
		require.NoError(t, err)
		expected, err := eth.BlockAsPayload(block, engine.backend.Config())
		require.NoError(t, err)
		require.Equal(t, &eth.ExecutionPayloadEnvelope{ExecutionPayload: expected}, payload)
	})

	t.Run("UnknownBlock", func(t *testing.T) {
		engine, _ := createOracleEngine(t, false)
		hash := common.HexToHash("0x878899")
		payload, err := engine.PayloadByHash(ctx, hash)
		require.ErrorIs(t, err, ethereum.NotFound)
		require.Nil(t, payload)
	})
}

func TestPayloadByNumber(t *testing.T) {
	ctx := context.Background()

	t.Run("KnownBlock", func(t *testing.T) {
		engine, stub := createOracleEngine(t, false)
		block := stub.head
		payload, err := engine.PayloadByNumber(ctx, block.NumberU64())
		require.NoError(t, err)
		expected, err := eth.BlockAsPayload(block, engine.backend.Config())
		require.NoError(t, err)
		require.Equal(t, &eth.ExecutionPayloadEnvelope{ExecutionPayload: expected}, payload)
	})

	t.Run("NoCanonicalHash", func(t *testing.T) {
		engine, _ := createOracleEngine(t, false)
		payload, err := engine.PayloadByNumber(ctx, uint64(700))
		require.ErrorIs(t, err, ethereum.NotFound)
		require.Nil(t, payload)
	})

	t.Run("UnknownBlock", func(t *testing.T) {
		engine, stub := createOracleEngine(t, false)
		hash := common.HexToHash("0x878899")
		number := uint64(700)
		stub.canonical[number] = hash
		payload, err := engine.PayloadByNumber(ctx, number)
		require.ErrorIs(t, err, ethereum.NotFound)
		require.Nil(t, payload)
	})
}

func TestL2BlockRefByLabel(t *testing.T) {
	ctx := context.Background()
	engine, stub := createOracleEngine(t, false)
	tests := []struct {
		name  eth.BlockLabel
		block *types.Block
	}{
		{eth.Unsafe, stub.head},
		{eth.Safe, stub.safe},
		{eth.Finalized, stub.finalized},
	}
	for _, test := range tests {
		t.Run(string(test.name), func(t *testing.T) {
			expected, err := derive.L2BlockToBlockRef(engine.rollupCfg, test.block)
			require.NoError(t, err)
			blockRef, err := engine.L2BlockRefByLabel(ctx, test.name)
			require.NoError(t, err)
			require.Equal(t, expected, blockRef)
		})
	}
	t.Run("UnknownLabel", func(t *testing.T) {
		_, err := engine.L2BlockRefByLabel(ctx, "nope")
		require.ErrorIs(t, err, ErrUnknownLabel)
	})
}

func TestL2BlockRefByHash(t *testing.T) {
	ctx := context.Background()
	engine, stub := createOracleEngine(t, false)

	t.Run("KnownBlock", func(t *testing.T) {
		expected, err := derive.L2BlockToBlockRef(engine.rollupCfg, stub.safe)
		require.NoError(t, err)
		ref, err := engine.L2BlockRefByHash(ctx, stub.safe.Hash())
		require.NoError(t, err)
		require.Equal(t, expected, ref)
	})

	t.Run("UnknownBlock", func(t *testing.T) {
		ref, err := engine.L2BlockRefByHash(ctx, common.HexToHash("0x878899"))
		require.ErrorIs(t, err, ethereum.NotFound)
		require.Equal(t, eth.L2BlockRef{}, ref)
	})
}

func TestSystemConfigByL2Hash(t *testing.T) {
	ctx := context.Background()
	engine, stub := createOracleEngine(t, false)

	t.Run("KnownBlock", func(t *testing.T) {
		payload, err := eth.BlockAsPayload(stub.safe, engine.backend.Config())
		require.NoError(t, err)
		expected, err := derive.PayloadToSystemConfig(engine.rollupCfg, payload)
		require.NoError(t, err)
		cfg, err := engine.SystemConfigByL2Hash(ctx, stub.safe.Hash())
		require.NoError(t, err)
		require.Equal(t, expected, cfg)
	})

	t.Run("UnknownBlock", func(t *testing.T) {
		ref, err := engine.SystemConfigByL2Hash(ctx, common.HexToHash("0x878899"))
		require.ErrorIs(t, err, ethereum.NotFound)
		require.Equal(t, eth.SystemConfig{}, ref)
	})
}

func TestL2OutputRootIsthmus(t *testing.T) {
	engine, _ := createOracleEngine(t, true)

	t.Run("Header withdrawalsRoot without fetching state", func(t *testing.T) {
		// should return without a panic since there's no need to fetch state when Isthmus is activate,
		// StateAt() is not implemented in the stub
		_, _, err := engine.L2OutputRoot(4)
		require.NoError(t, err)
	})
}

func createOracleEngine(t *testing.T, headBlockOnIsthmus bool) (*OracleEngine, *stubEngineBackend) {
	head := createL2Block(t, 4, headBlockOnIsthmus)
	safe := createL2Block(t, 3, false)
	finalized := createL2Block(t, 2, false)
	rollupCfg := chaincfg.OPSepolia()
	if headBlockOnIsthmus {
		rollupCfg.IsthmusTime = &head.Header().Time
	}
	backend := &stubEngineBackend{
		head:      head,
		safe:      safe,
		finalized: finalized,
		blocks: map[common.Hash]*types.Block{
			head.Hash():      head,
			safe.Hash():      safe,
			finalized.Hash(): finalized,
		},
		canonical: map[uint64]common.Hash{
			head.NumberU64():      head.Hash(),
			safe.NumberU64():      safe.Hash(),
			finalized.NumberU64(): finalized.Hash(),
		},
		rollupCfg: rollupCfg,
	}
	engine := OracleEngine{
		backend:   backend,
		rollupCfg: rollupCfg,
	}
	return &engine, backend
}

func createL2Block(t *testing.T, number int, setWithdrawalsRoot bool) *types.Block {
	tx, err := derive.L1InfoDeposit(chaincfg.OPSepolia(), eth.SystemConfig{}, uint64(1), eth.HeaderBlockInfo(&types.Header{
		Number:  big.NewInt(32),
		BaseFee: big.NewInt(7),
	}), 0)
	require.NoError(t, err)
	header := &types.Header{
		Number:  big.NewInt(int64(number)),
		BaseFee: big.NewInt(7),
	}
	body := &types.Body{
		Transactions: []*types.Transaction{types.NewTx(tx)},
	}
	blockConfig := types.DefaultBlockConfig
	var withdrawals []*types.Withdrawal
	if setWithdrawalsRoot {
		withdrawals = make([]*types.Withdrawal, 0)
		body.Withdrawals = withdrawals
		header.WithdrawalsHash = &types.EmptyWithdrawalsHash
		blockConfig = types.IsthmusBlockConfig
	}
	return types.NewBlock(header, body, nil, trie.NewStackTrie(nil), blockConfig)
}

type stubEngineBackend struct {
	head      *types.Block
	safe      *types.Block
	finalized *types.Block
	blocks    map[common.Hash]*types.Block
	canonical map[uint64]common.Hash
	rollupCfg *rollup.Config
}

func (s *stubEngineBackend) CurrentHeader() *types.Header {
	return s.head.Header()
}

func (s *stubEngineBackend) CurrentSafeBlock() *types.Header {
	return s.safe.Header()
}

func (s *stubEngineBackend) CurrentFinalBlock() *types.Header {
	return s.finalized.Header()
}

func (s *stubEngineBackend) GetBlockByHash(hash common.Hash) *types.Block {
	return s.blocks[hash]
}

func (s *stubEngineBackend) GetCanonicalHash(n uint64) common.Hash {
	return s.canonical[n]
}

func (s *stubEngineBackend) GetReceiptsByBlockHash(hash common.Hash) types.Receipts {
	panic("unsupported")
}

func (s *stubEngineBackend) GetBlock(hash common.Hash, number uint64) *types.Block {
	panic("unsupported")
}

func (s *stubEngineBackend) HasBlockAndState(hash common.Hash, number uint64) bool {
	panic("unsupported")
}

func (s *stubEngineBackend) GetVMConfig() *vm.Config {
	panic("unsupported")
}

func (s *stubEngineBackend) Config() *params.ChainConfig {
	return &params.ChainConfig{
		ShanghaiTime: s.rollupCfg.CanyonTime,
	}
}

func (s *stubEngineBackend) Engine() consensus.Engine {
	panic("unsupported")
}

func (s *stubEngineBackend) StateAt(root common.Hash) (*state.StateDB, error) {
	panic("unsupported")
}

func (s *stubEngineBackend) InsertBlockWithoutSetHead(block *types.Block, makeWitness bool) (*stateless.Witness, error) {
	panic("unsupported")
}

func (s stubEngineBackend) AssembleAndInsertBlockWithoutSetHead(_ *engineapi.BlockProcessor) (*types.Block, error) {
	panic("unsupported")
}

func (s *stubEngineBackend) SetCanonical(head *types.Block) (common.Hash, error) {
	panic("unsupported")
}

func (s *stubEngineBackend) SetFinalized(header *types.Header) {
	panic("unsupported")
}

func (s *stubEngineBackend) SetSafe(header *types.Header) {
	panic("unsupported")
}

func (s *stubEngineBackend) GetHeader(hash common.Hash, number uint64) *types.Header {
	panic("unsupported")
}

// currently returns the head block's header (as required by a test)
func (s *stubEngineBackend) GetHeaderByNumber(number uint64) *types.Header {
	return s.head.Header()
}

func (s *stubEngineBackend) GetHeaderByHash(hash common.Hash) *types.Header {
	panic("unsupported")
}

func (s *stubEngineBackend) GetTd(hash common.Hash, number uint64) *big.Int {
	panic("unsupported")
}
