// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

// Forge
import { Script } from "forge-std/Script.sol";

// Scripts
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

// Interfaces
import { IProxyAdmin } from "interfaces/universal/IProxyAdmin.sol";
import { IStorageSetter } from "interfaces/universal/IStorageSetter.sol";
import { ISoulGasToken } from "interfaces/L2/ISoulGasToken.sol";

/// @title UpgradeSoulGasToken
contract UpgradeSoulGasToken is Script {
    IProxyAdmin constant proxyAdmin = IProxyAdmin(0x4200000000000000000000000000000000000018);
    ISoulGasToken constant soulGasToken = ISoulGasToken(0x4200000000000000000000000000000000000800);

    function run(address _storageSetter) public {
        address sgtOwner = getSGTOwner();
        preCheck();

        vm.startBroadcast();
        upgradeSoulGasTokenImpl(_storageSetter);
        vm.stopBroadcast();
        postCheck(sgtOwner);
    }

    function upgradeSoulGasTokenImpl(address _storageSetter) internal {
        if (_storageSetter == address(0)) {
            _storageSetter = DeployUtils.create1({
                _name: "StorageSetter",
                _args: DeployUtils.encodeConstructor(abi.encodeCall(IStorageSetter.__constructor__, ()))
            });
        }

        address sgtOwner = ISoulGasToken(soulGasToken).owner();

        address impl = proxyAdmin.getProxyImplementation(address(soulGasToken));

        bytes memory data;
        data = encodeStorageSetterZeroOutInitializedSlot();
        upgradeAndCall(proxyAdmin, address(soulGasToken), _storageSetter, data);
        data = encodeSoulGasTokenInitializer(sgtOwner);
        upgradeAndCall(proxyAdmin, address(soulGasToken), impl, data);
    }

    function encodeStorageSetterZeroOutInitializedSlot() internal pure returns (bytes memory) {
        // Use selector directly to disambiguate overloaded setBytes32(bytes32,bytes32)
        return abi.encodeWithSelector(bytes4(keccak256("setBytes32(bytes32,bytes32)")), bytes32(0), bytes32(0));
    }

    function encodeSoulGasTokenInitializer(address _sgtOwner) internal view virtual returns (bytes memory) {
        return abi.encodeCall(ISoulGasToken.initialize, ("SoulQKC", "SoulQKC", _sgtOwner));
    }

    /// @notice Makes an external call to the target to initialize the proxy with the specified data.
    /// First performs safety checks to ensure the target, implementation, and proxy admin are valid.
    function upgradeAndCall(
        IProxyAdmin _proxyAdmin,
        address _target,
        address _implementation,
        bytes memory _data
    )
        internal
    {
        DeployUtils.assertValidContractAddress(address(_proxyAdmin));
        DeployUtils.assertValidContractAddress(_target);
        DeployUtils.assertValidContractAddress(_implementation);

        _proxyAdmin.upgradeAndCall(payable(address(_target)), _implementation, _data);
    }

    function getSGTOwner() public view returns (address) {
        return soulGasToken.owner();
    }

    function preCheck() public view {
        address sgtAdmin = proxyAdmin.getProxyAdmin(payable(address(soulGasToken)));
        require(sgtAdmin == address(proxyAdmin), "UpgradeSoulGasToken: admin not match");
    }

    function postCheck(address _sgtOwner) public view {
        require(
            keccak256(abi.encodePacked(soulGasToken.name())) == keccak256("SoulQKC"),
            "UpgradeSoulGasToken: name not match"
        );
        require(
            keccak256(abi.encodePacked(soulGasToken.symbol())) == keccak256("SoulQKC"),
            "UpgradeSoulGasToken: symbol not match"
        );
        require(soulGasToken.owner() == _sgtOwner, "UpgradeSoulGasToken: owner not match");
    }
}
