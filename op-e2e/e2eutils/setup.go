package e2eutils

import (
	"math/big"
	"os"
	"path"
	"time"

	"github.com/ethereum-optimism/optimism/op-e2e/config/secrets"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"

	"github.com/stretchr/testify/require"

	altda "github.com/ethereum-optimism/optimism/op-alt-da"
	"github.com/ethereum-optimism/optimism/op-chain-ops/genesis"
	"github.com/ethereum-optimism/optimism/op-e2e/config"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

var testingJWTSecret = [32]byte{123}

// WriteDefaultJWT writes a testing JWT to the temporary directory of the test and returns the path to the JWT file.
func WriteDefaultJWT(t TestingBase) string {
	// Sadly the geth node config cannot load JWT secret from memory, it has to be a file
	jwtPath := path.Join(t.TempDir(), "jwt_secret")
	if err := os.WriteFile(jwtPath, []byte(hexutil.Encode(testingJWTSecret[:])), 0o600); err != nil {
		t.Fatalf("failed to prepare jwt file for geth: %v", err)
	}
	return jwtPath
}

// DeployParams bundles the deployment parameters to generate further testing inputs with.
type DeployParams struct {
	DeployConfig   *genesis.DeployConfig
	MnemonicConfig *secrets.MnemonicConfig
	Secrets        *secrets.Secrets
	Addresses      *secrets.Addresses
	AllocType      config.AllocType
}

// TestParams parametrizes the most essential rollup configuration parameters
type TestParams struct {
	MaxSequencerDrift   uint64
	SequencerWindowSize uint64
	ChannelTimeout      uint64
	L1BlockTime         uint64
	UseAltDA            bool
	AllocType           config.AllocType
}

func MakeDeployParams(t require.TestingT, tp *TestParams) *DeployParams {
	mnemonicCfg := secrets.DefaultMnemonicConfig
	secrets := secrets.DefaultSecrets
	addresses := secrets.Addresses()

	deployConfig := config.DeployConfig(tp.AllocType)
	deployConfig.MaxSequencerDrift = tp.MaxSequencerDrift
	deployConfig.SequencerWindowSize = tp.SequencerWindowSize
	deployConfig.ChannelTimeoutBedrock = tp.ChannelTimeout
	deployConfig.L1BlockTime = tp.L1BlockTime
	deployConfig.UseAltDA = tp.UseAltDA
	ApplyDeployConfigForks(deployConfig)

	logger := log.NewLogger(log.DiscardHandler())
	require.NoError(t, deployConfig.Check(logger))
	require.Equal(t, addresses.Batcher, deployConfig.BatchSenderAddress)
	require.Equal(t, addresses.Proposer, deployConfig.L2OutputOracleProposer)
	require.Equal(t, addresses.SequencerP2P, deployConfig.P2PSequencerAddress)

	return &DeployParams{
		DeployConfig:   deployConfig,
		MnemonicConfig: mnemonicCfg,
		Secrets:        secrets,
		Addresses:      addresses,
		AllocType:      tp.AllocType,
	}
}

// SetupData bundles the L1, L2, rollup and deployment configuration data: everything for a full test setup.
type SetupData struct {
	L1Cfg         *core.Genesis
	L2Cfg         *core.Genesis
	RollupCfg     *rollup.Config
	DependencySet depset.DependencySet
	ChainSpec     *rollup.ChainSpec
	DeploymentsL1 *genesis.L1Deployments
}

// AllocParams defines genesis allocations to apply on top of the genesis generated by deploy parameters.
// These allocations override existing allocations per account,
// i.e. the allocations are merged with AllocParams having priority.
type AllocParams struct {
	L1Alloc          types.GenesisAlloc
	L2Alloc          types.GenesisAlloc
	PrefundTestUsers bool
}

var etherScalar = new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)

// Ether converts a uint64 Ether amount into a *big.Int amount in wei units, for allocating test balances.
func Ether(v uint64) *big.Int {
	return new(big.Int).Mul(new(big.Int).SetUint64(v), etherScalar)
}

