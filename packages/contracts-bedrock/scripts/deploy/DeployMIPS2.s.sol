// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Forge
import { Script } from "forge-std/Script.sol";

// Libraries
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

// Interfaces
import { IPreimageOracle } from "interfaces/cannon/IPreimageOracle.sol";
import { IMIPS } from "interfaces/cannon/IMIPS.sol";

/// @title DeployMIPS
contract DeployMIPS2 is Script {
    struct Input {
        // Specify the PreimageOracle to use
        IPreimageOracle preimageOracle;
        // Specify which MIPS version to use.
        uint256 mipsVersion;
    }

    struct Output {
        IMIPS mipsSingleton;
    }

    function run(Input memory _input) public returns (Output memory output_) {
        assertValidInput(_input);

        deployMipsSingleton(_input, output_);

        assertValidOutput(_input, output_);
    }

    function deployMipsSingleton(Input memory _input, Output memory _output) internal {
        uint256 mipsVersion = _input.mipsVersion;
        string memory contractName = mipsVersion == 1 ? "MIPS" : "MIPS64";

        IMIPS singleton = IMIPS(
            DeployUtils.createDeterministic({
                _name: contractName,
                _args: DeployUtils.encodeConstructor(abi.encodeCall(IMIPS.__constructor__, (_input.preimageOracle))),
                _salt: DeployUtils.DEFAULT_SALT
            })
        );

        vm.label(address(singleton), "MIPSSingleton");
        _output.mipsSingleton = singleton;
    }

    function assertValidInput(Input memory _input) public pure {
        require(address(_input.preimageOracle) != address(0), "DeployMIPS: preimageOracle not set");
        require(_input.mipsVersion != 0, "DeployMIPS: mipsVersion not set");
        require(_input.mipsVersion == 1 || _input.mipsVersion == 2, "DeployMIPS: unknown mips version");
    }

    function assertValidOutput(Input memory _input, Output memory _output) public view {
        DeployUtils.assertValidContractAddress(address(_output.mipsSingleton));
        require(address(_output.mipsSingleton.oracle()) == address(_input.preimageOracle), "MIPS-10");
    }
}
