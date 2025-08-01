// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { stdStorage, StdStorage } from "forge-std/Test.sol";
import { EIP1967Helper } from "test/mocks/EIP1967Helper.sol";
import { CommonTest } from "test/setup/CommonTest.sol";

// Contracts
import { ERC20 } from "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import { OptimismMintableERC20 } from "src/universal/OptimismMintableERC20.sol";

// Libraries
import { Predeploys } from "src/libraries/Predeploys.sol";
import { Hashing } from "src/libraries/Hashing.sol";
import { Types } from "src/libraries/Types.sol";

// Interfaces
import { ICrossDomainMessenger } from "interfaces/universal/ICrossDomainMessenger.sol";
import { IStandardBridge } from "interfaces/universal/IStandardBridge.sol";
import { IL2ToL1MessagePasser } from "interfaces/L2/IL2ToL1MessagePasser.sol";
import { IL2StandardBridge } from "interfaces/L2/IL2StandardBridge.sol";

/// @title L2StandardBridge_TestInit
/// @notice Reusable test initialization for `L2StandardBridge` tests.
contract L2StandardBridge_TestInit is CommonTest {
    /// @notice Sets up expected calls and emits for a successful ERC20 withdrawal.
    function _preBridgeERC20(bool _isLegacy, address _l2Token) internal {
        // Alice has 100 L2Token
        deal(_l2Token, alice, 100, true);
        assertEq(ERC20(_l2Token).balanceOf(alice), 100);
        uint256 nonce = l2CrossDomainMessenger.messageNonce();
        bytes memory message =
            abi.encodeCall(IStandardBridge.finalizeBridgeERC20, (address(L1Token), _l2Token, alice, alice, 100, hex""));
        uint64 baseGas = l2CrossDomainMessenger.baseGas(message, 1000);
        bytes memory withdrawalData = abi.encodeCall(
            ICrossDomainMessenger.relayMessage,
            (nonce, address(l2StandardBridge), address(l1StandardBridge), 0, 1000, message)
        );
        bytes32 withdrawalHash = Hashing.hashWithdrawal(
            Types.WithdrawalTransaction({
                nonce: nonce,
                sender: address(l2CrossDomainMessenger),
                target: address(l1CrossDomainMessenger),
                value: 0,
                gasLimit: baseGas,
                data: withdrawalData
            })
        );

        if (_isLegacy) {
            vm.expectCall(
                address(l2StandardBridge), abi.encodeCall(l2StandardBridge.withdraw, (_l2Token, 100, 1000, hex""))
            );
        } else {
            vm.expectCall(
                address(l2StandardBridge),
                abi.encodeCall(l2StandardBridge.bridgeERC20, (_l2Token, address(L1Token), 100, 1000, hex""))
            );
        }

        vm.expectCall(
            address(l2CrossDomainMessenger),
            abi.encodeCall(ICrossDomainMessenger.sendMessage, (address(l1StandardBridge), message, 1000))
        );

        vm.expectCall(
            Predeploys.L2_TO_L1_MESSAGE_PASSER,
            abi.encodeCall(
                IL2ToL1MessagePasser.initiateWithdrawal, (address(l1CrossDomainMessenger), baseGas, withdrawalData)
            )
        );

        // The l2StandardBridge should burn the tokens
        vm.expectCall(_l2Token, abi.encodeCall(OptimismMintableERC20.burn, (alice, 100)));

        vm.expectEmit(true, true, true, true);
        emit WithdrawalInitiated(address(L1Token), _l2Token, alice, alice, 100, hex"");

        vm.expectEmit(true, true, true, true);
        emit ERC20BridgeInitiated(_l2Token, address(L1Token), alice, alice, 100, hex"");

        vm.expectEmit(true, true, true, true);
        emit MessagePassed(
            nonce,
            address(l2CrossDomainMessenger),
            address(l1CrossDomainMessenger),
            0,
            baseGas,
            withdrawalData,
            withdrawalHash
        );

        // SentMessage event emitted by the CrossDomainMessenger
        vm.expectEmit(true, true, true, true);
        emit SentMessage(address(l1StandardBridge), address(l2StandardBridge), message, nonce, 1000);

        // SentMessageExtension1 event emitted by the CrossDomainMessenger
        vm.expectEmit(true, true, true, true);
        emit SentMessageExtension1(address(l2StandardBridge), 0);

        vm.prank(alice, alice);
    }

    /// @notice Sets up expected calls and emits for a successful ERC20 withdrawal to a different
    ///         recipient.
    /// @dev `withdrawTo` and `bridgeERC20To` should behave the same when transferring ERC20 tokens
    ///      so they should share the same setup and expectEmit calls
    function _preBridgeERC20To(bool _isLegacy, address _l2Token) internal {
        deal(_l2Token, alice, 100, true);
        assertEq(L2Token.balanceOf(alice), 100);
        uint256 nonce = l2CrossDomainMessenger.messageNonce();
        bytes memory message =
            abi.encodeCall(IStandardBridge.finalizeBridgeERC20, (address(L1Token), _l2Token, alice, bob, 100, hex""));
        uint64 baseGas = l2CrossDomainMessenger.baseGas(message, 1000);
        bytes memory withdrawalData = abi.encodeCall(
            ICrossDomainMessenger.relayMessage,
            (nonce, address(l2StandardBridge), address(l1StandardBridge), 0, 1000, message)
        );
        bytes32 withdrawalHash = Hashing.hashWithdrawal(
            Types.WithdrawalTransaction({
                nonce: nonce,
                sender: address(l2CrossDomainMessenger),
                target: address(l1CrossDomainMessenger),
                value: 0,
                gasLimit: baseGas,
                data: withdrawalData
            })
        );

        vm.expectEmit(address(l2StandardBridge));
        emit WithdrawalInitiated(address(L1Token), _l2Token, alice, bob, 100, hex"");

        vm.expectEmit(address(l2StandardBridge));
        emit ERC20BridgeInitiated(_l2Token, address(L1Token), alice, bob, 100, hex"");

        vm.expectEmit(address(l2ToL1MessagePasser));
        emit MessagePassed(
            nonce,
            address(l2CrossDomainMessenger),
            address(l1CrossDomainMessenger),
            0,
            baseGas,
            withdrawalData,
            withdrawalHash
        );

        // SentMessage event emitted by the CrossDomainMessenger
        vm.expectEmit(address(l2CrossDomainMessenger));
        emit SentMessage(address(l1StandardBridge), address(l2StandardBridge), message, nonce, 1000);

        // SentMessageExtension1 event emitted by the CrossDomainMessenger
        vm.expectEmit(address(l2CrossDomainMessenger));
        emit SentMessageExtension1(address(l2StandardBridge), 0);

        if (_isLegacy) {
            vm.expectCall(
                address(l2StandardBridge),
                abi.encodeCall(l2StandardBridge.withdrawTo, (_l2Token, bob, 100, 1000, hex""))
            );
        } else {
            vm.expectCall(
                address(l2StandardBridge),
                abi.encodeCall(l2StandardBridge.bridgeERC20To, (_l2Token, address(L1Token), bob, 100, 1000, hex""))
            );
        }

        vm.expectCall(
            address(l2CrossDomainMessenger),
            abi.encodeCall(ICrossDomainMessenger.sendMessage, (address(l1StandardBridge), message, 1000))
        );

        vm.expectCall(
            Predeploys.L2_TO_L1_MESSAGE_PASSER,
            abi.encodeCall(
                IL2ToL1MessagePasser.initiateWithdrawal, (address(l1CrossDomainMessenger), baseGas, withdrawalData)
            )
        );

        // The l2StandardBridge should burn the tokens
        vm.expectCall(address(L2Token), abi.encodeCall(OptimismMintableERC20.burn, (alice, 100)));

        vm.prank(alice, alice);
    }

    using stdStorage for StdStorage;
}