func GetL2AllocsMode(dc *genesis.DeployConfig, t uint64) genesis.L2AllocsMode {
	if fork := dc.JovianTime(t); fork != nil && *fork <= 0 {
		return genesis.L2AllocsJovian
	}
	if fork := dc.InteropTime(t); fork != nil && *fork <= 0 {
		return genesis.L2AllocsInterop
	}
	if fork := dc.IsthmusTime(t); fork != nil && *fork <= 0 {
		return genesis.L2AllocsIsthmus
	}
	if fork := dc.HoloceneTime(t); fork != nil && *fork <= 0 {
		return genesis.L2AllocsHolocene
	}
	if fork := dc.GraniteTime(t); fork != nil && *fork <= 0 {
		return genesis.L2AllocsGranite
	}
	if fork := dc.FjordTime(t); fork != nil && *fork <= 0 {
		return genesis.L2AllocsFjord
	}
	if fork := dc.EcotoneTime(t); fork != nil && *fork <= 0 {
		return genesis.L2AllocsEcotone
	}
	return genesis.L2AllocsDelta
}

// Setup computes the testing setup configurations from deployment configuration and optional allocation parameters.
func Setup(t require.TestingT, deployParams *DeployParams, alloc *AllocParams) *SetupData {
	deployConf := deployParams.DeployConfig.Copy()
	deployConf.L1GenesisBlockTimestamp = hexutil.Uint64(time.Now().Unix())
	logger := log.NewLogger(log.DiscardHandler())
	require.NoError(t, deployConf.Check(logger))

	l1Deployments := config.L1Deployments(deployParams.AllocType)
	require.NoError(t, l1Deployments.Check(deployConf))

	l1Genesis, err := genesis.BuildL1DeveloperGenesis(
		deployConf,
		config.L1Allocs(deployParams.AllocType),
		l1Deployments,
	)
	require.NoError(t, err, "failed to create l1 genesis")
	if alloc.PrefundTestUsers {
		for _, addr := range deployParams.Addresses.All() {
			l1Genesis.Alloc[addr] = types.Account{
				Balance: Ether(1e12),
			}
		}
	}
	for addr, val := range alloc.L1Alloc {
		l1Genesis.Alloc[addr] = val
	}

	l1Block := l1Genesis.ToBlock()

	allocsMode := GetL2AllocsMode(deployConf, l1Block.Time())
	l2Allocs := config.L2Allocs(deployParams.AllocType, allocsMode)
	l2Genesis, err := genesis.BuildL2Genesis(deployConf, l2Allocs, eth.BlockRefFromHeader(l1Block.Header()))
	require.NoError(t, err, "failed to create l2 genesis")
	if alloc.PrefundTestUsers {
		for _, addr := range deployParams.Addresses.All() {
			l2Genesis.Alloc[addr] = types.Account{
				Balance: Ether(1e12),
			}
		}
	}
	for addr, val := range alloc.L2Alloc {
		l2Genesis.Alloc[addr] = val
	}

	var pcfg *rollup.AltDAConfig
	if deployConf.UseAltDA {
		pcfg = &rollup.AltDAConfig{
			DAChallengeAddress: l1Deployments.DataAvailabilityChallengeProxy,
			DAChallengeWindow:  deployConf.DAChallengeWindow,
			DAResolveWindow:    deployConf.DAResolveWindow,
			CommitmentType:     altda.KeccakCommitmentString,
		}
	}

	rollupCfg := &rollup.Config{
		Genesis: rollup.Genesis{
			L1: eth.BlockID{
				Hash:   l1Block.Hash(),
				Number: 0,
			},
			L2: eth.BlockID{
				Hash:   l2Genesis.ToBlock().Hash(),
				Number: 0,
			},
			L2Time:       uint64(deployConf.L1GenesisBlockTimestamp),
			SystemConfig: SystemConfigFromDeployConfig(deployConf),
		},
		BlockTime:              deployConf.L2BlockTime,
		MaxSequencerDrift:      deployConf.MaxSequencerDrift,
		SeqWindowSize:          deployConf.SequencerWindowSize,
		ChannelTimeoutBedrock:  deployConf.ChannelTimeoutBedrock,
		L1ChainID:              new(big.Int).SetUint64(deployConf.L1ChainID),
		L2ChainID:              new(big.Int).SetUint64(deployConf.L2ChainID),
		BatchInboxAddress:      deployConf.BatchInboxAddress,
		DepositContractAddress: deployConf.OptimismPortalProxy,
		L1SystemConfigAddress:  deployConf.SystemConfigProxy,
		RegolithTime:           deployConf.RegolithTime(uint64(deployConf.L1GenesisBlockTimestamp)),
		CanyonTime:             deployConf.CanyonTime(uint64(deployConf.L1GenesisBlockTimestamp)),
		DeltaTime:              deployConf.DeltaTime(uint64(deployConf.L1GenesisBlockTimestamp)),
		EcotoneTime:            deployConf.EcotoneTime(uint64(deployConf.L1GenesisBlockTimestamp)),
		FjordTime:              deployConf.FjordTime(uint64(deployConf.L1GenesisBlockTimestamp)),
		GraniteTime:            deployConf.GraniteTime(uint64(deployConf.L1GenesisBlockTimestamp)),
		HoloceneTime:           deployConf.HoloceneTime(uint64(deployConf.L1GenesisBlockTimestamp)),
		PectraBlobScheduleTime: deployConf.PectraBlobScheduleTime(uint64(deployConf.L1GenesisBlockTimestamp)),
		IsthmusTime:            deployConf.IsthmusTime(uint64(deployConf.L1GenesisBlockTimestamp)),
		JovianTime:             deployConf.JovianTime(uint64(deployConf.L1GenesisBlockTimestamp)),
		InteropTime:            deployConf.InteropTime(uint64(deployConf.L1GenesisBlockTimestamp)),
		AltDAConfig:            pcfg,
		ChainOpConfig: &params.OptimismConfig{
			EIP1559Elasticity:        deployConf.EIP1559Elasticity,
			EIP1559Denominator:       deployConf.EIP1559Denominator,
			EIP1559DenominatorCanyon: &deployConf.EIP1559DenominatorCanyon,
		},
	}

	require.NoError(t, rollupCfg.Check())

	// Sanity check that the config is correct
	require.Equal(t, deployParams.Secrets.Addresses().Batcher, deployParams.DeployConfig.BatchSenderAddress)
	require.Equal(t, deployParams.Secrets.Addresses().SequencerP2P, deployParams.DeployConfig.P2PSequencerAddress)
	require.Equal(t, deployParams.Secrets.Addresses().Proposer, deployParams.DeployConfig.L2OutputOracleProposer)

	return &SetupData{
		L1Cfg:         l1Genesis,
		L2Cfg:         l2Genesis,
		RollupCfg:     rollupCfg,
		ChainSpec:     rollup.NewChainSpec(rollupCfg),
		DeploymentsL1: l1Deployments,
	}
}

