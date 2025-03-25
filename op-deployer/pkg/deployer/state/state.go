package state

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/broadcaster"

	"github.com/ethereum-optimism/optimism/op-chain-ops/foundry"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/ioutil"
	"github.com/ethereum-optimism/optimism/op-service/jsonutil"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
)

// State contains the data needed to recreate the deployment
// as it progresses and once it is fully applied.
type State struct {
	// Version versions the state so we can update it later.
	Version int `json:"version"`

	// Create2Salt is the salt used for CREATE2 deployments.
	Create2Salt common.Hash `json:"create2Salt"`

	// AppliedIntent contains the chain intent that was last
	// successfully applied. It is diffed against new intent
	// in order to determine what deployment steps to take.
	// This field is nil for new deployments.
	AppliedIntent *Intent `json:"appliedIntent"`

	// SuperchainDeployment contains the addresses of the Superchain
	// deployment. It only contains the proxies because the implementations
	// can be looked up on chain.
	SuperchainDeployment *SuperchainDeployment `json:"superchainDeployment"`

	// ImplementationsDeployment contains the addresses of the common implementation
	// contracts required for the Superchain to function.
	ImplementationsDeployment *ImplementationsDeployment `json:"implementationsDeployment"`

	// Chains contains data about L2 chain deployments.
	Chains []*ChainState `json:"opChainDeployments"`

	// L1StateDump contains the complete L1 state dump of the deployment.
	L1StateDump *GzipData[foundry.ForgeAllocs] `json:"l1StateDump"`

	// DeploymentCalldata contains the calldata of each transaction in the deployment. This is only
	// populated if apply is called with --deployment-target=calldata.
	DeploymentCalldata []broadcaster.CalldataDump
}

func (s *State) WriteToFile(path string) error {
	return jsonutil.WriteJSON(s, ioutil.ToAtomicFile(path, 0o755))
}

func (s *State) Chain(id common.Hash) (*ChainState, error) {
	for _, chain := range s.Chains {
		if chain.ID == id {
			return chain, nil
		}
	}
	return nil, fmt.Errorf("chain not found: %s", id.Hex())
}

type SuperchainDeployment struct {
	ProxyAdminAddress            common.Address `json:"proxyAdminAddress"`
	SuperchainConfigProxyAddress common.Address `json:"superchainConfigProxyAddress"`
	SuperchainConfigImplAddress  common.Address `json:"superchainConfigImplAddress"`
	ProtocolVersionsProxyAddress common.Address `json:"protocolVersionsProxyAddress"`
	ProtocolVersionsImplAddress  common.Address `json:"protocolVersionsImplAddress"`
}

type ImplementationsDeployment struct {
	OpcmAddress                             common.Address `json:"opcmAddress"`
	OpcmGameTypeAdderAddress                common.Address `json:"opcmGameTypeAdderAddress"`
	OpcmDeployerAddress                     common.Address `json:"opcmDeployerAddress"`
	OpcmUpgraderAddress                     common.Address `json:"opcmUpgraderAddress"`
	OpcmInteropMigratorAddress              common.Address `json:"opcmInteropMigratorAddress"`
	DelayedWETHImplAddress                  common.Address `json:"delayedWETHImplAddress"`
	OptimismPortalImplAddress               common.Address `json:"optimismPortalImplAddress"`
	ETHLockboxImplAddress                   common.Address `json:"ethLockboxImplAddress"`
	PreimageOracleSingletonAddress          common.Address `json:"preimageOracleSingletonAddress"`
	MipsSingletonAddress                    common.Address `json:"mipsSingletonAddress"`
	SystemConfigImplAddress                 common.Address `json:"systemConfigImplAddress"`
	L1CrossDomainMessengerImplAddress       common.Address `json:"l1CrossDomainMessengerImplAddress"`
	L1ERC721BridgeImplAddress               common.Address `json:"l1ERC721BridgeImplAddress"`
	L1StandardBridgeImplAddress             common.Address `json:"l1StandardBridgeImplAddress"`
	OptimismMintableERC20FactoryImplAddress common.Address `json:"optimismMintableERC20FactoryImplAddress"`
	DisputeGameFactoryImplAddress           common.Address `json:"disputeGameFactoryImplAddress"`
	AnchorStateRegistryImplAddress          common.Address `json:"anchorStateRegistryImplAddress"`
}

