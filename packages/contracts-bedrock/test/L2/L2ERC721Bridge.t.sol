// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { CommonTest } from "test/setup/CommonTest.sol";

// Contracts
import { ERC721 } from "@openzeppelin/contracts/token/ERC721/ERC721.sol";
import { OptimismMintableERC721 } from "src/L2/OptimismMintableERC721.sol";

// Interfaces
import { IL1ERC721Bridge } from "interfaces/L1/IL1ERC721Bridge.sol";
import { IL2ERC721Bridge } from "interfaces/L2/IL2ERC721Bridge.sol";

/// @title TestERC721
/// @notice A test ERC721 token used for `L2ERC721Bridge` tests.
contract L2ERC721Bridge_TestERC721_Harness is ERC721 {
    constructor() ERC721("Test", "TST") { }

    function mint(address to, uint256 tokenId) public {
        _mint(to, tokenId);
    }
}

/// @title TestMintableERC721
/// @notice A test OptimismMintableERC721 token used for `L2ERC721Bridge` tests.
contract L2ERC721Bridge_TestMintableERC721_Harness is OptimismMintableERC721 {
    constructor(
        address _bridge,
        address _remoteToken
    )
        OptimismMintableERC721(_bridge, 1, _remoteToken, "Test", "TST")
    { }

    function mint(address to, uint256 tokenId) public {
        _mint(to, tokenId);
    }
}

/// @title NonCompliantERC721
/// @notice A non-compliant ERC721 token that does not implement the full ERC721 interface.
///         This is used to test that the bridge will revert if the token does not claim to
///         support the ERC721 interface.
contract L2ERC721Bridge_NonCompliantERC721_Harness {
    address internal immutable owner;

    constructor(address _owner) {
        owner = _owner;
    }

    function ownerOf(uint256) external view returns (address) {
        return owner;
    }

    function remoteToken() external pure returns (address) {
        return address(0x01);
    }

    function burn(address, uint256) external {
        // Do nothing.
    }

    function supportsInterface(bytes4) external pure returns (bool) {
        return false;
    }
}

/// @title L2ERC721Bridge_TestInit
/// @notice Reusable test initialization for `L2ERC721Bridge` tests.
contract L2ERC721Bridge_TestInit is CommonTest {
    L2ERC721Bridge_TestMintableERC721_Harness internal localToken;
    L2ERC721Bridge_TestERC721_Harness internal remoteToken;
    uint256 internal constant tokenId = 1;

    event ERC721BridgeInitiated(
        address indexed localToken,
        address indexed remoteToken,
        address indexed from,
        address to,
        uint256 tokenId,
        bytes extraData
    );

    event ERC721BridgeFinalized(
        address indexed localToken,
        address indexed remoteToken,
        address indexed from,
        address to,
        uint256 tokenId,
        bytes extraData
    );

    /// @notice Sets up the test suite.
    function setUp() public override {
        super.setUp();

        remoteToken = new L2ERC721Bridge_TestERC721_Harness();
        localToken = new L2ERC721Bridge_TestMintableERC721_Harness(address(l2ERC721Bridge), address(remoteToken));

        // Mint alice a token.
        localToken.mint(alice, tokenId);

        // Approve the bridge to transfer the token.
        vm.prank(alice);
        localToken.approve(address(l2ERC721Bridge), tokenId);
    }
}

/// @title L2ERC721Bridge_Test_Constructor
/// @notice Tests the `constructor` of the `L2ERC721Bridge` contract.
contract L2ERC721Bridge_Constructor_Test is L2ERC721Bridge_TestInit {
    /// @notice Tests that the constructor sets the correct variables.
    function test_constructor_succeeds() public view {
        assertEq(address(l2ERC721Bridge.MESSENGER()), address(l2CrossDomainMessenger));
        assertEq(address(l2ERC721Bridge.OTHER_BRIDGE()), address(l1ERC721Bridge));
        assertEq(address(l2ERC721Bridge.messenger()), address(l2CrossDomainMessenger));
        assertEq(address(l2ERC721Bridge.otherBridge()), address(l1ERC721Bridge));
    }
}

