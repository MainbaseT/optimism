// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

// Testing utilities
import { Test } from "forge-std/Test.sol";

// Target contractS
import { TransientContext } from "src/libraries/TransientContext.sol";
import { TransientReentrancyAware } from "src/libraries/TransientContext.sol";

/// @title TransientContext_TestInit
/// @notice Reusable test initialization for `TransientContext` tests.
contract TransientContext_TestInit is Test {
    /// @notice Slot for call depth.
    bytes32 internal callDepthSlot = bytes32(uint256(keccak256("transient.calldepth")) - 1);
}

/// @title TransientContext_CallDepth_Test
/// @notice Tests the `callDepth` function of the `TransientContext` library.
contract TransientContext_CallDepth_Test is TransientContext_TestInit {
    /// @notice Tests that `callDepth()` outputs the corrects call depth.
    /// @param _callDepth Call depth to test.
    function testFuzz_callDepth_succeeds(uint256 _callDepth) public {
        assembly ("memory-safe") {
            tstore(sload(callDepthSlot.slot), _callDepth)
        }
        assertEq(TransientContext.callDepth(), _callDepth);
    }
}

/// @title TransientContext_Increment_Test
/// @notice Tests the `increment` function of the `TransientContext` library.
contract TransientContext_Increment_Test is TransientContext_TestInit {
    /// @notice Tests that `increment()` increments the call depth.
    /// @param _startingCallDepth Starting call depth.
    function testFuzz_increment_succeeds(uint256 _startingCallDepth) public {
        _startingCallDepth = bound(_startingCallDepth, 0, type(uint256).max - 1);
        assembly ("memory-safe") {
            tstore(sload(callDepthSlot.slot), _startingCallDepth)
        }
        assertEq(TransientContext.callDepth(), _startingCallDepth);

        TransientContext.increment();
        assertEq(TransientContext.callDepth(), _startingCallDepth + 1);
    }
}

/// @title TransientContext_Decrement_Test
/// @notice Tests the `decrement` function of the `TransientContext` library.
contract TransientContext_Decrement_Test is TransientContext_TestInit {
    /// @notice Tests that `decrement()` decrements the call depth.
    /// @param _startingCallDepth Starting call depth.
    function testFuzz_decrement_succeeds(uint256 _startingCallDepth) public {
        _startingCallDepth = bound(_startingCallDepth, 1, type(uint256).max);
        assembly ("memory-safe") {
            tstore(sload(callDepthSlot.slot), _startingCallDepth)
        }
        assertEq(TransientContext.callDepth(), _startingCallDepth);

        TransientContext.decrement();
        assertEq(TransientContext.callDepth(), _startingCallDepth - 1);
    }
}

/// @title TransientContext_Get_Test
/// @notice Tests the `get` function of the `TransientContext` library.
contract TransientContext_Get_Test is TransientContext_TestInit {
    /// @notice Tests that `get()` returns the correct value.
    /// @param _slot  Slot to test.
    /// @param _value Value to test.
    function testFuzz_get_succeeds(bytes32 _slot, uint256 _value) public {
        assertEq(TransientContext.get(_slot), 0);

        bytes32 tSlot = keccak256(abi.encodePacked(TransientContext.callDepth(), _slot));
        assembly ("memory-safe") {
            tstore(tSlot, _value)
        }

        assertEq(TransientContext.get(_slot), _value);
    }
}

/// @title TransientContext_Set_Test
/// @notice Tests the `set` function of the `TransientContext` library.
contract TransientContext_Set_Test is TransientContext_TestInit {
    /// @notice Tests that `set()` sets the correct value.
    /// @param _slot  Slot to test.
    /// @param _value Value to test.
    function testFuzz_set_succeeds(bytes32 _slot, uint256 _value) public {
        TransientContext.set(_slot, _value);
        bytes32 tSlot = keccak256(abi.encodePacked(TransientContext.callDepth(), _slot));
        uint256 tValue;
        assembly ("memory-safe") {
            tValue := tload(tSlot)
        }
        assertEq(tValue, _value);
    }
}

