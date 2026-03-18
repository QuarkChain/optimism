//! Soul Gas Token (SGT) support for OP Stack
//!
//! This module provides functionality for alternative gas payment using Soul Gas Tokens (SGT).
//! SGT enables transactions to pay gas fees using tokens from a predeploy contract instead of
//! native ETH.
//!
//! # Gas Payment Priority
//!
//! - **Deduction**: SGT first → Native balance second
//! - **Refund**: Native balance first → SGT second (reverse order)
//!
//! # Two Modes
//!
//! 1. **Native-backed**: SGT 1:1 with native token (backed by native balance)
//! 2. **Independent**: SGT is separate currency (minted/burned)

use alloy_primitives::{Address, address};

// Re-export from op-revm to avoid duplication
pub use op_revm::sgt::sgt_balance_slot;

/// SGT contract predeploy address
pub const SGT_CONTRACT: Address =
    address!("4200000000000000000000000000000000000800");

/// Balance mapping base slot (must match Solidity contract)
///
/// The SGT contract stores balances in a mapping at slot 51:
/// ```solidity
/// mapping(address => uint256) public balances; // slot 51
/// ```
pub const SGT_BALANCE_SLOT: u64 = 51;

/// SGT configuration
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct SgtConfig {
    /// Whether SGT is enabled
    pub enabled: bool,

    /// Whether SGT is backed 1:1 by native token
    pub is_native_backed: bool,
}

impl Default for SgtConfig {
    fn default() -> Self {
        Self {
            enabled: false,
            is_native_backed: true,
        }
    }
}