func SystemConfigFromDeployConfig(deployConfig *genesis.DeployConfig) eth.SystemConfig {
	return deployConfig.GenesisSystemConfig()
}

func ApplyDeployConfigForks(deployConfig *genesis.DeployConfig) {
	isJovian := os.Getenv("OP_E2E_USE_JOVIAN") == "true"
	isIsthmus := isJovian || os.Getenv("OP_E2E_USE_ISTHMUS") == "true"
	isHolocene := isIsthmus || os.Getenv("OP_E2E_USE_HOLOCENE") == "true"
	isGranite := isHolocene || os.Getenv("OP_E2E_USE_GRANITE") == "true"
	isFjord := isGranite || os.Getenv("OP_E2E_USE_FJORD") == "true"
	isEcotone := isFjord || os.Getenv("OP_E2E_USE_ECOTONE") == "true"
	isDelta := isEcotone || os.Getenv("OP_E2E_USE_DELTA") == "true"
	if isDelta {
		deployConfig.L2GenesisDeltaTimeOffset = new(hexutil.Uint64)
	}
	if isEcotone {
		deployConfig.L2GenesisEcotoneTimeOffset = new(hexutil.Uint64)
	}
	if isFjord {
		deployConfig.L2GenesisFjordTimeOffset = new(hexutil.Uint64)
	}
	if isGranite {
		deployConfig.L2GenesisGraniteTimeOffset = new(hexutil.Uint64)
	}
	if isHolocene {
		deployConfig.L2GenesisHoloceneTimeOffset = new(hexutil.Uint64)
	}
	if isIsthmus {
		deployConfig.L2GenesisIsthmusTimeOffset = new(hexutil.Uint64)
	}
	if isJovian {
		deployConfig.L2GenesisJovianTimeOffset = new(hexutil.Uint64)
	}
	// Canyon and lower is activated by default
	deployConfig.L2GenesisCanyonTimeOffset = new(hexutil.Uint64)
	deployConfig.L2GenesisRegolithTimeOffset = new(hexutil.Uint64)
	// Activated by default, contracts depend on it
	deployConfig.L1CancunTimeOffset = new(hexutil.Uint64)
}
