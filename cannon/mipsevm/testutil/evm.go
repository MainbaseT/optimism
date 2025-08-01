package testutil

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/big"
	"os"
	"testing"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
	"github.com/ethereum-optimism/optimism/op-chain-ops/srcmap"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/eth/tracers/logger"
	"github.com/ethereum/go-ethereum/triedb"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-chain-ops/foundry"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
)

type Artifacts struct {
	MIPS   *foundry.Artifact
	Oracle *foundry.Artifact
}

type Addresses struct {
	MIPS         common.Address
	Oracle       common.Address
	Sender       common.Address
	FeeRecipient common.Address
}

type ContractMetadata struct {
	Artifacts *Artifacts
	Addresses *Addresses
	Version   uint8 // versions.StateVersion can't be used as it causes dependency cycles
}

func TestContractsSetup(t require.TestingT, version MipsVersion, stateVersion uint8) *ContractMetadata {
	artifacts, err := loadArtifacts(version)
	require.NoError(t, err)

	addrs := &Addresses{
		MIPS:         common.Address{0: 0xff, 19: 1},
		Oracle:       common.Address{0: 0xff, 19: 2},
		Sender:       common.Address{0x13, 0x37},
		FeeRecipient: common.Address{0xaa},
	}

	return &ContractMetadata{Artifacts: artifacts, Addresses: addrs, Version: stateVersion}
}

// loadArtifacts loads the Cannon contracts, from the contracts package.
func loadArtifacts(version MipsVersion) (*Artifacts, error) {
	artifactFS := foundry.OpenArtifactsDir("../../../packages/contracts-bedrock/forge-artifacts")
	if arch.IsMips32 || version != MipsMultithreaded {
		return nil, fmt.Errorf("unknown MipsVersion supplied: %v", version)
	}
	mips, err := artifactFS.ReadArtifact("MIPS64.sol", "MIPS64")
	if err != nil {
		return nil, err
	}

	oracle, err := artifactFS.ReadArtifact("PreimageOracle.sol", "PreimageOracle")
	if err != nil {
		return nil, err
	}

	return &Artifacts{
		MIPS:   mips,
		Oracle: oracle,
	}, nil
}

func NewEVMEnv(t testing.TB, contracts *ContractMetadata) (*vm.EVM, *state.StateDB) {
	// Temporary hack until Cancun is activated on mainnet
	cpy := *params.MainnetChainConfig
	chainCfg := &cpy // don't modify the global chain config
	// Activate Cancun for EIP-4844 KZG point evaluation precompile
	cancunActivation := *chainCfg.ShanghaiTime + 10
	chainCfg.CancunTime = &cancunActivation
	offsetBlocks := uint64(1000) // blocks after cancun fork
	bc := &testChain{
		config:    chainCfg,
		startTime: *chainCfg.CancunTime + offsetBlocks*12,
	}
	header := bc.GetHeader(common.Hash{}, 17034870+offsetBlocks)
	db := rawdb.NewMemoryDatabase()
	statedb := state.NewDatabase(triedb.NewDatabase(db, nil), nil)
	state, err := state.New(types.EmptyRootHash, statedb)
	if err != nil {
		t.Fatalf("failed to create memory state db: %v", err)
	}
	blockContext := core.NewEVMBlockContext(header, bc, nil, chainCfg, state)
	vmCfg := vm.Config{}

	env := vm.NewEVM(blockContext, state, chainCfg, vmCfg)
	// pre-deploy the contracts
	env.StateDB.SetCode(contracts.Addresses.Oracle, contracts.Artifacts.Oracle.DeployedBytecode.Object)

	var ctorArgs []byte
	if contracts.Version == 0 { // Old MIPS.sol doesn't specify the state version in the constructor
		var mipsCtorArgs [32]byte
		copy(mipsCtorArgs[12:], contracts.Addresses.Oracle[:])
		ctorArgs = mipsCtorArgs[:]
	} else {
		var mipsCtorArgs [64]byte
		copy(mipsCtorArgs[12:], contracts.Addresses.Oracle[:])
		vers := uint256.NewInt(uint64(contracts.Version)).Bytes32()
		copy(mipsCtorArgs[32:], vers[:])
		ctorArgs = mipsCtorArgs[:]
	}
	mipsDeploy := append(bytes.Clone(contracts.Artifacts.MIPS.Bytecode.Object), ctorArgs...)
	startingGas := uint64(30_000_000)
	retVal, deployedMipsAddr, leftOverGas, err := env.Create(contracts.Addresses.Sender, mipsDeploy, startingGas, common.U2560)
	if err != nil {
		t.Fatalf("failed to deploy MIPS contract. error: '%v'. return value: 0x%x. took %d gas.", err, retVal, startingGas-leftOverGas)
	}
	contracts.Addresses.MIPS = deployedMipsAddr

	rules := env.ChainConfig().Rules(header.Number, true, header.Time)
	env.StateDB.Prepare(rules, contracts.Addresses.Sender, contracts.Addresses.FeeRecipient, &contracts.Addresses.MIPS, vm.ActivePrecompiles(rules), nil)
	return env, state
}

type testChain struct {
	config    *params.ChainConfig
	startTime uint64
}

func (d *testChain) Engine() consensus.Engine {
	return ethash.NewFullFaker()
}

func (d *testChain) Config() *params.ChainConfig {
	return d.config
}

func (d *testChain) GetHeader(h common.Hash, n uint64) *types.Header {
	parentHash := common.Hash{0: 0xff}
	binary.BigEndian.PutUint64(parentHash[1:], n-1)
	return &types.Header{
		ParentHash:      parentHash,
		UncleHash:       types.EmptyUncleHash,
		Coinbase:        common.Address{},
		Root:            common.Hash{},
		TxHash:          types.EmptyTxsHash,
		ReceiptHash:     types.EmptyReceiptsHash,
		Bloom:           types.Bloom{},
		Difficulty:      big.NewInt(0),
		Number:          new(big.Int).SetUint64(n),
		GasLimit:        30_000_000,
		GasUsed:         0,
		Time:            d.startTime + n*12,
		Extra:           nil,
		MixDigest:       common.Hash{},
		Nonce:           types.BlockNonce{},
		BaseFee:         big.NewInt(7),
		WithdrawalsHash: &types.EmptyWithdrawalsHash,
	}
}

func MarkdownTracer() *tracing.Hooks {
	return logger.NewMarkdownLogger(&logger.Config{}, os.Stdout).Hooks()
}

func SourceMapTracer(t require.TestingT, version MipsVersion, mips *foundry.Artifact, oracle *foundry.Artifact, addrs *Addresses) *tracing.Hooks {
	srcFS := foundry.NewSourceMapFS(os.DirFS("../../../packages/contracts-bedrock"))
	if arch.IsMips32 || version != MipsMultithreaded {
		require.Fail(t, "invalid mips version")
	}
	mipsSrcMap, err := srcFS.SourceMap(mips, "MIPS64")
	require.NoError(t, err)
	oracleSrcMap, err := srcFS.SourceMap(oracle, "PreimageOracle")
	require.NoError(t, err)

	return srcmap.NewSourceMapTracer(map[common.Address]*srcmap.SourceMap{
		addrs.MIPS:   mipsSrcMap,
		addrs.Oracle: oracleSrcMap,
	}, os.Stdout).Hooks()
}