/// @title L2ERC721Bridge_FinalizeBridgeERC721_Test
/// @notice Tests the `finalizeBridgeERC721` function of the `L2ERC721Bridge` contract.
contract L2ERC721Bridge_FinalizeBridgeERC721_Test is L2ERC721Bridge_TestInit {
    /// @notice Tests that `finalizeBridgeERC721` correctly finalizes a bridged token.
    function test_finalizeBridgeERC721_succeeds() external {
        // Bridge the token.
        vm.prank(alice, alice);
        l2ERC721Bridge.bridgeERC721(address(localToken), address(remoteToken), tokenId, 1234, hex"5678");

        // Expect an event to be emitted.
        vm.expectEmit(true, true, true, true);
        emit ERC721BridgeFinalized(address(localToken), address(remoteToken), alice, alice, tokenId, hex"5678");

        // Finalize a withdrawal.
        vm.mockCall(
            address(l2CrossDomainMessenger),
            abi.encodeCall(l2CrossDomainMessenger.xDomainMessageSender, ()),
            abi.encode(l1ERC721Bridge)
        );
        vm.prank(address(l2CrossDomainMessenger));
        l2ERC721Bridge.finalizeBridgeERC721(address(localToken), address(remoteToken), alice, alice, tokenId, hex"5678");

        // Token is not locked in the bridge.
        assertEq(localToken.ownerOf(tokenId), alice);
    }

    /// @notice Tests that `finalizeBridgeERC721` reverts if the token is not compliant with the
    ///         `IOptimismMintableERC721` interface.
    function test_finalizeBridgeERC721_interfaceNotCompliant_reverts() external {
        // Create a non-compliant token
        L2ERC721Bridge_NonCompliantERC721_Harness nonCompliantToken =
            new L2ERC721Bridge_NonCompliantERC721_Harness(alice);

        // Bridge the non-compliant token.
        vm.prank(alice, alice);
        l2ERC721Bridge.bridgeERC721(address(nonCompliantToken), address(0x01), tokenId, 1234, hex"5678");

        // Attempt to finalize the withdrawal. Should revert because the token does not claim to be
        // compliant with the `IOptimismMintableERC721` interface.
        vm.mockCall(
            address(l2CrossDomainMessenger),
            abi.encodeCall(l2CrossDomainMessenger.xDomainMessageSender, ()),
            abi.encode(l1ERC721Bridge)
        );
        vm.prank(address(l2CrossDomainMessenger));
        vm.expectRevert("L2ERC721Bridge: local token interface is not compliant");
        l2ERC721Bridge.finalizeBridgeERC721(
            address(address(nonCompliantToken)), address(address(0x01)), alice, alice, tokenId, hex"5678"
        );
    }

    /// @notice Tests that `finalizeBridgeERC721` reverts when not called by the remote bridge.
    function test_finalizeBridgeERC721_notViaLocalMessenger_reverts() external {
        // Finalize a withdrawal.
        vm.prank(alice);
        vm.expectRevert("ERC721Bridge: function can only be called from the other bridge");
        l2ERC721Bridge.finalizeBridgeERC721(address(localToken), address(remoteToken), alice, alice, tokenId, hex"5678");
    }

    /// @notice Tests that `finalizeBridgeERC721` reverts when not called by the remote bridge.
    function test_finalizeBridgeERC721_notFromRemoteMessenger_reverts() external {
        // Finalize a withdrawal.
        vm.mockCall(
            address(l2CrossDomainMessenger),
            abi.encodeCall(l2CrossDomainMessenger.xDomainMessageSender, ()),
            abi.encode(alice)
        );
        vm.prank(address(l2CrossDomainMessenger));
        vm.expectRevert("ERC721Bridge: function can only be called from the other bridge");
        l2ERC721Bridge.finalizeBridgeERC721(address(localToken), address(remoteToken), alice, alice, tokenId, hex"5678");
    }

    /// @notice Tests that `finalizeBridgeERC721` reverts when the local token is the address of
    ///         the bridge itself.
    function test_finalizeBridgeERC721_selfToken_reverts() external {
        // Finalize a withdrawal.
        vm.mockCall(
            address(l2CrossDomainMessenger),
            abi.encodeCall(l2CrossDomainMessenger.xDomainMessageSender, ()),
            abi.encode(address(l1ERC721Bridge))
        );
        vm.prank(address(l2CrossDomainMessenger));
        vm.expectRevert("L2ERC721Bridge: local token cannot be self");
        l2ERC721Bridge.finalizeBridgeERC721(
            address(l2ERC721Bridge), address(remoteToken), alice, alice, tokenId, hex"5678"
        );
    }

    /// @notice Tests that `finalizeBridgeERC721` reverts when already finalized.
    function test_finalizeBridgeERC721_alreadyExists_reverts() external {
        // Finalize a withdrawal.
        vm.mockCall(
            address(l2CrossDomainMessenger),
            abi.encodeCall(l2CrossDomainMessenger.xDomainMessageSender, ()),
            abi.encode(address(l1ERC721Bridge))
        );
        vm.prank(address(l2CrossDomainMessenger));
        vm.expectRevert("ERC721: token already minted");
        l2ERC721Bridge.finalizeBridgeERC721(address(localToken), address(remoteToken), alice, alice, tokenId, hex"5678");
    }
}