/// @title L2StandardBridge_Version_Test
/// @notice Tests the `version` function of the `L2StandardBridge` contract.
contract L2StandardBridge_Version_Test is L2StandardBridge_TestInit {
    /// @notice Tests that the version function returns a valid string. We avoid testing the
    ///         specific value of the string as it changes frequently.
    function test_version_succeeds() external view {
        assert(bytes(l2StandardBridge.version()).length > 0);
    }
}

/// @title L2StandardBridge_Constructor_Test
/// @notice Tests the constructor of the `L2StandardBridge` contract.
contract L2StandardBridge_Constructor_Test is L2StandardBridge_TestInit {
    /// @notice Test that the bridge's `constructor` sets the correct values.
    function test_constructor_succeeds() external view {
        IL2StandardBridge impl =
            IL2StandardBridge(payable(EIP1967Helper.getImplementation(artifacts.mustGetAddress("L2StandardBridge"))));
        // The implementation contract is initialized with a 0 L1 bridge address,
        // but the L2 cross-domain-messenger is always set to the predeploy address for both proxy and implementation.
        assertEq(address(impl.MESSENGER()), Predeploys.L2_CROSS_DOMAIN_MESSENGER, "constructor zero check MESSENGER");
        assertEq(address(impl.messenger()), Predeploys.L2_CROSS_DOMAIN_MESSENGER, "constructor zero check messenger");
        assertEq(address(impl.OTHER_BRIDGE()), address(0), "constructor zero check OTHER_BRIDGE");
        assertEq(address(impl.otherBridge()), address(0), "constructor zero check otherBridge");
        assertEq(address(impl.l1TokenBridge()), address(0), "constructor zero check l1TokenBridge");
    }
}

