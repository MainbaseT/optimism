package sysgo

import (
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/devnet-sdk/devstack/devtest"
	"github.com/ethereum-optimism/optimism/devnet-sdk/devstack/stack"
	"github.com/ethereum-optimism/optimism/op-chain-ops/devkeys"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/artifacts"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/inspect"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/state"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/intentbuilder"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	supervisortypes "github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type DeployerOption func(p devtest.P, keys devkeys.Keys, builder intentbuilder.Builder)

func WithDeployer(opts ...DeployerOption) stack.Option {
	return func(o stack.Orchestrator) {
		orch := o.(*Orchestrator)
		require := o.P().Require()

		wb := &worldBuilder{
			p:       o.P(),
			logger:  o.P().Logger(),
			require: o.P().Require(),
			keys:    orch.keys,
			builder: intentbuilder.New(),
		}
		for _, opt := range opts {
			opt(o.P(), orch.keys, wb.builder)
		}
		wb.Build()

		l1ID := stack.L1NetworkID(eth.ChainIDFromUInt64(wb.output.AppliedIntent.L1ChainID))
		superchainID := stack.SuperchainID("main")
		clusterID := stack.ClusterID("main")

		l1Net := &L1Network{
			id:        l1ID,
			genesis:   wb.outL1Genesis,
			blockTime: 6,
		}
		orch.l1Nets.Set(l1ID.ChainID(), l1Net)

		orch.superchains.Set(superchainID, &Superchain{
			id:         superchainID,
			deployment: wb.outSuperchainDeployment,
		})
		orch.clusters.Set(clusterID, &Cluster{
			id:     clusterID,
			depset: wb.outDepset,
		})

		for _, chainID := range wb.l2Chains {
			l2Genesis, ok := wb.outL2Genesis[chainID]
			require.True(ok, "L2 genesis must exist")
			l2RollupCfg, ok := wb.outL2RollupCfg[chainID]
			require.True(ok, "L2 rollup config must exist")
			l2Dep, ok := wb.outL2Deployment[chainID]
			require.True(ok, "L2 deployment must exist")

			l2ID := stack.L2NetworkID(chainID)
			l2Net := &L2Network{
				id:         l2ID,
				l1ChainID:  l1ID.ChainID(),
				genesis:    l2Genesis,
				rollupCfg:  l2RollupCfg,
				deployment: l2Dep,
				keys:       orch.keys,
			}
			orch.l2Nets.Set(l2ID.ChainID(), l2Net)
		}
	}
}

type worldBuilder struct {
	p devtest.P

	logger  log.Logger
	require *require.Assertions
	keys    devkeys.Keys

	builder intentbuilder.Builder

	output                  *state.State
	outL1Genesis            *core.Genesis
	l2Chains                []eth.ChainID
	outL2Genesis            map[eth.ChainID]*core.Genesis
	outL2RollupCfg          map[eth.ChainID]*rollup.Config
	outL2Deployment         map[eth.ChainID]*L2Deployment
	outDepset               *depset.StaticConfigDependencySet
	outSuperchainDeployment *SuperchainDeployment
}

var (
	oneEth     = uint256.NewInt(1e18)
	millionEth = new(uint256.Int).Mul(uint256.NewInt(1e6), oneEth)
)

func WithLocalContractSources() DeployerOption {
	return func(p devtest.P, keys devkeys.Keys, builder intentbuilder.Builder) {
		paths, err := contractPaths()
		p.Require().NoError(err)
		wd, err := os.Getwd()
		p.Require().NoError(err)
		artifactsPath := filepath.Join(wd, paths.FoundryArtifacts)
		p.Require().NoError(ensureDir(artifactsPath))
		contractArtifacts, err := artifacts.NewFileLocator(artifactsPath)
		p.Require().NoError(err)
		builder.WithL1ContractsLocator(contractArtifacts)
		builder.WithL2ContractsLocator(contractArtifacts)
	}
}

func WithCommons(l1ChainID eth.ChainID) DeployerOption {
	return func(p devtest.P, keys devkeys.Keys, builder intentbuilder.Builder) {
		_, l1Config := builder.WithL1(l1ChainID)

		l1StartTimestamp := uint64(time.Now().Unix()) + 1
		l1Config.WithTimestamp(l1StartTimestamp)

		l1Config.WithPragueOffset(0) // activate pectra on L1

		// We use the L1 chain ID to identify the superchain-wide roles.
		addrFor := intentbuilder.RoleToAddrProvider(p, keys, l1ChainID)
		_, superCfg := builder.WithSuperchain()
		intentbuilder.WithDevkeySuperRoles(p, keys, l1ChainID, superCfg)
		l1Config.WithPrefundedAccount(addrFor(devkeys.SuperchainProxyAdminOwner), *millionEth)
		l1Config.WithPrefundedAccount(addrFor(devkeys.SuperchainProtocolVersionsOwner), *millionEth)
		l1Config.WithPrefundedAccount(addrFor(devkeys.SuperchainConfigGuardianKey), *millionEth)
	}
}

func WithPrefundedL2(chainID eth.ChainID) DeployerOption {
	return func(p devtest.P, keys devkeys.Keys, builder intentbuilder.Builder) {
		_, l2Config := builder.WithL2(chainID)
		l2Config.ChainID()

		intentbuilder.WithDevkeyVaults(p, keys, l2Config)
		intentbuilder.WithDevkeyRoles(p, keys, l2Config)

		{
			addrFor := intentbuilder.KeyToAddrProvider(p, keys)
			// unique key for this L2
			l2Config.WithPrefundedAccount(addrFor(devkeys.ChainUserKeys(chainID.ToBig())(0)), *millionEth)
		}
		{
			addrFor := intentbuilder.RoleToAddrProvider(p, keys, chainID)
			l1Config := l2Config.L1Config()
			l1Config.WithPrefundedAccount(addrFor(devkeys.BatcherRole), *millionEth)
			l1Config.WithPrefundedAccount(addrFor(devkeys.ProposerRole), *millionEth)
			l1Config.WithPrefundedAccount(addrFor(devkeys.ChallengerRole), *millionEth)
			l1Config.WithPrefundedAccount(addrFor(devkeys.SystemConfigOwner), *millionEth)
			l1Config.WithPrefundedAccount(addrFor(devkeys.L1ProxyAdminOwnerRole), *millionEth)
		}
	}
}

func (wb *worldBuilder) buildL1Genesis() {
	wb.require.NotNil(wb.output.L1DevGenesis, "must have L1 genesis outer config")
	wb.require.NotNil(wb.output.L1StateDump, "must have L1 genesis alloc")

	genesisOuter := wb.output.L1DevGenesis
	genesisAlloc := wb.output.L1StateDump.Data.Accounts
	genesisCfg := *genesisOuter
	genesisCfg.StateHash = nil
	genesisCfg.Alloc = genesisAlloc

	wb.outL1Genesis = &genesisCfg
}

func (wb *worldBuilder) buildL2Genesis() {
	wb.outL2Genesis = make(map[eth.ChainID]*core.Genesis)
	wb.outL2RollupCfg = make(map[eth.ChainID]*rollup.Config)
	for _, ch := range wb.output.Chains {
		l2Genesis, l2RollupCfg, err := inspect.GenesisAndRollup(wb.output, ch.ID)
		wb.require.NoError(err, "need L2 genesis and rollup")
		id := eth.ChainIDFromBytes32(ch.ID)
		wb.outL2Genesis[id] = l2Genesis
		wb.outL2RollupCfg[id] = l2RollupCfg
	}
}

func (wb *worldBuilder) buildDepSet() {
	if wb.output.InteropDepSet == nil {
		return
	}
	// Deployer uses a different type than the dependency-set itself, so we have to convert
	depSetContents := make(map[eth.ChainID]*depset.StaticConfigDependency)
	for _, ch := range wb.output.Chains {
		id := eth.ChainIDFromBytes32(ch.ID)
		deployerDep, ok := wb.output.InteropDepSet.Dependencies[id.String()]
		wb.require.True(ok, "expecting deployer to use stringified chain IDs")
		depSetContents[id] = &depset.StaticConfigDependency{
			ChainIndex:     supervisortypes.ChainIndex(deployerDep.ChainIndex),
			ActivationTime: deployerDep.ActivationTime,
			HistoryMinTime: deployerDep.HistoryMinTime,
		}
	}
	staticDepSet, err := depset.NewStaticConfigDependencySet(depSetContents)
	wb.require.NoError(err)
	wb.outDepset = staticDepSet
}

func (wb *worldBuilder) buildL2DeploymentOutputs() {
	wb.outL2Deployment = make(map[eth.ChainID]*L2Deployment)
	for _, ch := range wb.output.Chains {
		chainID := eth.ChainIDFromBytes32(ch.ID)
		wb.outL2Deployment[chainID] = &L2Deployment{
			systemConfigProxyAddr:   ch.SystemConfigProxyAddress,
			disputeGameFactoryProxy: ch.DisputeGameFactoryProxyAddress,
		}
	}
	wb.outSuperchainDeployment = &SuperchainDeployment{
		protocolVersionsAddr: wb.output.SuperchainDeployment.ProtocolVersionsProxyAddress,
		superchainConfigAddr: wb.output.SuperchainDeployment.SuperchainConfigProxyAddress,
	}
}

func (wb *worldBuilder) Build() {
	st := &state.State{
		Version: 1,
	}

	// Work-around of op-deployer design issue.
	// We use the same deployer key for all L1 and L2 chains we deploy here.
	deployerKey, err := wb.keys.Secret(devkeys.DeployerRole.Key(big.NewInt(0)))
	wb.require.NoError(err, "need deployer key")

	intent, err := wb.builder.Build()
	wb.require.NoError(err)

	if len(intent.Chains) > 1 { // multiple L2s implies interop
		intent.UseInterop = true
	}

	pipelineOpts := deployer.ApplyPipelineOpts{
		DeploymentTarget:   deployer.DeploymentTargetGenesis,
		L1RPCUrl:           "",
		DeployerPrivateKey: deployerKey,
		Intent:             intent,
		State:              st,
		Logger:             wb.logger,
		StateWriter:        wb, // direct output back here
	}
	err = deployer.ApplyPipeline(wb.p.Ctx(), pipelineOpts)
	wb.require.NoError(err)

	wb.require.NotNil(wb.output, "expected state-write to output")

	for _, id := range wb.output.Chains {
		chainID := eth.ChainIDFromBytes32(id.ID)
		wb.l2Chains = append(wb.l2Chains, chainID)
	}

	wb.buildL1Genesis()
	wb.buildL2Genesis()
	wb.buildL2DeploymentOutputs()
	wb.buildDepSet()
}

// WriteState is a callback used by deployer.ApplyPipeline to write the output
func (wb *worldBuilder) WriteState(st *state.State) error {
	wb.output = st
	return nil
}
