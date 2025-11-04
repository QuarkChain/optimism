// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

// Forge
import { Script } from "forge-std/Script.sol";
import { console } from "forge-std/console.sol";

// Scripts
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

// Libraries
import { GameType, Hash, Proposal } from "src/dispute/lib/Types.sol";
// Contracts
import { IStorageSetter } from "interfaces/universal/IStorageSetter.sol";
// Interfaces
import { IAnchorStateRegistry } from "interfaces/dispute/IAnchorStateRegistry.sol";
import { IDisputeGameFactory } from "interfaces/dispute/IDisputeGameFactory.sol";
import { IAnchorStateRegistry } from "interfaces/dispute/IAnchorStateRegistry.sol";
import { IProxyAdmin } from "interfaces/universal/IProxyAdmin.sol";
import { ISystemConfig } from "interfaces/L1/ISystemConfig.sol";

/// @title UpgradeAnchorStateRegistry
contract UpgradeAnchorStateRegistry is Script {
    function run(
        address _disputeGameFactoryProxy,
        address _opChainProxyAdmin,
        address _anchorStateRegistryProxy,
        address _systemConfig,
        address _storageSetter,
        uint32 _type,
        Proposal memory _startingAnchorRoot
    )
        public
    {
        console.log("_disputeGameFactoryProxy: %s", _disputeGameFactoryProxy);
        console.log("_opChainProxyAdmin: %s", _opChainProxyAdmin);
        console.log("_anchorStateRegistryProxy: %s", _anchorStateRegistryProxy);
        console.log("_systemConfig: %s", _systemConfig);
        console.log("_storageSetter: %s", _storageSetter);
        console.log("_type: %s", _type);
        console.log("_l2BlockNumber: %s", _startingAnchorRoot.l2SequenceNumber);
        console.log("_outputRoot: %s", bytes32ToHex(Hash.unwrap(_startingAnchorRoot.root)));

        vm.startBroadcast();
        upgradeAnchorStateRegistryImpl(
            IDisputeGameFactory(_disputeGameFactoryProxy),
            IProxyAdmin(_opChainProxyAdmin),
            IAnchorStateRegistry(_anchorStateRegistryProxy),
            ISystemConfig(_systemConfig),
            _storageSetter,
            GameType.wrap(_type),
            _startingAnchorRoot
        );
        vm.stopBroadcast();
        checkOutput(IAnchorStateRegistry(_anchorStateRegistryProxy), GameType.wrap(_type), _startingAnchorRoot);
    }

    function upgradeAnchorStateRegistryImpl(
        IDisputeGameFactory _disputeGameFactoryProxy,
        IProxyAdmin _opChainProxyAdmin,
        IAnchorStateRegistry _anchorStateRegistryProxy,
        ISystemConfig _systemConfig,
        address _storageSetter,
        GameType _type,
        Proposal memory _startingAnchorRoot
    )
        internal
    {
        address anchorStateRegistryImpl = DeployUtils.create1({
            _name: "AnchorStateRegistry",
            _args: DeployUtils.encodeConstructor(
                abi.encodeCall(
                    IAnchorStateRegistry.__constructor__, (_anchorStateRegistryProxy.disputeGameFinalityDelaySeconds())
                )
            )
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
        data = encodeAnchorStateRegistryInitializer(_disputeGameFactoryProxy, _type, _startingAnchorRoot, _systemConfig);
        upgradeAndCall(_opChainProxyAdmin, address(_anchorStateRegistryProxy), anchorStateRegistryImpl, data);
    }

    function encodeStorageSetterZeroOutInitializedSlot() internal pure returns (bytes memory) {
        return abi.encodeCall(IStorageSetter.setBytes32, (0, 0));
    }

    function encodeAnchorStateRegistryInitializer(
        IDisputeGameFactory _disputeGameFactory,
        GameType _type,
        Proposal memory _startingAnchorRoot,
        ISystemConfig _systemConfig
    )
        internal
        view
        virtual
        returns (bytes memory)
    {
        return abi.encodeCall(
            IAnchorStateRegistry.initialize, (_systemConfig, _disputeGameFactory, _startingAnchorRoot, _type)
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
        GameType _type,
        Proposal memory _startingAnchorRoot
    )
        public
        view
    {
        (Hash root, uint256 l2BlockNumber) = IAnchorStateRegistry(_anchorStateRegistryProxy).anchors(_type);
        require(
            Hash.unwrap(root) == Hash.unwrap(_startingAnchorRoot.root),
            "UpgradeAnchorStateRegistryOutput: root mismatch"
        );
        require(
            l2BlockNumber == _startingAnchorRoot.l2SequenceNumber,
            "UpgradeAnchorStateRegistryOutput: l2BlockNumber mismatch"
        );
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
