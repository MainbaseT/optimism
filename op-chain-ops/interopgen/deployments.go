package interopgen

import (
	"github.com/ethereum/go-ethereum/common"
)

type L1Deployment struct {
	// No global deployed contracts that aren't part of the superchain, yet.
}

type Implementations struct {
	Opcm                             common.Address `json:"OPCM"`
	OpcmContractsContainer           common.Address `json:"OPCMContractsContainer"`
	OpcmGameTypeAdder                common.Address `json:"OPCMGameTypeAdder"`
	OpcmDeployer                     common.Address `json:"OPCMDeployer"`
	OpcmUpgrader                     common.Address `json:"OPCMUpgrader"`
	OpcmInteropMigrator              common.Address `json:"OPCMInteropMigrator"`
	OpcmStandardValidator            common.Address `json:"OPCMStandardValidator"`
	DelayedWETHImpl                  common.Address `json:"DelayedWETHImpl"`
	OptimismPortalImpl               common.Address `json:"OptimismPortalImpl"`
	ETHLockboxImpl                   common.Address `json:"ETHLockboxImpl"`
	PreimageOracleSingleton          common.Address `json:"PreimageOracleSingleton"`
	MipsSingleton                    common.Address `json:"MipsSingleton"`
	SystemConfigImpl                 common.Address `json:"SystemConfigImpl"`
	L1CrossDomainMessengerImpl       common.Address `json:"L1CrossDomainMessengerImpl"`
	L1ERC721BridgeImpl               common.Address `json:"L1ERC721BridgeImpl"`
	L1StandardBridgeImpl             common.Address `json:"L1StandardBridgeImpl"`
	OptimismMintableERC20FactoryImpl common.Address `json:"OptimismMintableERC20FactoryImpl"`
	DisputeGameFactoryImpl           common.Address `json:"DisputeGameFactoryImpl"`
	AnchorStateRegistryImpl          common.Address `json:"AnchorStateRegistryImpl"`
	SuperchainConfigImpl             common.Address `json:"SuperchainConfigImpl"`
	ProtocolVersionsImpl             common.Address `json:"ProtocolVersionsImpl"`
}

type SuperchainDeployment struct {
	Implementations

	ProxyAdmin common.Address `json:"ProxyAdmin"`

	ProtocolVersions      common.Address `json:"ProtocolVersions"`
	ProtocolVersionsProxy common.Address `json:"ProtocolVersionsProxy"`

	SuperchainConfig      common.Address `json:"SuperchainConfig"`
	SuperchainConfigProxy common.Address `json:"SuperchainConfigProxy"`
}

type L2OpchainDeployment struct {
	OpChainProxyAdmin                 common.Address `json:"OpChainProxyAdmin"`
	AddressManager                    common.Address `json:"AddressManager"`
	L1ERC721BridgeProxy               common.Address `json:"L1ERC721BridgeProxy"`
	SystemConfigProxy                 common.Address `json:"SystemConfigProxy"`
	OptimismMintableERC20FactoryProxy common.Address `json:"OptimismMintableERC20FactoryProxy"`
	L1StandardBridgeProxy             common.Address `json:"L1StandardBridgeProxy"`
	L1CrossDomainMessengerProxy       common.Address `json:"L1CrossDomainMessengerProxy"`
	// Fault proof contracts below.
	OptimismPortalProxy                common.Address `json:"OptimismPortalProxy"`
	ETHLockboxProxy                    common.Address `json:"ETHLockboxProxy"`
	DisputeGameFactoryProxy            common.Address `json:"DisputeGameFactoryProxy"`
	AnchorStateRegistryProxy           common.Address `json:"AnchorStateRegistryProxy"`
	FaultDisputeGame                   common.Address `json:"FaultDisputeGame"`
	PermissionedDisputeGame            common.Address `json:"PermissionedDisputeGame"`
	DelayedWETHPermissionedGameProxy   common.Address `json:"DelayedWETHPermissionedGameProxy"`
	DelayedWETHPermissionlessGameProxy common.Address `json:"DelayedWETHPermissionlessGameProxy"`
}

type L2Deployment struct {
	L2OpchainDeployment

	// In the future this may contain optional extras,
	// e.g. a Safe that will own the L2 chain contracts
}

type InteropDeployment struct {
	DisputeGameFactory common.Address `json:"DisputeGameFactory"`
}

type WorldDeployment struct {
	L1         *L1Deployment            `json:"L1"`
	Superchain *SuperchainDeployment    `json:"Superchain"`
	L2s        map[string]*L2Deployment `json:"L2s"`
	Interop    *InteropDeployment       `json:"Interop"`
}
