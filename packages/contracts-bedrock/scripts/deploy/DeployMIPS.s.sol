// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

// Forge
import { Script } from "forge-std/Script.sol";

// Scripts
import { BaseDeployIO } from "scripts/deploy/BaseDeployIO.sol";
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

// Interfaces
import { IPreimageOracle } from "interfaces/cannon/IPreimageOracle.sol";
import { IMIPS64 } from "interfaces/cannon/IMIPS64.sol";
import { StandardConstants } from "scripts/deploy/StandardConstants.sol";

/// @title DeployMIPSInput
contract DeployMIPSInput is BaseDeployIO {
    // Specify the PreimageOracle to use
    address internal _preimageOracle;

    // Specify which MIPS version to use.
    uint256 internal _mipsVersion;

    function set(bytes4 _sel, uint256 _value) public {
        if (_sel == this.mipsVersion.selector) {
            require(_value == StandardConstants.MIPS_VERSION, "DeployMIPS: unsupported mips version");
            _mipsVersion = _value;
        } else {
            revert("DeployMIPS: unknown selector");
        }
    }

    function set(bytes4 _sel, address _value) public {
        if (_sel == this.preimageOracle.selector) {
            require(_value != address(0), "DeployMIPS: preimageOracle cannot be empty");
            _preimageOracle = _value;
        } else {
            revert("DeployMIPS: unknown selector");
        }
    }

    function mipsVersion() public view returns (uint256) {
        require(_mipsVersion != 0, "DeployMIPS: mipsVersion not set");
        require(_mipsVersion == StandardConstants.MIPS_VERSION, "DeployMIPS: unsupported mips version");
        return _mipsVersion;
    }

    function preimageOracle() public view returns (address) {
        require(_preimageOracle != address(0), "DeployMIPS: preimageOracle not set");
        return _preimageOracle;
    }
}

/// @title DeployMIPSOutput
contract DeployMIPSOutput is BaseDeployIO {
    IMIPS64 internal _mipsSingleton;

    function set(bytes4 _sel, address _value) public {
        if (_sel == this.mipsSingleton.selector) {
            require(_value != address(0), "DeployMIPS: mipsSingleton cannot be zero address");
            _mipsSingleton = IMIPS64(_value);
        } else {
            revert("DeployMIPS: unknown selector");
        }
    }

    function mipsSingleton() public view returns (IMIPS64) {
        DeployUtils.assertValidContractAddress(address(_mipsSingleton));
        return _mipsSingleton;
    }
}

/// @title DeployMIPS
contract DeployMIPS is Script {
    function run(DeployMIPSInput _mi, DeployMIPSOutput _mo) public {
        deployMipsSingleton(_mi, _mo);
        assertValidDeploy(_mi, _mo);
    }

    function deployMipsSingleton(DeployMIPSInput _mi, DeployMIPSOutput _mo) internal {
        uint256 mipsVersion = _mi.mipsVersion();
        IPreimageOracle preimageOracle = IPreimageOracle(_mi.preimageOracle());

        IMIPS64 singleton = IMIPS64(
            DeployUtils.createDeterministic({
                _name: "MIPS64",
                _args: DeployUtils.encodeConstructor(abi.encodeCall(IMIPS64.__constructor__, (preimageOracle, mipsVersion))),
                _salt: DeployUtils.DEFAULT_SALT
            })
        );

        vm.label(address(singleton), "MIPSSingleton");
        _mo.set(_mo.mipsSingleton.selector, address(singleton));
    }

    function assertValidDeploy(DeployMIPSInput _mi, DeployMIPSOutput _mo) public view {
        DeployUtils.assertValidContractAddress(address(_mo.mipsSingleton()));
        assertValidMipsSingleton(_mi, _mo);
    }

    function assertValidMipsSingleton(DeployMIPSInput _mi, DeployMIPSOutput _mo) internal view {
        IMIPS64 mips = _mo.mipsSingleton();
        require(address(mips.oracle()) == address(_mi.preimageOracle()), "MIPS-10");
    }
}
