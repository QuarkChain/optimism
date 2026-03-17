//! Soul Gas Token (SGT) error types.

use alloc::string::String;
use alloy_primitives::{Address, U256};

/// SGT-related errors
#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
pub enum SgtError {
    /// Insufficient funds to cover transaction cost
    #[error("insufficient funds: account {account} has {available} (native: {native}, sgt: {sgt}), needs {required}")]
    InsufficientFunds {
        /// The account address
        account: Address,
        /// Available native balance
        native: U256,
        /// Available SGT balance
        sgt: U256,
        /// Total available balance
        available: U256,
        /// Required amount
        required: U256,
    },

    /// Storage read failed
    #[error("SGT storage read failed for account {account}: {reason}")]
    StorageReadFailed {
        /// The account address
        account: Address,
        /// Error reason
        reason: String,
    },

    /// Storage write failed
    #[error("SGT storage write failed for account {account}: {reason}")]
    StorageWriteFailed {
        /// The account address
        account: Address,
        /// Error reason
        reason: String,
    },

    /// SGT contract not found at expected address
    #[error("SGT contract not found at address {address}")]
    ContractNotFound {
        /// The expected contract address
        address: Address,
    },

    /// Invalid SGT configuration
    #[error("invalid SGT configuration: {reason}")]
    InvalidConfig {
        /// Configuration error reason
        reason: String,
    },

    /// Arithmetic overflow/underflow
    #[error("arithmetic error during SGT operation: {reason}")]
    ArithmeticError {
        /// Error reason
        reason: String,
    },

    /// Database error
    #[error("database error: {0}")]
    DatabaseError(String),
}

impl SgtError {
    /// Create an insufficient funds error
    pub fn insufficient_funds(
        account: Address,
        native: U256,
        sgt: U256,
        required: U256,
    ) -> Self {
        Self::InsufficientFunds {
            account,
            native,
            sgt,
            available: native.saturating_add(sgt),
            required,
        }
    }

    /// Create a storage read error
    pub fn storage_read_failed(account: Address, reason: impl Into<String>) -> Self {
        Self::StorageReadFailed { account, reason: reason.into() }
    }

    /// Create a storage write error
    pub fn storage_write_failed(account: Address, reason: impl Into<String>) -> Self {
        Self::StorageWriteFailed { account, reason: reason.into() }
    }

    /// Create a database error
    pub fn database_error(reason: impl Into<String>) -> Self {
        Self::DatabaseError(reason.into())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use alloy_primitives::address;

    #[test]
    fn test_insufficient_funds_error() {
        let account = address!("0000000000000000000000000000000000000001");
        let native = U256::from(100);
        let sgt = U256::from(50);
        let required = U256::from(200);

        let error = SgtError::insufficient_funds(account, native, sgt, required);

        match error {
            SgtError::InsufficientFunds { account: addr, native: n, sgt: s, available, required: r } => {
                assert_eq!(addr, account);
                assert_eq!(n, native);
                assert_eq!(s, sgt);
                assert_eq!(available, U256::from(150));
                assert_eq!(r, required);
            }
            _ => panic!("Expected InsufficientFunds error"),
        }
    }

    #[test]
    fn test_error_display() {
        let account = address!("0000000000000000000000000000000000000001");
        let error = SgtError::storage_read_failed(account, "test error");

        assert!(error.to_string().contains("storage read failed"));
        assert!(error.to_string().contains("test error"));
    }
}