/// @title L2StandardBridge_Initialize_Test
/// @notice Tests the `initialize` function of the `L2StandardBridge` contract.
contract L2StandardBridge_Initialize_Test is L2StandardBridge_TestInit {
    /// @notice Tests that the bridge is initialized correctly.
    function test_initialize_succeeds() external view {
        assertEq(address(l2StandardBridge.MESSENGER()), address(l2CrossDomainMessenger));
        assertEq(address(l2StandardBridge.messenger()), address(l2CrossDomainMessenger));
        assertEq(l1StandardBridge.l2TokenBridge(), address(l2StandardBridge));
        assertEq(address(l2StandardBridge.OTHER_BRIDGE()), address(l1StandardBridge));
        assertEq(address(l2StandardBridge.otherBridge()), address(l1StandardBridge));
    }
}

/// @title L2StandardBridge_Receive_Test
/// @notice Tests the `receive` function of the `L2StandardBridge` contract.
contract L2StandardBridge_Receive_Test is L2StandardBridge_TestInit {
    /// @notice Tests that the bridge receives ETH and successfully initiates a withdrawal.
    function test_receive_succeeds() external {
        assertEq(address(l2ToL1MessagePasser).balance, 0);
        uint256 nonce = l2CrossDomainMessenger.messageNonce();

        bytes memory message = abi.encodeCall(IStandardBridge.finalizeBridgeETH, (alice, alice, 100, hex""));
        uint64 baseGas = l2CrossDomainMessenger.baseGas(message, 200_000);
        bytes memory withdrawalData = abi.encodeCall(
            ICrossDomainMessenger.relayMessage,
            (nonce, address(l2StandardBridge), address(l1StandardBridge), 100, 200_000, message)
        );
        bytes32 withdrawalHash = Hashing.hashWithdrawal(
            Types.WithdrawalTransaction({
                nonce: nonce,
                sender: address(l2CrossDomainMessenger),
                target: address(l1CrossDomainMessenger),
                value: 100,
                gasLimit: baseGas,
                data: withdrawalData
            })
        );

        vm.expectEmit(true, true, true, true);
        emit WithdrawalInitiated(address(0), Predeploys.LEGACY_ERC20_ETH, alice, alice, 100, hex"");

        vm.expectEmit(true, true, true, true);
        emit ETHBridgeInitiated(alice, alice, 100, hex"");

        // L2ToL1MessagePasser will emit a MessagePassed event
        vm.expectEmit(address(l2ToL1MessagePasser));
        emit MessagePassed(
            nonce,
            address(l2CrossDomainMessenger),
            address(l1CrossDomainMessenger),
            100,
            baseGas,
            withdrawalData,
            withdrawalHash
        );

        // SentMessage event emitted by the CrossDomainMessenger
        vm.expectEmit(address(l2CrossDomainMessenger));
        emit SentMessage(address(l1StandardBridge), address(l2StandardBridge), message, nonce, 200_000);

        // SentMessageExtension1 event emitted by the CrossDomainMessenger
        vm.expectEmit(address(l2CrossDomainMessenger));
        emit SentMessageExtension1(address(l2StandardBridge), 100);

        vm.expectCall(
            address(l2CrossDomainMessenger),
            abi.encodeCall(
                ICrossDomainMessenger.sendMessage,
                (address(l1StandardBridge), message, 200_000) // StandardBridge's RECEIVE_DEFAULT_GAS_LIMIT
            )
        );

        vm.expectCall(
            Predeploys.L2_TO_L1_MESSAGE_PASSER,
            abi.encodeCall(
                IL2ToL1MessagePasser.initiateWithdrawal, (address(l1CrossDomainMessenger), baseGas, withdrawalData)
            )
        );

        vm.prank(alice, alice);
        (bool success,) = address(l2StandardBridge).call{ value: 100 }(hex"");
        assertEq(success, true);
        assertEq(address(l2ToL1MessagePasser).balance, 100);
    }
}

