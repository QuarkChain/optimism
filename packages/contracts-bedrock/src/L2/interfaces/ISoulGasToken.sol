// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/// @title ISoulGasToken
/// @notice The interface for the SoulGasToken.
interface ISoulGasToken {
    function initialize(string memory name_, string memory symbol_, address owner_) external;

    function __constructor__() external;
}
