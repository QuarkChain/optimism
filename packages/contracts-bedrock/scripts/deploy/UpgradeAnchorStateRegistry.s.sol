// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

// Forge
import { Script } from "forge-std/Script.sol";
import { console } from "forge-std/console.sol";

// Scripts
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

// Libraries
import { GameType, Hash, OutputRoot } from "src/dispute/lib/Types.sol";
// Contracts
import { IStorageSetter } from "interfaces/universal/IStorageSetter.sol";
// Interfaces
import { IAnchorStateRegistry } from "interfaces/dispute/IAnchorStateRegistry.sol";
import { IDisputeGameFactory } from "interfaces/dispute/IDisputeGameFactory.sol";
import { IProxyAdmin } from "interfaces/universal/IProxyAdmin.sol";
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";
import { IOptimismPortal2 } from "interfaces/L1/IOptimismPortal2.sol";

/// @title UpgradeAnchorStateRegistry
contract UpgradeAnchorStateRegistry is Script {
    function run(
        address _portal,
        address _disputeGameFactoryProxy,
        address _opChainProxyAdmin,
        address _anchorStateRegistryProxy,
        address _superchainConfig,
        address _storageSetter,
        uint256 _l2BlockNumber,
        bytes32 _outputRoot
    )
        public
    {
        console.log("_portal: %s", _portal);
        console.log("_disputeGameFactoryProxy: %s", _disputeGameFactoryProxy);
        console.log("_opChainProxyAdmin: %s", _opChainProxyAdmin);
        console.log("_anchorStateRegistryProxy: %s", _anchorStateRegistryProxy);
        console.log("_superchainConfig: %s", _superchainConfig);
        console.log("_storageSetter: %s", _storageSetter);
        console.log("_l2BlockNumber: %s", _l2BlockNumber);
        console.log("_outputRoot: %s", bytes32ToHex(_outputRoot));

        vm.startBroadcast();
        upgradeAnchorStateRegistryImpl(
            IOptimismPortal2(payable(_portal)),
            IDisputeGameFactory(_disputeGameFactoryProxy),
            IProxyAdmin(_opChainProxyAdmin),
            IAnchorStateRegistry(_anchorStateRegistryProxy),
            ISuperchainConfig(_superchainConfig),
            _storageSetter,
            _l2BlockNumber,
            Hash.wrap(_outputRoot)
        );
        vm.stopBroadcast();
        checkOutput(IAnchorStateRegistry(_anchorStateRegistryProxy), _l2BlockNumber, Hash.wrap(_outputRoot));
    }

    function upgradeAnchorStateRegistryImpl(
        IOptimismPortal2 _portal,
        IDisputeGameFactory _disputeGameFactoryProxy,
        IProxyAdmin _opChainProxyAdmin,
        IAnchorStateRegistry _anchorStateRegistryProxy,
        ISuperchainConfig _superchainConfig,
        address _storageSetter,
        uint256 _l2BlockNumber,
        Hash _outputRoot
    )
        internal
    {
        address anchorStateRegistryImpl = DeployUtils.create1({
            _name: "AnchorStateRegistry",
            _args: DeployUtils.encodeConstructor(abi.encodeCall(IAnchorStateRegistry.__constructor__, ()))
        });

        if (_storageSetter == address(0)) {
            _storageSetter = DeployUtils.create1({
                _name: "StorageSetter",
                _args: DeployUtils.encodeConstructor(abi.encodeCall(IStorageSetter.__constructor__, ()))
            });
        }

        bytes memory data;
        data = encodeStorageSetterZeroOutInitializedSlot();
        upgradeAndCall(_opChainProxyAdmin, address(_anchorStateRegistryProxy), _storageSetter, data);

        data = encodeAnchorStateRegistryInitializer(
            _l2BlockNumber, _outputRoot, _superchainConfig, _disputeGameFactoryProxy, _portal
        );
        upgradeAndCall(_opChainProxyAdmin, address(_anchorStateRegistryProxy), anchorStateRegistryImpl, data);
    }

    function encodeStorageSetterZeroOutInitializedSlot() internal pure returns (bytes memory) {
        return abi.encodeCall(IStorageSetter.setBytes32, (0, 0));
    }

    function encodeAnchorStateRegistryInitializer(
        uint256 _l2BlockNumber,
        Hash _outputRoot,
        ISuperchainConfig _superchainConfig,
        IDisputeGameFactory _disputeGameFactory,
        IOptimismPortal2 _portal
    )
        internal
        view
        virtual
        returns (bytes memory)
    {
        return abi.encodeCall(
            IAnchorStateRegistry.initialize,
            (
                _superchainConfig,
                _disputeGameFactory,
                _portal,
                OutputRoot({ root: _outputRoot, l2BlockNumber: _l2BlockNumber })
            )
        );
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

    function checkOutput(
        IAnchorStateRegistry _anchorStateRegistryProxy,
        uint256 _l2BlockNumber,
        Hash _outputRoot
    )
        public
        view
    {
        (Hash root, uint256 l2BlockNumber) = IAnchorStateRegistry(_anchorStateRegistryProxy).getAnchorRoot();
        require(Hash.unwrap(root) == Hash.unwrap(_outputRoot), "UpgradeAnchorStateRegistryOutput: root mismatch");
        require(l2BlockNumber == _l2BlockNumber, "UpgradeAnchorStateRegistryOutput: l2BlockNumber mismatch");
    }

    function bytes32ToHex(bytes32 _data) internal pure returns (string memory) {
        bytes memory result = new bytes(64);
        for (uint256 i = 0; i < 32; i++) {
            uint8 byteValue = uint8(_data[i]);
            result[i * 2] = toHexChar(byteValue / 16);
            result[i * 2 + 1] = toHexChar(byteValue % 16);
        }
        return string(result);
    }

    function toHexChar(uint8 _b) internal pure returns (bytes1) {
        return _b < 10 ? bytes1(_b + 0x30) : bytes1(_b + 0x57);
    }
}
