// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { CommonTest } from "test/setup/CommonTest.sol";

// Scripts
import { ForgeArtifacts } from "scripts/libraries/ForgeArtifacts.sol";
import { Process } from "scripts/libraries/Process.sol";

// Libraries
import { LibString } from "@solady/utils/LibString.sol";
import { GameType, Hash, Proposal } from "src/dispute/lib/Types.sol";
import { EIP1967Helper } from "test/mocks/EIP1967Helper.sol";

// Interfaces
import { ISystemConfig } from "interfaces/L1/ISystemConfig.sol";
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";
import { IResourceMetering } from "interfaces/L1/IResourceMetering.sol";
import { IDisputeGameFactory } from "interfaces/dispute/IDisputeGameFactory.sol";
import { ProtocolVersion } from "interfaces/L1/IProtocolVersions.sol";
import { IOptimismPortal2 } from "interfaces/L1/IOptimismPortal2.sol";

/// @title Initializer_Test
/// @dev Ensures that the `initialize()` function on contracts cannot be called more than
///      once. This contract inherits from `ERC721Bridge_Initializer` because it is the
///      deepest contract in the inheritance chain for setting up the system contracts.
///      For each L1 contract both the implementation and the proxy are tested.
contract Initializer_Test is CommonTest {
    /// @notice Contains the address of an `Initializable` contract and the calldata
    ///         used to initialize it.
    struct InitializeableContract {
        string name;
        address target;
        bytes initCalldata;
    }

    /// @notice Array of contracts to test.
    InitializeableContract[] contracts;

    /// @notice Mapping of nickname to actual contract name.
    /// @dev Nicknames are only used when one proxy contract has multiple potential implementations
    ///      as can happen when a new implementation is being developed.
    mapping(string => string) nicknames;

    function setUp() public override {
        super.enableAltDA();
        super.setUp();

        // Initialize the `contracts` array with the addresses of the contracts to test, the
        // calldata used to initialize them, and the storage slot of their `_initialized` flag.
        // This array should contain all initializable L1 contracts. L2 contract initialization is
        // tested in Predeploys.t.sol.
        // The 'name' field should be the name of the contract as it saved in the deployment
        // script.

        // SuperchainConfigImpl
        contracts.push(
            InitializeableContract({
                name: "SuperchainConfigImpl",
                target: EIP1967Helper.getImplementation(address(superchainConfig)),
                initCalldata: abi.encodeCall(superchainConfig.initialize, (address(0)))
            })
        );
        // SuperchainConfigProxy
        contracts.push(
            InitializeableContract({
                name: "SuperchainConfigProxy",
                target: address(superchainConfig),
                initCalldata: abi.encodeCall(superchainConfig.initialize, (address(0)))
            })
        );
        // L1CrossDomainMessengerImpl
        contracts.push(
            InitializeableContract({
                name: "L1CrossDomainMessengerImpl",
                target: addressManager.getAddress("OVM_L1CrossDomainMessenger"),
                initCalldata: abi.encodeCall(l1CrossDomainMessenger.initialize, (systemConfig, optimismPortal2))
            })
        );
        // L1CrossDomainMessengerProxy
        contracts.push(
            InitializeableContract({
                name: "L1CrossDomainMessengerProxy",
                target: address(l1CrossDomainMessenger),
                initCalldata: abi.encodeCall(l1CrossDomainMessenger.initialize, (systemConfig, optimismPortal2))
            })
        );
        // DisputeGameFactoryImpl
        contracts.push(
            InitializeableContract({
                name: "DisputeGameFactoryImpl",
                target: EIP1967Helper.getImplementation(address(disputeGameFactory)),
                initCalldata: abi.encodeCall(disputeGameFactory.initialize, (address(0)))
            })
        );
        // DisputeGameFactoryProxy
        contracts.push(
            InitializeableContract({
                name: "DisputeGameFactoryProxy",
                target: address(disputeGameFactory),
                initCalldata: abi.encodeCall(disputeGameFactory.initialize, (address(0)))
            })
        );
        // DelayedWETHImpl
        contracts.push(
            InitializeableContract({
                name: "DelayedWETHImpl",
                target: EIP1967Helper.getImplementation(address(delayedWeth)),
                initCalldata: abi.encodeCall(delayedWeth.initialize, (ISystemConfig(address(0))))
            })
        );
        // DelayedWETHProxy
        contracts.push(
            InitializeableContract({
                name: "DelayedWETHProxy",
                target: address(delayedWeth),
                initCalldata: abi.encodeCall(delayedWeth.initialize, (ISystemConfig(address(0))))
            })
        );
        // OptimismPortal2Impl
        contracts.push(
            InitializeableContract({
                name: "OptimismPortal2Impl",
                target: EIP1967Helper.getImplementation(address(optimismPortal2)),
                initCalldata: abi.encodeCall(optimismPortal2.initialize, (systemConfig, anchorStateRegistry, ethLockbox))
            })
        );
        // OptimismPortal2Proxy
        contracts.push(
            InitializeableContract({
                name: "OptimismPortal2Proxy",
                target: address(optimismPortal2),
                initCalldata: abi.encodeCall(optimismPortal2.initialize, (systemConfig, anchorStateRegistry, ethLockbox))
            })
        );

        // SystemConfigImpl
        contracts.push(
            InitializeableContract({
                name: "SystemConfigImpl",
                target: EIP1967Helper.getImplementation(address(systemConfig)),
                initCalldata: abi.encodeCall(
                    systemConfig.initialize,
                    (
                        address(0xdead),
                        0,
                        0,
                        bytes32(0),
                        1,
                        address(0),
                        IResourceMetering.ResourceConfig({
                            maxResourceLimit: 1,
                            elasticityMultiplier: 1,
                            baseFeeMaxChangeDenominator: 2,
                            minimumBaseFee: 0,
                            systemTxMaxGas: 0,
                            maximumBaseFee: 0
                        }),
                        address(0),
                        ISystemConfig.Addresses({
                            l1CrossDomainMessenger: address(0),
                            l1ERC721Bridge: address(0),
                            l1StandardBridge: address(0),
                            optimismPortal: address(0),
                            optimismMintableERC20Factory: address(0)
                        }),
                        0,
                        ISuperchainConfig(address(0))
                    )
                )
            })
        );
        // SystemConfigProxy
        contracts.push(
            InitializeableContract({
                name: "SystemConfigProxy",
                target: address(systemConfig),
                initCalldata: abi.encodeCall(
                    systemConfig.initialize,
                    (
                        address(0xdead),
                        0,
                        0,
                        bytes32(0),
                        1,
                        address(0),
                        IResourceMetering.ResourceConfig({
                            maxResourceLimit: 1,
                            elasticityMultiplier: 1,
                            baseFeeMaxChangeDenominator: 2,
                            minimumBaseFee: 0,
                            systemTxMaxGas: 0,
                            maximumBaseFee: 0
                        }),
                        address(0),
                        ISystemConfig.Addresses({
                            l1CrossDomainMessenger: address(0),
                            l1ERC721Bridge: address(0),
                            l1StandardBridge: address(0),
                            optimismPortal: address(0),
                            optimismMintableERC20Factory: address(0)
                        }),
                        0,
                        ISuperchainConfig(address(0))
                    )
                )
            })
        );
        // ProtocolVersionsImpl
        contracts.push(
            InitializeableContract({
                name: "ProtocolVersionsImpl",
                target: EIP1967Helper.getImplementation(address(protocolVersions)),
                initCalldata: abi.encodeCall(
                    protocolVersions.initialize, (address(0), ProtocolVersion.wrap(1), ProtocolVersion.wrap(2))
                )
            })
        );
        // ProtocolVersionsProxy
        contracts.push(
            InitializeableContract({
                name: "ProtocolVersionsProxy",
                target: address(protocolVersions),
                initCalldata: abi.encodeCall(
                    protocolVersions.initialize, (address(0), ProtocolVersion.wrap(1), ProtocolVersion.wrap(2))
                )
            })
        );
        // L1StandardBridgeImpl
        contracts.push(
            InitializeableContract({
                name: "L1StandardBridgeImpl",
                target: EIP1967Helper.getImplementation(address(l1StandardBridge)),
                initCalldata: abi.encodeCall(l1StandardBridge.initialize, (l1CrossDomainMessenger, systemConfig))
            })
        );
        // L1StandardBridgeProxy
        contracts.push(
            InitializeableContract({
                name: "L1StandardBridgeProxy",
                target: address(l1StandardBridge),
                initCalldata: abi.encodeCall(l1StandardBridge.initialize, (l1CrossDomainMessenger, systemConfig))
            })
        );
        // L1ERC721BridgeImpl
        contracts.push(
            InitializeableContract({
                name: "L1ERC721BridgeImpl",
                target: EIP1967Helper.getImplementation(address(l1ERC721Bridge)),
                initCalldata: abi.encodeCall(l1ERC721Bridge.initialize, (l1CrossDomainMessenger, systemConfig))
            })
        );
        // L1ERC721BridgeProxy
        contracts.push(
            InitializeableContract({
                name: "L1ERC721BridgeProxy",
                target: address(l1ERC721Bridge),
                initCalldata: abi.encodeCall(l1ERC721Bridge.initialize, (l1CrossDomainMessenger, systemConfig))
            })
        );
        // OptimismMintableERC20FactoryImpl
        contracts.push(
            InitializeableContract({
                name: "OptimismMintableERC20FactoryImpl",
                target: EIP1967Helper.getImplementation(address(l1OptimismMintableERC20Factory)),
                initCalldata: abi.encodeCall(l1OptimismMintableERC20Factory.initialize, (address(l1StandardBridge)))
            })
        );
        // OptimismMintableERC20FactoryProxy
        contracts.push(
            InitializeableContract({
                name: "OptimismMintableERC20FactoryProxy",
                target: address(l1OptimismMintableERC20Factory),
                initCalldata: abi.encodeCall(l1OptimismMintableERC20Factory.initialize, (address(l1StandardBridge)))
            })
        );
        // DataAvailabilityChallengeImpl
        contracts.push(
            InitializeableContract({
                name: "DataAvailabilityChallengeImpl",
                target: EIP1967Helper.getImplementation(address(dataAvailabilityChallenge)),
                initCalldata: abi.encodeCall(dataAvailabilityChallenge.initialize, (address(0), 0, 0, 0, 0))
            })
        );
        // DataAvailabilityChallengeProxy
        contracts.push(
            InitializeableContract({
                name: "DataAvailabilityChallengeProxy",
                target: address(dataAvailabilityChallenge),
                initCalldata: abi.encodeCall(dataAvailabilityChallenge.initialize, (address(0), 0, 0, 0, 0))
            })
        );
        // AnchorStateRegistry
        contracts.push(
            InitializeableContract({
                name: "AnchorStateRegistryImpl",
                target: EIP1967Helper.getImplementation(address(anchorStateRegistry)),
                initCalldata: abi.encodeCall(
                    anchorStateRegistry.initialize,
                    (
                        ISystemConfig(address(0)),
                        IDisputeGameFactory(address(0)),
                        Proposal({ root: Hash.wrap(bytes32(0)), l2SequenceNumber: 0 }),
                        GameType.wrap(uint32(deploy.cfg().respectedGameType()))
                    )
                )
            })
        );
        // AnchorStateRegistryProxy
        contracts.push(
            InitializeableContract({
                name: "AnchorStateRegistryProxy",
                target: address(anchorStateRegistry),
                initCalldata: abi.encodeCall(
                    anchorStateRegistry.initialize,
                    (
                        ISystemConfig(address(0)),
                        IDisputeGameFactory(address(0)),
                        Proposal({ root: Hash.wrap(bytes32(0)), l2SequenceNumber: 0 }),
                        GameType.wrap(uint32(deploy.cfg().respectedGameType()))
                    )
                )
            })
        );

        // ETHLockboxImpl
        contracts.push(
            InitializeableContract({
                name: "ETHLockboxImpl",
                target: EIP1967Helper.getImplementation(address(ethLockbox)),
                initCalldata: abi.encodeCall(ethLockbox.initialize, (ISystemConfig(address(0)), new IOptimismPortal2[](0)))
            })
        );

        // ETHLockboxProxy
        contracts.push(
            InitializeableContract({
                name: "ETHLockboxProxy",
                target: address(ethLockbox),
                initCalldata: abi.encodeCall(ethLockbox.initialize, (ISystemConfig(address(0)), new IOptimismPortal2[](0)))
            })
        );
    }

    /// @notice Tests that:
    ///         1. All `Initializable` contracts in `src/` (except periphery) are accounted for in `contracts`.
    ///         2. The `_initialized` flag of each contract is properly set.
    ///         3. The `initialize()` function of each contract cannot be called again.
    function test_cannotReinitialize_succeeds() public {
        // Collect exclusions.
        uint256 j;
        string[] memory excludes = new string[](8);
        // Contract is currently not being deployed as part of the standard deployment script.
        excludes[j++] = "src/L2/OptimismSuperchainERC20.sol";
        // Periphery contracts don't get deployed as part of the standard deployment script.
        excludes[j++] = "src/periphery/*";
        // TODO: Deployment script is currently "broken" in the sense that it doesn't properly
        //       label the FaultDisputeGame, PermissionedDisputeGame, SuperFaultDisputeGame, and
        // SuperPermissionedDisputeGame
        //       contracts and instead simply deploys them anonymously. Means that functions like "getInitializedSlot"
        //       don't work properly. Remove these exclusions once the deployment script is fixed.
        excludes[j++] = "src/dispute/FaultDisputeGame.sol";
        excludes[j++] = "src/dispute/SuperFaultDisputeGame.sol";
        excludes[j++] = "src/dispute/PermissionedDisputeGame.sol";
        excludes[j++] = "src/dispute/SuperPermissionedDisputeGame.sol";
        // TODO: Eventually remove this exclusion. Same reason as above dispute contracts.
        excludes[j++] = "src/L1/OPContractsManager.sol";
        // L2 contract initialization is tested in Predeploys.t.sol
        excludes[j++] = "src/L2/*";

        // Get all contract names in the src directory, minus the excluded contracts.
        string[] memory contractNames = ForgeArtifacts.getContractNames("src/*", excludes);

        // Iterate over all contracts to assert that they are accounted for in the `contracts
        // array. All contracts that have an `initialize()` function must be accounted for in the
        // `contracts` array or an error will be thrown. If the contract is proxied, both the
        // implementation and the proxy must be accounted for in the `contracts` array.
        for (uint256 i; i < contractNames.length; i++) {
            string memory contractName = contractNames[i];
            string memory contractKind = ForgeArtifacts.getContractKind(contractName);

            // Filter out non-contracts.
            if (!LibString.eq(contractKind, "contract")) {
                continue;
            }

            // Construct the query for the initialize function in the contract's ABI.
            string memory cmd = string.concat(
                "echo '",
                ForgeArtifacts.getAbi(contractName),
                "' | jq '.[] | select(.name == \"initialize\" and .type == \"function\")'"
            );

            // If the contract does not have an `initialize()` function, skip it.
            if (bytes(Process.bash(cmd)).length == 0) {
                continue;
            }

            // Check if this contract is in the contracts array.
            assertTrue(
                _hasMatchingContract(string.concat(contractName, "Impl")),
                string.concat("Missing ", contractName, " from contracts array")
            );

            // If the contract is proxied, check that the proxy is in the contracts array.
            // Skip predeployed contracts for now since we don't yet keep track of the
            // implementations inside of the deploy script.
            // TODO: We should add support for this in the future so that we can properly check that
            //       the implementations for predeployed contracts are initialized too.
            if (ForgeArtifacts.isProxiedContract(contractName) && !ForgeArtifacts.isPredeployedContract(contractName)) {
                assertTrue(
                    _hasMatchingContract(string.concat(contractName, "Proxy")),
                    string.concat("Missing ", contractName, "Proxy from contracts array")
                );
            }
        }

        // Attempt to re-initialize all contracts within the `contracts` array.
        for (uint256 i; i < contracts.length; i++) {
            InitializeableContract memory _contract = contracts[i];
            string memory deploymentName = _getRealContractName(_contract.name);

            // Assert that the contract is already initialized.
            assertTrue(
                ForgeArtifacts.isInitialized({ _name: _removeSuffix(deploymentName), _address: _contract.target }),
                "Initializable: contract is not initialized"
            );

            // Then, attempt to re-initialize the contract. This should fail.
            (bool success, bytes memory returnData) = _contract.target.call(_contract.initCalldata);
            assertFalse(success);
            assertEq(_extractErrorString(returnData), "Initializable: contract is already initialized");
        }
    }

    /// @dev Returns true if the contract with the given name is in the `contracts` array.
    /// @param _name The name of the contract to check.
    /// @return matching_ True if the contract is in the `contracts` array, false otherwise.
    function _hasMatchingContract(string memory _name) internal view returns (bool matching_) {
        for (uint256 i; i < contracts.length; i++) {
            if (LibString.eq(contracts[i].name, _getRealContractName(_name))) {
                // return early
                return true;
            }
        }
    }

    /// @dev Returns the real name of the contract, including any nicknames.
    /// @param _name The name of the contract.
    /// @return real_ The real name of the contract.
    function _getRealContractName(string memory _name) internal view returns (string memory real_) {
        real_ = bytes(nicknames[_name]).length > 0 ? nicknames[_name] : _name;
    }

    /// @dev Extracts the revert string from returndata encoded in the form of `Error(string)`.
    function _extractErrorString(bytes memory _returnData) internal pure returns (string memory error_) {
        // The first 4 bytes of the return data should be the selector for `Error(string)`. If not, revert.
        if (bytes4(_returnData) == 0x08c379a0) {
            // Extract the error string from the returndata. The error string is located 68 bytes after
            // the pointer to `returnData`.
            //
            // 32 bytes: `returnData` length
            // 4 bytes: `Error(string)` selector
            // 32 bytes: ABI encoding metadata; String offset
            // = 68 bytes
            assembly {
                error_ := add(_returnData, 0x44)
            }
        } else {
            revert("Initializer_Test: Invalid returndata format. Expected `Error(string)`");
        }
    }

    /// @dev Removes the `Proxy` or `Impl` suffix from the contract name.
    function _removeSuffix(string memory _contractName) internal pure returns (string memory deploymentName_) {
        if (LibString.endsWith(_contractName, "Proxy")) {
            deploymentName_ = LibString.slice(_contractName, 0, bytes(_contractName).length - 5);
        }
        if (LibString.endsWith(_contractName, "Impl")) {
            deploymentName_ = LibString.slice(_contractName, 0, bytes(_contractName).length - 4);
        }
    }
}
