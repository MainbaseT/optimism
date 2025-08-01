package interopgen

import (
	"errors"
	"fmt"
	"math/big"
	"sort"

	"golang.org/x/exp/maps"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-chain-ops/foundry"
	"github.com/ethereum-optimism/optimism/op-chain-ops/genesis"
	"github.com/ethereum-optimism/optimism/op-chain-ops/genesis/beacondeposit"
	"github.com/ethereum-optimism/optimism/op-chain-ops/script"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/manage"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/opcm"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

var (
	// sysGenesisDeployer is used as tx.origin/msg.sender on system genesis script calls.
	// At the end we verify none of the deployed contracts persist (there may be temporary ones, to insert bytecode).
	sysGenesisDeployer = common.Address(crypto.Keccak256([]byte("System genesis deployer"))[12:])
)

func Deploy(logger log.Logger, fa *foundry.ArtifactsFS, srcFS *foundry.SourceMapFS, cfg *WorldConfig) (*WorldDeployment, *WorldOutput, error) {
	// Sanity check all L2s have consistent chain ID and attach to the same L1
	for id, l2Cfg := range cfg.L2s {
		if fmt.Sprintf("%d", l2Cfg.L2ChainID) != id {
			return nil, nil, fmt.Errorf("chain L2 %s declared different L2 chain ID %d in config", id, l2Cfg.L2ChainID)
		}
		if !cfg.L1.ChainID.IsUint64() || cfg.L1.ChainID.Uint64() != l2Cfg.L1ChainID {
			return nil, nil, fmt.Errorf("chain L2 %s declared different L1 chain ID %d in config than global %d", id, l2Cfg.L1ChainID, cfg.L1.ChainID)
		}
	}

	deployments := &WorldDeployment{
		L2s: make(map[string]*L2Deployment),
	}

	l1Host := CreateL1(logger, fa, srcFS, cfg.L1)
	if err := l1Host.EnableCheats(); err != nil {
		return nil, nil, fmt.Errorf("failed to enable cheats in L1 state: %w", err)
	}

	//
	// Gather all the deployment scripts
	//
	// Loading all deployment scripts should happen before we start deploying anything
	// and after we have access to the contract artifacts.
	//
	// If done this way, any errors (such as ABI mismatches) will be caught before the first transaction is sent.
	//
	opcmScripts, err := opcm.NewScripts(l1Host)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load OPCM script: %w", err)
	}

	l1Deployment, err := PrepareInitialL1(l1Host, cfg.L1)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to deploy initial L1 content: %w", err)
	}
	deployments.L1 = l1Deployment

	superDeployment, err := DeploySuperchainToL1(l1Host, opcmScripts, cfg.Superchain)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to deploy superchain to L1: %w", err)
	}
	deployments.Superchain = superDeployment

	// We deploy contracts for each L2 to the L1
	// because we need to compute the genesis block hash
	// to put into the L2 genesis configs, and can thus not mutate the L1 state
	// after creating the final config for any particular L2. Will add comments.

	for l2ChainID, l2Cfg := range cfg.L2s {
		l2Deployment, err := DeployL2ToL1(l1Host, cfg.Superchain, superDeployment, l2Cfg)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to deploy L2 %d to L1: %w", &l2ChainID, err)
		}
		deployments.L2s[l2ChainID] = l2Deployment
	}

	interopDeployment, err := MigrateInterop(l1Host, uint64(cfg.L1.L1GenesisBlockTimestamp), cfg.Superchain, superDeployment, cfg.L2s, deployments.L2s)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to migrate interop: %w", err)
	}
	deployments.Interop = interopDeployment

	out := &WorldOutput{
		L2s: make(map[string]*L2Output),
	}
	l1Out, err := CompleteL1(l1Host, cfg.L1)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to complete L1: %w", err)
	}
	out.L1 = l1Out

	// Now that the L1 does not change anymore we can compute the L1 genesis block, to anchor all the L2s to.
	l1GenesisBlock := l1Out.Genesis.ToBlock()
	genesisTimestamp := l1Out.Genesis.Timestamp

	for l2ChainID, l2Cfg := range cfg.L2s {
		l2Host := CreateL2(logger, fa, srcFS, l2Cfg, genesisTimestamp)
		if err := l2Host.EnableCheats(); err != nil {
			return nil, nil, fmt.Errorf("failed to enable cheats in L2 state %s: %w", l2ChainID, err)
		}
		if err := GenesisL2(l2Host, l2Cfg, deployments.L2s[l2ChainID], len(cfg.L2s) > 1); err != nil {
			return nil, nil, fmt.Errorf("failed to apply genesis data to L2 %s: %w", l2ChainID, err)
		}
		l2Out, err := CompleteL2(l2Host, l2Cfg, l1GenesisBlock, deployments.L2s[l2ChainID])
		if err != nil {
			return nil, nil, fmt.Errorf("failed to complete L2 %s: %w", l2ChainID, err)
		}
		out.L2s[l2ChainID] = l2Out
	}
	return deployments, out, nil
}

