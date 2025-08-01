// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing utilities
import { CommonTest } from "test/setup/CommonTest.sol";

// Libraries
import { Types } from "src/libraries/Types.sol";

/// @title L1FeeVault_Constructor_Test
/// @notice Tests the `constructor` of the `L1FeeVault` contract.
contract L1FeeVault_Constructor_Test is CommonTest {
    /// @notice Tests that the constructor sets the correct values.
    function test_constructor_l1FeeVault_succeeds() external view {
        assertEq(l1FeeVault.RECIPIENT(), deploy.cfg().l1FeeVaultRecipient());
        assertEq(l1FeeVault.recipient(), deploy.cfg().l1FeeVaultRecipient());
        assertEq(l1FeeVault.MIN_WITHDRAWAL_AMOUNT(), deploy.cfg().l1FeeVaultMinimumWithdrawalAmount());
        assertEq(l1FeeVault.minWithdrawalAmount(), deploy.cfg().l1FeeVaultMinimumWithdrawalAmount());
        assertEq(uint8(l1FeeVault.WITHDRAWAL_NETWORK()), uint8(Types.WithdrawalNetwork.L1));
        assertEq(uint8(l1FeeVault.withdrawalNetwork()), uint8(Types.WithdrawalNetwork.L1));
    }
}
