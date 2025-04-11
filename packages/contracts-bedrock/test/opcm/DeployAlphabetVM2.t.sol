// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { Test } from "forge-std/Test.sol";

// Interfaces
import { IPreimageOracle } from "interfaces/cannon/IPreimageOracle.sol";

import { DeployAlphabetVM2 } from "scripts/deploy/DeployAlphabetVM2.s.sol";

contract DeployAlphabetVM2_Test is Test {
    DeployAlphabetVM2 deployAlphanetVM;

    IPreimageOracle private preimageOracle = IPreimageOracle(makeAddr("preimageOracle"));
    bytes32 private absolutePrestate = bytes32(uint256(1));

    function setUp() public {
        deployAlphanetVM = new DeployAlphabetVM2();
    }

    function test_run_succeeds() public {
        DeployAlphabetVM2.Input memory input = defaultInput();
        DeployAlphabetVM2.Output memory output = deployAlphanetVM.run(input);

        assertNotEq(address(output.alphabetVM), address(0), "100");
        assertEq(address(output.alphabetVM.oracle()), address(input.preimageOracle), "200");
    }

    function test_run_nullInput_reverts() public {
        DeployAlphabetVM2.Input memory input;

        input = defaultInput();
        input.absolutePrestate = hex"";
        vm.expectRevert("DeployAlphabetVM: absolutePrestate not set");
        deployAlphanetVM.run(input);

        input = defaultInput();
        input.preimageOracle = IPreimageOracle(address(0));
        vm.expectRevert("DeployAlphabetVM: preimageOracle not set");
        deployAlphanetVM.run(input);
    }

    function defaultInput() private view returns (DeployAlphabetVM2.Input memory input_) {
        input_ = DeployAlphabetVM2.Input(absolutePrestate, preimageOracle);
    }
}
