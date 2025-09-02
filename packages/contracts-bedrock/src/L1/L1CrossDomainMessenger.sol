// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { ProxyAdminOwnedBase } from "src/L1/ProxyAdminOwnedBase.sol";
import { ReinitializableBase } from "src/universal/ReinitializableBase.sol";
import { CrossDomainMessenger } from "src/universal/CrossDomainMessenger.sol";

// Libraries
import { Predeploys } from "src/libraries/Predeploys.sol";

// Interfaces
import { ISemver } from "interfaces/universal/ISemver.sol";
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";
import { ISystemConfig } from "interfaces/L1/ISystemConfig.sol";
import { IOptimismPortal2 as IOptimismPortal } from "interfaces/L1/IOptimismPortal2.sol";

/// @custom:proxied true
/// @title L1CrossDomainMessenger
/// @notice The L1CrossDomainMessenger is a message passing interface between L1 and L2 responsible
///         for sending and receiving data on the L1 side. Users are encouraged to use this
///         interface instead of interacting with lower-level contracts directly.
contract L1CrossDomainMessenger is CrossDomainMessenger, ProxyAdminOwnedBase, ReinitializableBase, ISemver {
    /// @notice Thrown when the caller is not the minter.
    error L1CrossDomainMessenger_NotMinter();

    /// @custom:legacy
    /// @custom:spacer superchainConfig
    /// @notice Spacer taking up the legacy `superchainConfig` slot.
    address private spacer_251_0_20;

    /// @notice Contract of the OptimismPortal.
    /// @custom:network-specific
    IOptimismPortal public portal;

    /// @custom:legacy
    /// @custom:spacer systemConfig
    /// @notice Spacer taking up the legacy `systemConfig` slot.
    address private spacer_253_0_20;

    /// @notice Semantic version.
    /// @custom:semver 2.9.0
    string public constant version = "2.9.0";

    /// @notice Contract of the SystemConfig.
    ISystemConfig public systemConfig;

    /// @notice Emitted when a minter is set.
    event MinterSet(address indexed minter);

    /// @notice Constructs the L1CrossDomainMessenger contract.
    constructor() ReinitializableBase(2) {
        _disableInitializers();
    }

    /// @notice Initializes the contract.
    /// @param _systemConfig Contract of the SystemConfig contract on this network.
    /// @param _portal Contract of the OptimismPortal contract on this network.
    function initialize(ISystemConfig _systemConfig, IOptimismPortal _portal) external reinitializer(initVersion()) {
        // Initialization transactions must come from the ProxyAdmin or its owner.
        _assertOnlyProxyAdminOrProxyAdminOwner();

        // Now perform initialization logic.
        systemConfig = _systemConfig;
        portal = _portal;
        __CrossDomainMessenger_init({ _otherMessenger: CrossDomainMessenger(Predeploys.L2_CROSS_DOMAIN_MESSENGER) });
    }

    /// @notice Upgrades the contract to have a reference to the SystemConfig.
    /// @param _systemConfig The new SystemConfig contract.
    function upgrade(ISystemConfig _systemConfig) external reinitializer(initVersion()) {
        // Upgrade transactions must come from the ProxyAdmin or its owner.
        _assertOnlyProxyAdminOrProxyAdminOwner();

        // Now perform upgrade logic.
        systemConfig = _systemConfig;
    }

    /// @inheritdoc CrossDomainMessenger
    function paused() public view override returns (bool) {
        return systemConfig.paused();
    }

    /// @notice Returns the SuperchainConfig contract.
    /// @return ISuperchainConfig The SuperchainConfig contract.
    function superchainConfig() public view returns (ISuperchainConfig) {
        return systemConfig.superchainConfig();
    }

    /// @notice Getter function for the OptimismPortal contract on this chain.
    ///         Public getter is legacy and will be removed in the future. Use `portal()` instead.
    /// @return Contract of the OptimismPortal on this chain.
    /// @custom:legacy
    function PORTAL() external view returns (IOptimismPortal) {
        return portal;
    }

    // keccak256(abi.encode(uint256(keccak256("openzeppelin.storage.L1CrossDomainMessenger.QKCConfigStorage")) - 1)) &
    // ~bytes32(uint256(0xff))
    bytes32 private constant _QKC_CONFIG_STORAGE_LOCATION =
        0x21f30a216d738aeb55799dae7148f127e3b8f70b0224a5edb846c108cd573c00;
    /// @custom:storage-location erc7201:openzeppelin.storage.L1CrossDomainMessenger.QKCConfigStorage

    struct QKCConfigStorage {
        /// @notice The minter for migrating existing L1 token to L2 native token.
        address minter;
    }

    function _getQKCConfigStorage() private pure returns (QKCConfigStorage storage $) {
        assembly {
            $.slot := _QKC_CONFIG_STORAGE_LOCATION
        }
    }

    /// @notice Add a minter to the L1CrossDomainMessenger contract. To disable, set an empty value.
    function setMinter(address _minter) external {
        _assertOnlyProxyAdminOrProxyAdminOwner();
        QKCConfigStorage storage $ = _getQKCConfigStorage();
        $.minter = _minter;
        emit MinterSet(_minter);
    }

    /// @notice Triggers a QKC mint message via the relayMessage function on L2. Can only be called by the minter.
    /// @dev    This function can only be called by the minter.
    /// @param _target      Target contract or wallet address.
    /// @param _message     Message to trigger the target address with.
    /// @param _mintValue   Value to mint.
    /// @param _minGasLimit Minimum gas limit that the message can be executed with.
    function sendMintMessage(
        address _target,
        bytes calldata _message,
        uint256 _mintValue,
        uint32 _minGasLimit
    )
        external
    {
        QKCConfigStorage storage $ = _getQKCConfigStorage();
        if (msg.sender != $.minter) {
            revert L1CrossDomainMessenger_NotMinter();
        }

        portal.mintTransaction({
            _to: address(otherMessenger),
            _mintValue: _mintValue,
            _gasLimit: baseGas(_message, _minGasLimit),
            _data: abi.encodeWithSelector(
                this.relayMessage.selector, messageNonce(), msg.sender, _target, _mintValue, _minGasLimit, _message
            )
        });

        unchecked {
            ++msgNonce;
        }
    }

    /// @inheritdoc CrossDomainMessenger
    function _sendMessage(address _to, uint64 _gasLimit, uint256 _value, bytes memory _data) internal override {
        portal.depositTransaction{ value: _value }({
            _to: _to,
            _value: _value,
            _gasLimit: _gasLimit,
            _isCreation: false,
            _data: _data
        });
    }

    /// @inheritdoc CrossDomainMessenger
    function _isOtherMessenger() internal view override returns (bool) {
        return msg.sender == address(portal) && portal.l2Sender() == address(otherMessenger);
    }

    /// @inheritdoc CrossDomainMessenger
    function _isUnsafeTarget(address _target) internal view override returns (bool) {
        return _target == address(this) || _target == address(portal);
    }
}