type AdditionalDisputeGameState struct {
	GameType      uint32
	GameAddress   common.Address
	VMAddress     common.Address
	OracleAddress common.Address
	VMType        VMType
}

type ChainState struct {
	ID common.Hash `json:"id"`

	ProxyAdminAddress                         common.Address               `json:"proxyAdminAddress"`
	AddressManagerAddress                     common.Address               `json:"addressManagerAddress"`
	L1ERC721BridgeProxyAddress                common.Address               `json:"l1ERC721BridgeProxyAddress"`
	SystemConfigProxyAddress                  common.Address               `json:"systemConfigProxyAddress"`
	OptimismMintableERC20FactoryProxyAddress  common.Address               `json:"optimismMintableERC20FactoryProxyAddress"`
	L1StandardBridgeProxyAddress              common.Address               `json:"l1StandardBridgeProxyAddress"`
	L1CrossDomainMessengerProxyAddress        common.Address               `json:"l1CrossDomainMessengerProxyAddress"`
	OptimismPortalProxyAddress                common.Address               `json:"optimismPortalProxyAddress"`
	ETHLockboxProxyAddress                    common.Address               `json:"ethLockboxProxyAddress"`
	DisputeGameFactoryProxyAddress            common.Address               `json:"disputeGameFactoryProxyAddress"`
	AnchorStateRegistryProxyAddress           common.Address               `json:"anchorStateRegistryProxyAddress"`
	FaultDisputeGameAddress                   common.Address               `json:"faultDisputeGameAddress"`
	PermissionedDisputeGameAddress            common.Address               `json:"permissionedDisputeGameAddress"`
	DelayedWETHPermissionedGameProxyAddress   common.Address               `json:"delayedWETHPermissionedGameProxyAddress"`
	DelayedWETHPermissionlessGameProxyAddress common.Address               `json:"delayedWETHPermissionlessGameProxyAddress"`
	DataAvailabilityChallengeProxyAddress     common.Address               `json:"dataAvailabilityChallengeProxyAddress"`
	DataAvailabilityChallengeImplAddress      common.Address               `json:"dataAvailabilityChallengeImplAddress"`
	AdditionalDisputeGames                    []AdditionalDisputeGameState `json:"additionalDisputeGames"`

	Allocs *GzipData[foundry.ForgeAllocs] `json:"allocs"`

	StartBlock *L1BlockRefJSON `json:"startBlock"`
}

type L1BlockRefJSON struct {
	Hash       common.Hash    `json:"hash"`
	ParentHash common.Hash    `json:"parentHash"`
	Number     hexutil.Uint64 `json:"number"`
	Time       hexutil.Uint64 `json:"timestamp"`
}

func (b *L1BlockRefJSON) ToBlockRef() *eth.BlockRef {
	return &eth.BlockRef{
		Hash:       b.Hash,
		Number:     uint64(b.Number),
		ParentHash: b.ParentHash,
		Time:       uint64(b.Time),
	}
}

func BlockRefJsonFromBlockRef(br *eth.BlockRef) *L1BlockRefJSON {
	return &L1BlockRefJSON{
		Hash:       br.Hash,
		Number:     hexutil.Uint64(br.Number),
		ParentHash: br.ParentHash,
		Time:       hexutil.Uint64(br.Time),
	}
}

func BlockRefJsonFromHeader(h *types.Header) *L1BlockRefJSON {
	return BlockRefJsonFromBlockRef(eth.BlockRefFromHeader(h))
}
