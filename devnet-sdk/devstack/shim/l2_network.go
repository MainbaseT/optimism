package shim

import (
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/devnet-sdk/devstack/stack"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/locks"
)

type L2NetworkConfig struct {
	NetworkConfig
	ID           stack.L2NetworkID
	RollupConfig *rollup.Config
	Deployment   stack.L2Deployment
	Keys         stack.L2Keys

	Superchain stack.Superchain
	L1         stack.L1Network
	Cluster    stack.Cluster
}

type presetL2Network struct {
	presetNetwork
	id stack.L2NetworkID

	rollupCfg  *rollup.Config
	deployment stack.L2Deployment
	keys       stack.L2Keys

	superchain stack.Superchain
	l1         stack.L1Network
	cluster    stack.Cluster

	batchers    locks.RWMap[stack.L2BatcherID, stack.L2Batcher]
	proposers   locks.RWMap[stack.L2ProposerID, stack.L2Proposer]
	challengers locks.RWMap[stack.L2ChallengerID, stack.L2Challenger]

	els locks.RWMap[stack.L2ELNodeID, stack.L2ELNode]
	cls locks.RWMap[stack.L2CLNodeID, stack.L2CLNode]
}

var _ stack.L2Network = (*presetL2Network)(nil)

func NewL2Network(cfg L2NetworkConfig) stack.ExtensibleL2Network {
	// sanity-check the configs match the expected chains
	require.Equal(cfg.T, cfg.ID.ChainID, eth.ChainIDFromBig(cfg.NetworkConfig.ChainConfig.ChainID), "chain config must match expected chain")
	require.Equal(cfg.T, cfg.L1.ChainID(), eth.ChainIDFromBig(cfg.RollupConfig.L1ChainID), "rollup config must match expected L1 chain")
	require.Equal(cfg.T, cfg.ID.ChainID, eth.ChainIDFromBig(cfg.RollupConfig.L2ChainID), "rollup config must match expected L2 chain")
	cfg.Log = cfg.Log.New("chainID", cfg.ID.ChainID, "id", cfg.ID)
	return &presetL2Network{
		id:            cfg.ID,
		presetNetwork: newNetwork(cfg.NetworkConfig),
		rollupCfg:     cfg.RollupConfig,
		deployment:    cfg.Deployment,
		keys:          cfg.Keys,
		superchain:    cfg.Superchain,
		l1:            cfg.L1,
		cluster:       cfg.Cluster,
	}
}

func (p *presetL2Network) ID() stack.L2NetworkID {
	return p.id
}

func (p *presetL2Network) RollupConfig() *rollup.Config {
	p.require().NotNil(p.rollupCfg, "l2 chain %s must have a rollup config", p.ID())
	return p.rollupCfg
}

func (p *presetL2Network) Deployment() stack.L2Deployment {
	p.require().NotNil(p.deployment, "l2 chain %s must have a deployment", p.ID())
	return p.deployment
}

func (p *presetL2Network) Keys() stack.L2Keys {
	p.require().NotNil(p.keys, "l2 chain %s must have keys", p.ID())
	return p.keys
}

func (p *presetL2Network) Superchain() stack.Superchain {
	p.require().NotNil(p.superchain, "l2 chain %s must have a superchain", p.ID())
	return p.superchain
}

func (p *presetL2Network) L1() stack.L1Network {
	p.require().NotNil(p.l1, "l2 chain %s must have an L1 chain", p.ID())
	return p.l1
}

func (p *presetL2Network) Cluster() stack.Cluster {
	p.require().NotNil(p.cluster, "l2 chain %s must have a cluster", p.ID())
	return p.cluster
}

func (p *presetL2Network) L2Batcher(id stack.L2BatcherID) stack.L2Batcher {
	v, ok := p.batchers.Get(id)
	p.require().True(ok, "l2 batcher %s must exist", id)
	return v
}

func (p *presetL2Network) AddL2Batcher(v stack.L2Batcher) {
	id := v.ID()
	p.require().Equal(p.chainID, id.ChainID, "l2 batcher %s must be on chain %s", id, p.chainID)
	p.require().True(p.batchers.SetIfMissing(id, v), "l2 batcher %s must not already exist", id)
}

func (p *presetL2Network) L2Proposer(id stack.L2ProposerID) stack.L2Proposer {
	v, ok := p.proposers.Get(id)
	p.require().True(ok, "l2 proposer %s must exist", id)
	return v
}

func (p *presetL2Network) AddL2Proposer(v stack.L2Proposer) {
	id := v.ID()
	p.require().Equal(p.chainID, id.ChainID, "l2 proposer %s must be on chain %s", id, p.chainID)
	p.require().True(p.proposers.SetIfMissing(id, v), "l2 proposer %s must not already exist", id)
}

func (p *presetL2Network) L2Challenger(id stack.L2ChallengerID) stack.L2Challenger {
	v, ok := p.challengers.Get(id)
	p.require().True(ok, "l2 challenger %s must exist", id)
	return v
}

func (p *presetL2Network) AddL2Challenger(v stack.L2Challenger) {
	id := v.ID()
	p.require().Equal(p.chainID, id.ChainID, "l2 challenger %s must be on chain %s", id, p.chainID)
	p.require().True(p.challengers.SetIfMissing(id, v), "l2 challenger %s must not already exist", id)
}

func (p *presetL2Network) L2CLNode(id stack.L2CLNodeID) stack.L2CLNode {
	v, ok := p.cls.Get(id)
	p.require().True(ok, "l2 CL node %s must exist", id)
	return v
}

func (p *presetL2Network) AddL2CLNode(v stack.L2CLNode) {
	id := v.ID()
	p.require().Equal(p.chainID, id.ChainID, "l2 CL node %s must be on chain %s", id, p.chainID)
	p.require().True(p.cls.SetIfMissing(id, v), "l2 CL node %s must not already exist", id)
}

func (p *presetL2Network) L2ELNode(id stack.L2ELNodeID) stack.L2ELNode {
	v, ok := p.els.Get(id)
	p.require().True(ok, "l2 EL node %s must exist", id)
	return v
}

func (p *presetL2Network) AddL2ELNode(v stack.L2ELNode) {
	id := v.ID()
	p.require().Equal(p.chainID, id.ChainID, "l2 EL node %s must be on chain %s", id, p.chainID)
	p.require().True(p.els.SetIfMissing(id, v), "l2 EL node %s must not already exist", id)
}

func (p *presetL2Network) L2Batchers() []stack.L2BatcherID {
	return stack.SortL2BatcherIDs(p.batchers.Keys())
}

func (p *presetL2Network) L2Proposers() []stack.L2ProposerID {
	return stack.SortL2ProposerIDs(p.proposers.Keys())
}

func (p *presetL2Network) L2Challengers() []stack.L2ChallengerID {
	return stack.SortL2ChallengerIDs(p.challengers.Keys())
}

func (p *presetL2Network) L2CLNodes() []stack.L2CLNodeID {
	return stack.SortL2CLNodeIDs(p.cls.Keys())
}

func (p *presetL2Network) L2ELNodes() []stack.L2ELNodeID {
	return stack.SortL2ELNodeIDs(p.els.Keys())
}