func CreateL1(logger log.Logger, fa *foundry.ArtifactsFS, srcFS *foundry.SourceMapFS, cfg *L1Config) *script.Host {
	l1Context := script.Context{
		ChainID:      cfg.ChainID,
		Sender:       sysGenesisDeployer,
		Origin:       sysGenesisDeployer,
		FeeRecipient: common.Address{},
		GasLimit:     script.DefaultFoundryGasLimit,
		BlockNum:     uint64(cfg.L1GenesisBlockNumber),
		Timestamp:    uint64(cfg.L1GenesisBlockTimestamp),
		PrevRandao:   cfg.L1GenesisBlockMixHash,
		BlobHashes:   nil,
	}
	l1Host := script.NewHost(logger.New("role", "l1", "chain", cfg.ChainID), fa, srcFS, l1Context, script.WithCreate2Deployer())
	return l1Host
}

func CreateL2(logger log.Logger, fa *foundry.ArtifactsFS, srcFS *foundry.SourceMapFS, l2Cfg *L2Config, genesisTimestamp uint64) *script.Host {
	l2Context := script.Context{
		ChainID:      new(big.Int).SetUint64(l2Cfg.L2ChainID),
		Sender:       sysGenesisDeployer,
		Origin:       sysGenesisDeployer,
		FeeRecipient: common.Address{},
		GasLimit:     script.DefaultFoundryGasLimit,
		BlockNum:     uint64(l2Cfg.L2GenesisBlockNumber),
		Timestamp:    genesisTimestamp,
		PrevRandao:   l2Cfg.L2GenesisBlockMixHash,
		BlobHashes:   nil,
	}
	l2Host := script.NewHost(logger.New("role", "l2", "chain", l2Cfg.L2ChainID), fa, srcFS, l2Context)
	l2Host.SetEnvVar("OUTPUT_MODE", "none") // we don't use the cheatcode, but capture the state outside of EVM execution
	l2Host.SetEnvVar("FORK", "holocene")    // latest fork
	return l2Host
}

// PrepareInitialL1 deploys basics such as preinstalls to L1  (incl. EIP-4788)
func PrepareInitialL1(l1Host *script.Host, cfg *L1Config) (*L1Deployment, error) {
	l1Host.SetTxOrigin(sysGenesisDeployer)

	if err := opcm.InsertPreinstalls(l1Host); err != nil {
		return nil, fmt.Errorf("failed to install preinstalls in L1: %w", err)
	}
	// No global contracts inserted at this point.
	// All preinstalls have known constant addresses.
	return &L1Deployment{}, nil
}