/// @title L2StandardBridge_Withdraw_Test
/// @notice Tests the `withdraw` function of the `L2StandardBridge` contract.
contract L2StandardBridge_Withdraw_Test is L2StandardBridge_TestInit {
    /// @notice Tests that `withdraw` reverts if the amount is not equal to the value sent.
    function test_withdraw_insufficientValue_reverts() external {
        assertEq(address(l2ToL1MessagePasser).balance, 0);

        vm.expectRevert("StandardBridge: bridging ETH must include sufficient ETH value");
        vm.prank(alice, alice);
        l2StandardBridge.withdraw{ value: 1 }(address(Predeploys.LEGACY_ERC20_ETH), 100, 1, hex"");
    }

    /// @notice Tests that `withdraw` reverts when sending value and attempting to withdraw an
    ///         ERC20 token. This prevents ether from being stuck in the contract.
    function test_withdraw_erc20WithValue_reverts() external {
        vm.deal(alice, 100);
        vm.expectRevert("StandardBridge: cannot send value");
        vm.prank(alice, alice);
        l2StandardBridge.withdraw{ value: 100 }(address(L2Token), 100, 1, hex"");
    }

    /// @notice Tests that the legacy `withdraw` interface on the L2StandardBridge sucessfully
    ///         initiates a withdrawal.
    function test_withdraw_ether_succeeds() external {
        assertTrue(alice.balance >= 100);
        assertEq(Predeploys.L2_TO_L1_MESSAGE_PASSER.balance, 0);

        vm.expectEmit(address(l2StandardBridge));
        emit WithdrawalInitiated({
            l1Token: address(0),
            l2Token: Predeploys.LEGACY_ERC20_ETH,
            from: alice,
            to: alice,
            amount: 100,
            data: hex""
        });

        vm.expectEmit(address(l2StandardBridge));
        emit ETHBridgeInitiated({ from: alice, to: alice, amount: 100, data: hex"" });

        vm.prank(alice, alice);
        l2StandardBridge.withdraw{ value: 100 }({
            _l2Token: Predeploys.LEGACY_ERC20_ETH,
            _amount: 100,
            _minGasLimit: 1000,
            _extraData: hex""
        });

        assertEq(Predeploys.L2_TO_L1_MESSAGE_PASSER.balance, 100);
    }

    /// @notice Tests that `withdraw` burns the tokens, emits `WithdrawalInitiated`, and initiates
    ///         a withdrawal with `Withdrawer.initiateWithdrawal`.
    function test_withdraw_withdrawingERC20_succeeds() external {
        _preBridgeERC20({ _isLegacy: true, _l2Token: address(L2Token) });
        l2StandardBridge.withdraw(address(L2Token), 100, 1000, hex"");

        assertEq(L2Token.balanceOf(alice), 0);
    }

    function test_withdrawLegacyERC20_succeeds() external {
        _preBridgeERC20({ _isLegacy: true, _l2Token: address(LegacyL2Token) });
        l2StandardBridge.withdraw(address(LegacyL2Token), 100, 1000, hex"");

        assertEq(L2Token.balanceOf(alice), 0);
    }

    function test_withdraw_notEOA_reverts() external {
        // This contract has 100 L2Token
        deal(address(L2Token), address(this), 100, true);

        vm.expectRevert("StandardBridge: function can only be called from an EOA");
        l2StandardBridge.withdraw(address(L2Token), 100, 1000, hex"");
    }
}