/// @title TransientContext_ReentrantAware_Test
/// @notice Tests the `reentrantAware` modifier of the `TransientContext` library.
contract TransientContext_ReentrantAware_Test is TransientContext_TestInit, TransientReentrancyAware {
    /// @notice Reentrant-aware mock function to set a value in transient storage.
    /// @param _slot  Slot to set.
    /// @param _value Value to set.
    function mock(bytes32 _slot, uint256 _value) internal reentrantAware {
        TransientContext.set(_slot, _value);
    }

    /// @notice Reentrant-aware mock function to set a value in transient storage at multiple
    ///         depths.
    /// @param _slot   Slot to set.
    /// @param _value1 Value to set at call depth 1.
    /// @param _value2 Value to set at call depth 2.
    function mockMultiDepth(bytes32 _slot, uint256 _value1, uint256 _value2) internal reentrantAware {
        TransientContext.set(_slot, _value1);
        mock(_slot, _value2);
    }

    /// @notice Tests the mock function is reentrant-aware.
    /// @param _callDepth Call depth to test.
    /// @param _slot      Slot to test.
    /// @param _value     Value to test.
    function testFuzz_reentrantAware_succeeds(uint256 _callDepth, bytes32 _slot, uint256 _value) public {
        _callDepth = bound(_callDepth, 0, type(uint256).max - 1);
        assembly ("memory-safe") {
            tstore(sload(callDepthSlot.slot), _callDepth)
        }
        assertEq(TransientContext.callDepth(), _callDepth);

        mock(_slot, _value);

        assertEq(TransientContext.get(_slot), 0);

        TransientContext.increment();
        assertEq(TransientContext.callDepth(), _callDepth + 1);
        assertEq(TransientContext.get(_slot), _value);
    }

    /// @notice Tests the mock function is reentrant-aware at multiple depths.
    /// @param _callDepth Call depth to test.
    /// @param _slot      Slot to test.
    /// @param _value1    Value to test at call depth 1.
    /// @param _value2    Value to test at call depth 2.
    function testFuzz_reentrantAware_multiDepth_succeeds(
        uint256 _callDepth,
        bytes32 _slot,
        uint256 _value1,
        uint256 _value2
    )
        public
    {
        _callDepth = bound(_callDepth, 0, type(uint256).max - 2);
        assembly ("memory-safe") {
            tstore(sload(callDepthSlot.slot), _callDepth)
        }
        assertEq(TransientContext.callDepth(), _callDepth);

        mockMultiDepth(_slot, _value1, _value2);

        assertEq(TransientContext.get(_slot), 0);

        TransientContext.increment();
        assertEq(TransientContext.callDepth(), _callDepth + 1);
        assertEq(TransientContext.get(_slot), _value1);

        TransientContext.increment();
        assertEq(TransientContext.callDepth(), _callDepth + 2);
        assertEq(TransientContext.get(_slot), _value2);
    }
}

/// @title TransientContext_Unclassified_Test
/// @notice General tests that are not testing any function directly of the `TransientContext`
///         contract or are testing multiple functions at once.
contract TransientContext_Unclassified_Test is TransientContext_Set_Test {
    /// @notice Tests that `set()` and `get()` work together.
    /// @param _slot  Slot to test.
    /// @param _value Value to test.
    function testFuzz_setGet_succeeds(bytes32 _slot, uint256 _value) public {
        testFuzz_set_succeeds(_slot, _value);
        assertEq(TransientContext.get(_slot), _value);
    }

    /// @notice Tests that `set()` and `get()` work together at the same depth.
    /// @param _slot    Slot to test.
    /// @param _value1  Value to write to slot at call depth 0.
    /// @param _value2  Value to write to slot at call depth 1.
    function testFuzz_setGet_twiceSameDepth_succeeds(bytes32 _slot, uint256 _value1, uint256 _value2) public {
        assertEq(TransientContext.callDepth(), 0);
        testFuzz_set_succeeds(_slot, _value1);
        assertEq(TransientContext.get(_slot), _value1);

        assertEq(TransientContext.callDepth(), 0);
        testFuzz_set_succeeds(_slot, _value2);
        assertEq(TransientContext.get(_slot), _value2);
    }

    /// @notice Tests that `set()` and `get()` work together at different depths.
    /// @param _slot    Slot to test.
    /// @param _value1  Value to write to slot at call depth 0.
    /// @param _value2  Value to write to slot at call depth 1.
    function testFuzz_setGet_twiceDifferentDepth_succeeds(bytes32 _slot, uint256 _value1, uint256 _value2) public {
        assertEq(TransientContext.callDepth(), 0);
        testFuzz_set_succeeds(_slot, _value1);
        assertEq(TransientContext.get(_slot), _value1);

        TransientContext.increment();

        assertEq(TransientContext.callDepth(), 1);
        testFuzz_set_succeeds(_slot, _value2);
        assertEq(TransientContext.get(_slot), _value2);

        TransientContext.decrement();

        assertEq(TransientContext.callDepth(), 0);
        assertEq(TransientContext.get(_slot), _value1);
    }
}