func DeploySuperchainToL1(l1Host *script.Host, opcmScripts *opcm.Scripts, superCfg *SuperchainConfig) (*SuperchainDeployment, error) {
	l1Host.SetTxOrigin(superCfg.Deployer)

	superDeployment, err := opcmScripts.DeploySuperchain.Run(opcm.DeploySuperchainInput{
		SuperchainProxyAdminOwner:  superCfg.ProxyAdminOwner,
		ProtocolVersionsOwner:      superCfg.ProtocolVersionsOwner,
		Guardian:                   superCfg.SuperchainConfigGuardian,
		Paused:                     superCfg.Paused,
		RequiredProtocolVersion:    superCfg.RequiredProtocolVersion,
		RecommendedProtocolVersion: superCfg.RecommendedProtocolVersion,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to deploy Superchain contracts: %w", err)
	}

	implementationsDeployment, err := opcmScripts.DeployImplementations.Run(opcm.DeployImplementationsInput{
		WithdrawalDelaySeconds:          superCfg.Implementations.FaultProof.WithdrawalDelaySeconds,
		MinProposalSizeBytes:            superCfg.Implementations.FaultProof.MinProposalSizeBytes,
		ChallengePeriodSeconds:          superCfg.Implementations.FaultProof.ChallengePeriodSeconds,
		ProofMaturityDelaySeconds:       superCfg.Implementations.FaultProof.ProofMaturityDelaySeconds,
		DisputeGameFinalityDelaySeconds: superCfg.Implementations.FaultProof.DisputeGameFinalityDelaySeconds,
		MipsVersion:                     superCfg.Implementations.FaultProof.MipsVersion,
		L1ContractsRelease:              superCfg.Implementations.L1ContractsRelease,
		SuperchainProxyAdmin:            superDeployment.SuperchainProxyAdmin,
		SuperchainConfigProxy:           superDeployment.SuperchainConfigProxy,
		ProtocolVersionsProxy:           superDeployment.ProtocolVersionsProxy,
		UpgradeController:               superCfg.ProxyAdminOwner,
		Challenger:                      superCfg.Challenger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to deploy Implementations contracts: %w", err)
	}

	// Collect deployment addresses
	// This could all be automatic once we have better output-contract typing/scripting
	return &SuperchainDeployment{
		Implementations:       Implementations(implementationsDeployment),
		ProxyAdmin:            superDeployment.SuperchainProxyAdmin,
		ProtocolVersions:      superDeployment.ProtocolVersionsImpl,
		ProtocolVersionsProxy: superDeployment.ProtocolVersionsProxy,
		SuperchainConfig:      superDeployment.SuperchainConfigImpl,
		SuperchainConfigProxy: superDeployment.SuperchainConfigProxy,
	}, nil
}

func DeployL2ToL1(l1Host *script.Host, superCfg *SuperchainConfig, superDeployment *SuperchainDeployment, cfg *L2Config) (*L2Deployment, error) {
	if cfg.UseAltDA {
		return nil, errors.New("alt-da mode not supported yet")
	}

	l1Host.SetTxOrigin(cfg.Deployer)

	output, err := opcm.DeployOPChain(l1Host, opcm.DeployOPChainInput{
		OpChainProxyAdminOwner:       superCfg.ProxyAdminOwner,
		SystemConfigOwner:            cfg.SystemConfigOwner,
		Batcher:                      cfg.BatchSenderAddress,
		UnsafeBlockSigner:            cfg.P2PSequencerAddress,
		Proposer:                     cfg.Proposer,
		Challenger:                   cfg.Challenger,
		BasefeeScalar:                cfg.GasPriceOracleBaseFeeScalar,
		BlobBaseFeeScalar:            cfg.GasPriceOracleBlobBaseFeeScalar,
		L2ChainId:                    new(big.Int).SetUint64(cfg.L2ChainID),
		Opcm:                         superDeployment.Opcm,
		SaltMixer:                    cfg.SaltMixer,
		GasLimit:                     cfg.GasLimit,
		DisputeGameType:              cfg.DisputeGameType,
		DisputeAbsolutePrestate:      cfg.DisputeAbsolutePrestate,
		DisputeMaxGameDepth:          cfg.DisputeMaxGameDepth,
		DisputeSplitDepth:            cfg.DisputeSplitDepth,
		DisputeClockExtension:        cfg.DisputeClockExtension,
		DisputeMaxClockDuration:      cfg.DisputeMaxClockDuration,
		AllowCustomDisputeParameters: true,
		OperatorFeeScalar:            cfg.GasPriceOracleOperatorFeeScalar,
		OperatorFeeConstant:          cfg.GasPriceOracleOperatorFeeConstant,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to deploy L2 OP chain: %w", err)
	}

	// Collect deployment addresses
	return &L2Deployment{
		L2OpchainDeployment: L2OpchainDeployment(output),
	}, nil
}

func MigrateInterop(
	l1Host *script.Host, l1GenesisTimestamp uint64, superCfg *SuperchainConfig, superDeployment *SuperchainDeployment, l2Cfgs map[string]*L2Config, l2Deployments map[string]*L2Deployment,
) (*InteropDeployment, error) {
	l2ChainIDs := maps.Keys(l2Deployments)
	sort.Strings(l2ChainIDs)
	chainConfigs := make([]manage.OPChainConfig, len(l2Deployments))
	for i, l2ChainID := range l2ChainIDs {
		l2Deployment := l2Deployments[l2ChainID]
		chainConfigs[i] = manage.OPChainConfig{
			SystemConfigProxy: l2Deployment.SystemConfigProxy,
			ProxyAdmin:        superDeployment.ProxyAdmin,
			AbsolutePrestate:  l2Cfgs[l2ChainID].DisputeAbsolutePrestate,
		}
	}

	// For now get the fault game parameters from the first chain
	l2ChainID := l2ChainIDs[0]
	// We don't have a super root at genesis. But stub the starting anchor root anyways to facilitate super DG testing.
	startingAnchorRoot := common.Hash(opcm.PermissionedGameStartingAnchorRoot)
	imi := manage.InteropMigrationInput{
		Prank:                          superCfg.ProxyAdminOwner,
		Opcm:                           superDeployment.Opcm,
		UsePermissionlessGame:          true,
		StartingAnchorRoot:             startingAnchorRoot,
		StartingAnchorL2SequenceNumber: big.NewInt(int64(l1GenesisTimestamp)),
		Proposer:                       l2Cfgs[l2ChainID].Proposer,
		Challenger:                     l2Cfgs[l2ChainID].Challenger,
		MaxGameDepth:                   l2Cfgs[l2ChainID].DisputeMaxGameDepth,
		SplitDepth:                     l2Cfgs[l2ChainID].DisputeSplitDepth,
		InitBond:                       big.NewInt(0),
		ClockExtension:                 l2Cfgs[l2ChainID].DisputeClockExtension,
		MaxClockDuration:               l2Cfgs[l2ChainID].DisputeMaxClockDuration,
		EncodedChainConfigs:            chainConfigs,
	}
	output, err := manage.Migrate(l1Host, imi)
	if err != nil {
		return nil, fmt.Errorf("failed to migrate interop: %w", err)
	}

	return &InteropDeployment{
		DisputeGameFactory: output.DisputeGameFactory,
	}, nil
}

func GenesisL2(l2Host *script.Host, cfg *L2Config, deployment *L2Deployment, multichainDepSet bool) error {
	genesisScript, err := opcm.NewL2GenesisScript(l2Host)
	if err != nil {
		return fmt.Errorf("failed to create L2 genesis script: %w", err)
	}

	if err := genesisScript.Run(opcm.L2GenesisInput{
		L1ChainID:                                new(big.Int).SetUint64(cfg.L1ChainID),
		L2ChainID:                                new(big.Int).SetUint64(cfg.L2ChainID),
		L1CrossDomainMessengerProxy:              deployment.L1CrossDomainMessengerProxy,
		L1StandardBridgeProxy:                    deployment.L1StandardBridgeProxy,
		L1ERC721BridgeProxy:                      deployment.L1ERC721BridgeProxy,
		OpChainProxyAdminOwner:                   cfg.ProxyAdminOwner,
		SequencerFeeVaultRecipient:               cfg.SequencerFeeVaultRecipient,
		SequencerFeeVaultMinimumWithdrawalAmount: cfg.SequencerFeeVaultMinimumWithdrawalAmount.ToInt(),
		SequencerFeeVaultWithdrawalNetwork:       big.NewInt(int64(cfg.SequencerFeeVaultWithdrawalNetwork.ToUint8())),
		BaseFeeVaultRecipient:                    cfg.BaseFeeVaultRecipient,
		BaseFeeVaultMinimumWithdrawalAmount:      cfg.BaseFeeVaultMinimumWithdrawalAmount.ToInt(),
		BaseFeeVaultWithdrawalNetwork:            big.NewInt(int64(cfg.BaseFeeVaultWithdrawalNetwork.ToUint8())),
		L1FeeVaultRecipient:                      cfg.L1FeeVaultRecipient,
		L1FeeVaultMinimumWithdrawalAmount:        cfg.L1FeeVaultMinimumWithdrawalAmount.ToInt(),
		L1FeeVaultWithdrawalNetwork:              big.NewInt(int64(cfg.L1FeeVaultWithdrawalNetwork.ToUint8())),
		GovernanceTokenOwner:                     cfg.GovernanceTokenOwner,
		Fork:                                     big.NewInt(cfg.SolidityForkNumber(1)),
		DeployCrossL2Inbox:                       multichainDepSet,
		EnableGovernance:                         cfg.EnableGovernance,
		FundDevAccounts:                          cfg.FundDevAccounts,
	}); err != nil {
		return fmt.Errorf("failed L2 genesis: %w", err)
	}

	return nil
}

func CompleteL1(l1Host *script.Host, cfg *L1Config) (*L1Output, error) {
	l1Genesis, err := genesis.NewL1Genesis(&genesis.DeployConfig{
		L2InitializationConfig: genesis.L2InitializationConfig{
			L2CoreDeployConfig: genesis.L2CoreDeployConfig{
				L1ChainID: cfg.ChainID.Uint64(),
			},
			UpgradeScheduleDeployConfig: genesis.UpgradeScheduleDeployConfig{
				L1CancunTimeOffset: new(hexutil.Uint64),
			},
		},
		DevL1DeployConfig: cfg.DevL1DeployConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to build L1 genesis template: %w", err)
	}
	allocs, err := l1Host.StateDump()
	if err != nil {
		return nil, fmt.Errorf("failed to dump L1 state: %w", err)
	}

	// Sanity check that the default deployer didn't include anything,
	// and make sure it's not in the state.
	if err := ensureNoDeployed(allocs, sysGenesisDeployer); err != nil {
		return nil, fmt.Errorf("unexpected deployed account content by L1 genesis deployer: %w", err)
	}

	for addr, amount := range cfg.Prefund {
		acc := allocs.Accounts[addr]
		acc.Balance = amount
		allocs.Accounts[addr] = acc
	}

	l1Genesis.Alloc = allocs.Accounts

	// Insert an empty beaconchain deposit contract with valid empty-tree prestate.
	// This is part of dev-genesis, but not part of scripts yet.
	beaconDepositAddr := common.HexToAddress("0x1111111111111111111111111111111111111111")
	if err := beacondeposit.InsertEmptyBeaconDepositContract(l1Genesis, beaconDepositAddr); err != nil {
		return nil, fmt.Errorf("failed to insert beacon deposit contract into L1 dev genesis: %w", err)
	}

	return &L1Output{
		Genesis: l1Genesis,
	}, nil
}

func CompleteL2(l2Host *script.Host, cfg *L2Config, l1Block *types.Block, deployment *L2Deployment) (*L2Output, error) {
	deployCfg := &genesis.DeployConfig{
		L2InitializationConfig: cfg.L2InitializationConfig,
		L1DependenciesConfig: genesis.L1DependenciesConfig{
			L1StandardBridgeProxy:       deployment.L1StandardBridgeProxy,
			L1CrossDomainMessengerProxy: deployment.L1CrossDomainMessengerProxy,
			L1ERC721BridgeProxy:         deployment.L1ERC721BridgeProxy,
			SystemConfigProxy:           deployment.SystemConfigProxy,
			OptimismPortalProxy:         deployment.OptimismPortalProxy,
			DAChallengeProxy:            common.Address{}, // unsupported for now
		},
	}
	// l1Block is used to determine genesis time.
	l2Genesis, err := genesis.NewL2Genesis(deployCfg, eth.BlockRefFromHeader(l1Block.Header()))
	if err != nil {
		return nil, fmt.Errorf("failed to build L2 genesis config: %w", err)
	}

	allocs, err := l2Host.StateDump()
	if err != nil {
		return nil, fmt.Errorf("failed to dump L1 state: %w", err)
	}

	// Sanity check that the default deployer didn't include anything,
	// and make sure it's not in the state.
	if err := ensureNoDeployed(allocs, sysGenesisDeployer); err != nil {
		return nil, fmt.Errorf("unexpected deployed account content by L2 genesis deployer: %w", err)
	}

	for addr, amount := range cfg.Prefund {
		acc := allocs.Accounts[addr]
		acc.Balance = amount
		allocs.Accounts[addr] = acc
	}

	for addr, account := range allocs.Accounts {
		l2Genesis.Alloc[addr] = account
	}
	l2GenesisBlock := l2Genesis.ToBlock()

	rollupCfg, err := deployCfg.RollupConfig(eth.BlockRefFromHeader(l1Block.Header()), l2GenesisBlock.Hash(), l2GenesisBlock.NumberU64())
	if err != nil {
		return nil, fmt.Errorf("failed to build L2 rollup config: %w", err)
	}
	return &L2Output{
		Genesis:   l2Genesis,
		RollupCfg: rollupCfg,
	}, nil
}

// ensureNoDeployed checks that non of the contracts that
// could have been deployed by the given deployer address are around.
// And removes deployer from the allocs.
func ensureNoDeployed(allocs *foundry.ForgeAllocs, deployer common.Address) error {
	// Sanity check we have no deploy output that's not meant to be there.
	for i := uint64(0); i <= allocs.Accounts[deployer].Nonce; i++ {
		addr := crypto.CreateAddress(deployer, i)
		if _, ok := allocs.Accounts[addr]; ok {
			return fmt.Errorf("system deployer output %s (deployed with nonce %d) was not cleaned up", addr, i)
		}
	}
	// Don't include the deployer account
	delete(allocs.Accounts, deployer)
	return nil
}