/// @title L2ERC721Bridge_Uncategorized_Test
/// @notice General tests that are not testing any function directly of the `L2ERC721Bridge`
///         contract or are testing multiple functions at once.
contract L2ERC721Bridge_Uncategorized_Test is L2ERC721Bridge_TestInit {
    /// @notice Ensures that the L2ERC721Bridge is always not paused. The pausability happens on L1
    ///         and not L2.
    function test_paused_succeeds() external view {
        assertFalse(l2ERC721Bridge.paused());
    }

    /// @notice Tests that `bridgeERC721` correctly bridges a token and burns it on the origin
    ///         chain.
    function test_bridgeERC721_succeeds() public {
        // Expect a call to the messenger.
        vm.expectCall(
            address(l2CrossDomainMessenger),
            abi.encodeCall(
                l2CrossDomainMessenger.sendMessage,
                (
                    address(l1ERC721Bridge),
                    abi.encodeCall(
                        IL2ERC721Bridge.finalizeBridgeERC721,
                        (address(remoteToken), address(localToken), alice, alice, tokenId, hex"5678")
                    ),
                    1234
                )
            )
        );

        // Expect an event to be emitted.
        vm.expectEmit(true, true, true, true);
        emit ERC721BridgeInitiated(address(localToken), address(remoteToken), alice, alice, tokenId, hex"5678");

        // Bridge the token.
        vm.prank(alice, alice);
        l2ERC721Bridge.bridgeERC721(address(localToken), address(remoteToken), tokenId, 1234, hex"5678");

        // Token is burned.
        vm.expectRevert("ERC721: invalid token ID");
        localToken.ownerOf(tokenId);
    }

    /// @notice Tests that `bridgeERC721` reverts if the owner is not an EOA.
    function test_bridgeERC721_fromContract_reverts() external {
        // Bridge the token.
        vm.etch(alice, hex"01");
        vm.prank(alice);
        vm.expectRevert("ERC721Bridge: account is not externally owned");
        l2ERC721Bridge.bridgeERC721(address(localToken), address(remoteToken), tokenId, 1234, hex"5678");

        // Token is not locked in the bridge.
        assertEq(localToken.ownerOf(tokenId), alice);
    }

    /// @notice Tests that `bridgeERC721` reverts if the local token is the zero address.
    function test_bridgeERC721_localTokenZeroAddress_reverts() external {
        // Bridge the token.
        vm.prank(alice, alice);
        vm.expectRevert(bytes(""));
        l2ERC721Bridge.bridgeERC721(address(0), address(remoteToken), tokenId, 1234, hex"5678");

        // Token is not locked in the bridge.
        assertEq(localToken.ownerOf(tokenId), alice);
    }

    /// @notice Tests that `bridgeERC721` reverts if the remote token is the zero address.
    function test_bridgeERC721_remoteTokenZeroAddress_reverts() external {
        // Bridge the token.
        vm.prank(alice, alice);
        vm.expectRevert("L2ERC721Bridge: remote token cannot be address(0)");
        l2ERC721Bridge.bridgeERC721(address(localToken), address(0), tokenId, 1234, hex"5678");

        // Token is not locked in the bridge.
        assertEq(localToken.ownerOf(tokenId), alice);
    }

    /// @notice Tests that `bridgeERC721` reverts if the caller is not the token owner.
    function test_bridgeERC721_wrongOwner_reverts() external {
        // Bridge the token.
        vm.prank(bob, bob);
        vm.expectRevert("L2ERC721Bridge: Withdrawal is not being initiated by NFT owner");
        l2ERC721Bridge.bridgeERC721(address(localToken), address(remoteToken), tokenId, 1234, hex"5678");

        // Token is not locked in the bridge.
        assertEq(localToken.ownerOf(tokenId), alice);
    }

    /// @notice Tests that `bridgeERC721To` correctly bridges a token and burns it on the origin
    ///         chain.
    function test_bridgeERC721To_succeeds() external {
        // Expect a call to the messenger.
        vm.expectCall(
            address(l2CrossDomainMessenger),
            abi.encodeCall(
                l2CrossDomainMessenger.sendMessage,
                (
                    address(l1ERC721Bridge),
                    abi.encodeCall(
                        IL1ERC721Bridge.finalizeBridgeERC721,
                        (address(remoteToken), address(localToken), alice, bob, tokenId, hex"5678")
                    ),
                    1234
                )
            )
        );

        // Expect an event to be emitted.
        vm.expectEmit(true, true, true, true);
        emit ERC721BridgeInitiated(address(localToken), address(remoteToken), alice, bob, tokenId, hex"5678");

        // Bridge the token.
        vm.prank(alice);
        l2ERC721Bridge.bridgeERC721To(address(localToken), address(remoteToken), bob, tokenId, 1234, hex"5678");

        // Token is burned.
        vm.expectRevert("ERC721: invalid token ID");
        localToken.ownerOf(tokenId);
    }

    /// @notice Tests that `bridgeERC721To` reverts if the local token is the zero address.
    function test_bridgeERC721To_localTokenZeroAddress_reverts() external {
        // Bridge the token.
        vm.prank(alice);
        vm.expectRevert(bytes(""));
        l2ERC721Bridge.bridgeERC721To(address(0), address(l1ERC721Bridge), bob, tokenId, 1234, hex"5678");

        // Token is not locked in the bridge.
        assertEq(localToken.ownerOf(tokenId), alice);
    }

    /// @notice Tests that `bridgeERC721To` reverts if the remote token is the zero address.
    function test_bridgeERC721To_remoteTokenZeroAddress_reverts() external {
        // Bridge the token.
        vm.prank(alice);
        vm.expectRevert("L2ERC721Bridge: remote token cannot be address(0)");
        l2ERC721Bridge.bridgeERC721To(address(localToken), address(0), bob, tokenId, 1234, hex"5678");

        // Token is not locked in the bridge.
        assertEq(localToken.ownerOf(tokenId), alice);
    }

    /// @notice Tests that `bridgeERC721To` reverts if the caller is not the token owner.
    function test_bridgeERC721To_wrongOwner_reverts() external {
        // Bridge the token.
        vm.prank(bob);
        vm.expectRevert("L2ERC721Bridge: Withdrawal is not being initiated by NFT owner");
        l2ERC721Bridge.bridgeERC721To(address(localToken), address(remoteToken), bob, tokenId, 1234, hex"5678");

        // Token is not locked in the bridge.
        assertEq(localToken.ownerOf(tokenId), alice);
    }

    /// @notice Tests that `bridgeERC721To` reverts if the to address is the zero address.
    function test_bridgeERC721To_toZeroAddress_reverts() external {
        // Bridge the token.
        vm.prank(bob);
        vm.expectRevert("ERC721Bridge: nft recipient cannot be address(0)");
        l2ERC721Bridge.bridgeERC721To(address(localToken), address(remoteToken), address(0), tokenId, 1234, hex"5678");
    }
}