/// @title L2StandardBridge_WithdrawTo_Test
/// @notice Tests the `withdrawTo` function of the `L2StandardBridge` contract.
contract L2StandardBridge_WithdrawTo_Test is L2StandardBridge_TestInit {
    /// @notice Tests that `withdrawTo` reverts when sending value and attempting to withdraw an
    ///         ERC20 token. This prevents ether from being stuck in the contract.
    function test_withdrawTo_erc20WithValue_reverts() external {
        vm.deal(alice, 100);
        vm.expectRevert("StandardBridge: cannot send value");
        vm.prank(alice, alice);
        l2StandardBridge.withdrawTo{ value: 100 }(address(L2Token), alice, 100, 1, hex"");
    }

    /// @notice Tests that `withdrawTo` burns the tokens, emits `WithdrawalInitiated`, and
    ///         initiates a withdrawal with `Withdrawer.initiateWithdrawal`.
    function test_withdrawTo_withdrawingERC20_succeeds() external {
        _preBridgeERC20To({ _isLegacy: true, _l2Token: address(L2Token) });
        l2StandardBridge.withdrawTo(address(L2Token), bob, 100, 1000, hex"");

        assertEq(L2Token.balanceOf(alice), 0);
    }
}

/// @title L2StandardBridge_Unclassified_Test
/// @notice General tests that are not testing any function directly of the `L2StandardBridge`
///         contract.
contract L2StandardBridge_Unclassified_Test is L2StandardBridge_TestInit {
    /// @notice Ensures that the L2StandardBridge is always not paused. The pausability happens
    ///         on L1 and not L2.
    function test_paused_succeeds() external view {
        assertFalse(l2StandardBridge.paused());
    }

    /// @notice Tests that `bridgeERC20` burns the tokens, emits `WithdrawalInitiated`, and
    ///         initiates a withdrawal with `Withdrawer.initiateWithdrawal`.
    function test_bridgeERC20_succeeds() external {
        _preBridgeERC20({ _isLegacy: false, _l2Token: address(L2Token) });
        l2StandardBridge.bridgeERC20(address(L2Token), address(L1Token), 100, 1000, hex"");

        assertEq(L2Token.balanceOf(alice), 0);
    }

    function test_bridgeERC20_isNotCorrectTokenPair_reverts() external {
        vm.expectRevert("StandardBridge: wrong remote token for Optimism Mintable ERC20 local token");
        vm.prank(alice, alice);
        l2StandardBridge.bridgeERC20(address(L2Token), address(BadL1Token), 100, 1000, hex"");
    }

    function test_bridgeLegacyERC20_succeeds() external {
        _preBridgeERC20({ _isLegacy: false, _l2Token: address(LegacyL2Token) });
        l2StandardBridge.bridgeERC20(address(LegacyL2Token), address(L1Token), 100, 1000, hex"");

        assertEq(L2Token.balanceOf(alice), 0);
    }

    /// @notice Tests that `bridgeERC20To` burns the tokens, emits `WithdrawalInitiated`, and
    ///         initiates a withdrawal with `Withdrawer.initiateWithdrawal`.
    function test_bridgeERC20To_succeeds() external {
        _preBridgeERC20To({ _isLegacy: false, _l2Token: address(L2Token) });
        l2StandardBridge.bridgeERC20To(address(L2Token), address(L1Token), bob, 100, 1000, hex"");
        assertEq(L2Token.balanceOf(alice), 0);
    }

    /// @notice Tests that `finalizeBridgeETH` reverts if the recipient is the other bridge.
    function test_finalizeBridgeETH_sendToSelf_reverts() external {
        vm.mockCall(
            address(l2StandardBridge.messenger()),
            abi.encodeCall(ICrossDomainMessenger.xDomainMessageSender, ()),
            abi.encode(address(l2StandardBridge.OTHER_BRIDGE()))
        );
        vm.deal(address(l2CrossDomainMessenger), 100);
        vm.prank(address(l2CrossDomainMessenger));
        vm.expectRevert("StandardBridge: cannot send to self");
        l2StandardBridge.finalizeBridgeETH{ value: 100 }(alice, address(l2StandardBridge), 100, hex"");
    }

    /// @notice Tests that `finalizeBridgeETH` reverts if the recipient is the messenger.
    function test_finalizeBridgeETH_sendToMessenger_reverts() external {
        vm.mockCall(
            address(l2StandardBridge.messenger()),
            abi.encodeCall(ICrossDomainMessenger.xDomainMessageSender, ()),
            abi.encode(address(l2StandardBridge.OTHER_BRIDGE()))
        );
        vm.deal(address(l2CrossDomainMessenger), 100);
        vm.prank(address(l2CrossDomainMessenger));
        vm.expectRevert("StandardBridge: cannot send to messenger");
        l2StandardBridge.finalizeBridgeETH{ value: 100 }(alice, address(l2CrossDomainMessenger), 100, hex"");
    }

    /// @notice Tests that bridging ETH succeeds.
    function testFuzz_bridgeETH_succeeds(uint256 _value, uint32 _minGasLimit, bytes calldata _extraData) external {
        uint256 nonce = l2CrossDomainMessenger.messageNonce();

        bytes memory message = abi.encodeCall(IStandardBridge.finalizeBridgeETH, (alice, alice, _value, _extraData));

        vm.expectCall(
            address(l2StandardBridge), _value, abi.encodeCall(l2StandardBridge.bridgeETH, (_minGasLimit, _extraData))
        );

        vm.expectCall(
            address(l2CrossDomainMessenger),
            _value,
            abi.encodeCall(ICrossDomainMessenger.sendMessage, (address(l1StandardBridge), message, _minGasLimit))
        );

        vm.expectEmit(address(l2StandardBridge));
        emit ETHBridgeInitiated(alice, alice, _value, _extraData);

        // SentMessage event emitted by the CrossDomainMessenger
        vm.expectEmit(address(l2CrossDomainMessenger));
        emit SentMessage(address(l1StandardBridge), address(l2StandardBridge), message, nonce, _minGasLimit);

        // SentMessageExtension1 event emitted by the CrossDomainMessenger
        vm.expectEmit(address(l2CrossDomainMessenger));
        emit SentMessageExtension1(address(l2StandardBridge), _value);

        vm.deal(alice, _value);
        vm.prank(alice, alice);

        l2StandardBridge.bridgeETH{ value: _value }(_minGasLimit, _extraData);
    }

    /// @notice Tests that bridging ETH to a different address succeeds.
    function testFuzz_bridgeETHTo_succeeds(uint256 _value, uint32 _minGasLimit, bytes calldata _extraData) external {
        uint256 nonce = l2CrossDomainMessenger.messageNonce();

        vm.expectCall(
            address(l2StandardBridge),
            _value,
            abi.encodeCall(l1StandardBridge.bridgeETHTo, (bob, _minGasLimit, _extraData))
        );

        bytes memory message = abi.encodeCall(IStandardBridge.finalizeBridgeETH, (alice, bob, _value, _extraData));

        // the L2 bridge should call
        // L2CrossDomainMessenger.sendMessage
        vm.expectCall(
            address(l2CrossDomainMessenger),
            abi.encodeCall(ICrossDomainMessenger.sendMessage, (address(l1StandardBridge), message, _minGasLimit))
        );

        vm.expectEmit(address(l2StandardBridge));
        emit ETHBridgeInitiated(alice, bob, _value, _extraData);

        // SentMessage event emitted by the CrossDomainMessenger
        vm.expectEmit(address(l2CrossDomainMessenger));
        emit SentMessage(address(l1StandardBridge), address(l2StandardBridge), message, nonce, _minGasLimit);

        // SentMessageExtension1 event emitted by the CrossDomainMessenger
        vm.expectEmit(address(l2CrossDomainMessenger));
        emit SentMessageExtension1(address(l2StandardBridge), _value);

        // deposit eth to bob
        vm.deal(alice, _value);
        vm.prank(alice, alice);

        l2StandardBridge.bridgeETHTo{ value: _value }(bob, _minGasLimit, _extraData);
    }

    /// @notice Tests that `finalizeBridgeETH` succeeds.
    function test_finalizeBridgeETH_succeeds() external {
        address messenger = address(l2StandardBridge.messenger());
        vm.mockCall(
            messenger,
            abi.encodeCall(ICrossDomainMessenger.xDomainMessageSender, ()),
            abi.encode(address(l2StandardBridge.OTHER_BRIDGE()))
        );
        vm.deal(messenger, 100);
        vm.prank(messenger);

        vm.expectEmit(true, true, true, true);
        emit DepositFinalized(address(0), Predeploys.LEGACY_ERC20_ETH, alice, alice, 100, hex"");

        vm.expectEmit(true, true, true, true);
        emit ETHBridgeFinalized(alice, alice, 100, hex"");

        l2StandardBridge.finalizeBridgeETH{ value: 100 }(alice, alice, 100, hex"");
    }

    /// @notice Tests that `finalizeBridgeERC20` succeeds.
    function test_finalizeBridgeERC20_succeeds() external {
        address messenger = address(l2StandardBridge.messenger());
        address localToken = address(L2Token);
        address remoteToken = address(L1Token);
        vm.mockCall(
            messenger,
            abi.encodeCall(ICrossDomainMessenger.xDomainMessageSender, ()),
            abi.encode(address(l2StandardBridge.OTHER_BRIDGE()))
        );
        deal(localToken, messenger, 100, true);
        vm.prank(messenger);

        vm.expectEmit(true, true, true, true);
        emit DepositFinalized(remoteToken, localToken, alice, alice, 100, hex"");

        vm.expectEmit(true, true, true, true);
        emit ERC20BridgeFinalized(localToken, remoteToken, alice, alice, 100, hex"");

        l2StandardBridge.finalizeBridgeERC20(localToken, remoteToken, alice, alice, 100, hex"");
    }

    function test_finalizeBridgeERC20_isNotCorrectTokenPair_reverts() external {
        address messenger = address(l2StandardBridge.messenger());
        address localToken = address(L2Token);
        address remoteToken = address(BadL1Token);
        vm.mockCall(
            messenger,
            abi.encodeCall(ICrossDomainMessenger.xDomainMessageSender, ()),
            abi.encode(address(l2StandardBridge.OTHER_BRIDGE()))
        );
        deal(localToken, messenger, 100, true);
        vm.prank(messenger);

        vm.expectRevert("StandardBridge: wrong remote token for Optimism Mintable ERC20 local token");
        l2StandardBridge.finalizeBridgeERC20(localToken, remoteToken, alice, alice, 100, hex"");
    }
}
