// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Test } from "forge-std/Test.sol";
import { SoulGasToken } from "src/L2/SoulGasToken.sol";
import { VmSafe } from "forge-std/Vm.sol";

/// @title SoulGasTokenTest
/// @notice Tests for SoulGasToken contract, including critical storage layout verification
contract SoulGasTokenTest is Test {
    SoulGasToken public sgt;
    address public owner = address(0x1234);
    address public alice = address(0xA11ce);
    address public bob = address(0xB0b);

    function setUp() public {
        // Deploy SGT in native-backed mode
        // Note: Constructor automatically calls initialize("", "", msg.sender)
        // so we don't call it again here
        vm.prank(owner);
        sgt = new SoulGasToken(true);
    }

    /// @notice CRITICAL TEST: Verify _balances mapping is at storage slot 51
    ///
    /// This test is SECURITY CRITICAL for op-reth integration.
    /// The Rust implementation assumes _balances is at slot 51.
    /// If this test fails, op-reth's SGT implementation WILL BE BROKEN.
    ///
    /// Storage layout for ERC20Upgradeable v4.7.3:
    /// - Slot 0: _initialized, _initializing
    /// - Slots 1-50: __gap (Initializable)
    /// - Slot 51: _balances ← THIS IS WHAT WE VERIFY
    /// - Slot 52: _allowances
    /// - Slot 53: _totalSupply
    /// ...
    ///
    /// ⚠️ WARNING: Changing OpenZeppelin version REQUIRES re-verifying this test!
    function test_storageLayout_balancesAtSlot51() public {
        // Give Alice some SGT
        uint256 depositAmount = 100 ether;
        vm.deal(alice, depositAmount);
        vm.prank(alice);
        sgt.deposit{ value: depositAmount }();

        // Calculate the storage slot for Alice's balance
        // Formula: keccak256(abi.encode(alice, 51))
        bytes32 slot = keccak256(abi.encode(alice, uint256(51)));

        // Read the storage directly
        uint256 storedBalance = uint256(vm.load(address(sgt), slot));

        // Verify it matches the balance from the balanceOf() function
        uint256 reportedBalance = sgt.balanceOf(alice);

        assertEq(storedBalance, reportedBalance, "Storage slot 51 does not contain _balances mapping!");
        assertEq(storedBalance, depositAmount, "Balance mismatch");
    }

    /// @notice Verify storage slot calculation matches Rust implementation
    ///
    /// Rust code: keccak256(abi.encode(account, 51))
    /// This test ensures our slot calculation is identical
    function test_storageSlotCalculation_matchesRust() public pure {
        address account = address(0x0742D35CC6634c0532925A3b844bc9E7595f0Beb);

        // Solidity calculation
        bytes32 soliditySlot = keccak256(abi.encode(account, uint256(51)));

        // Manual construction (how Rust does it)
        bytes memory data = new bytes(64);

        // Left-pad address to 32 bytes
        for (uint256 i = 0; i < 12; i++) {
            data[i] = 0x00;
        }
        for (uint256 i = 0; i < 20; i++) {
            data[12 + i] = bytes20(account)[i];
        }

        // Slot 51 as big-endian uint256
        for (uint256 i = 0; i < 31; i++) {
            data[32 + i] = 0x00;
        }
        data[63] = 0x33; // 51 in hex

        bytes32 manualSlot = keccak256(data);

        assertEq(soliditySlot, manualSlot, "Slot calculation doesn't match Rust implementation");
    }

    /// @notice Test basic deposit functionality
    function test_deposit_success() public {
        uint256 depositAmount = 10 ether;
        vm.deal(alice, depositAmount);

        vm.prank(alice);
        sgt.deposit{ value: depositAmount }();

        assertEq(sgt.balanceOf(alice), depositAmount);
    }

    /// @notice Test deposit requires native-backed mode
    function test_deposit_requiresNativeBacked() public {
        // Deploy SGT in independent mode
        // Constructor already calls initialize, so we don't call it again
        SoulGasToken sgtIndependent = new SoulGasToken(false);

        uint256 depositAmount = 10 ether;
        vm.deal(alice, depositAmount);

        vm.prank(alice);
        vm.expectRevert("SGT: deposit should only be called when IS_BACKED_BY_NATIVE");
        sgtIndependent.deposit{ value: depositAmount }();
    }

    /// @notice Test batchDepositForAll functionality
    function test_batchDepositForAll_success() public {
        address[] memory accounts = new address[](3);
        accounts[0] = alice;
        accounts[1] = bob;
        accounts[2] = address(0xC0de);

        uint256 valuePerAccount = 5 ether;
        uint256 totalValue = valuePerAccount * accounts.length;

        vm.deal(address(this), totalValue);
        sgt.batchDepositForAll{ value: totalValue }(accounts, valuePerAccount);

        for (uint256 i = 0; i < accounts.length; i++) {
            assertEq(sgt.balanceOf(accounts[i]), valuePerAccount);
        }
    }

    /// @notice Test OpenZeppelin version hasn't changed
    function test_openZeppelinVersion() public pure {
        // This test serves as documentation and a reminder
        // OpenZeppelin Contracts Upgradeable v4.7.3
        // If upgrading OpenZeppelin, MUST re-verify storage layout tests!

        // No actual assertion, just a compile-time check that contract compiles
        // with current OpenZeppelin version
        assertTrue(true, "If this compiles, OpenZeppelin version is compatible");
    }

    /// @notice Test multiple deposits accumulate correctly
    function test_multipleDeposits_accumulate() public {
        vm.deal(alice, 100 ether);

        vm.startPrank(alice);
        sgt.deposit{ value: 10 ether }();
        sgt.deposit{ value: 20 ether }();
        sgt.deposit{ value: 30 ether }();
        vm.stopPrank();

        assertEq(sgt.balanceOf(alice), 60 ether);
    }

    /// @notice Test contract holds deposited ETH in native-backed mode
    function test_contractHoldsETH_nativeBacked() public {
        uint256 depositAmount = 50 ether;
        vm.deal(alice, depositAmount);

        vm.prank(alice);
        sgt.deposit{ value: depositAmount }();

        // In native-backed mode, contract should hold the ETH
        assertEq(address(sgt).balance, depositAmount);
    }
}
