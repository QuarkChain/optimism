//! SGT (Soul Gas Token) balance reading utilities for RPC

use alloy_op_evm::sgt::{sgt_balance_slot, SGT_CONTRACT};
use alloy_primitives::{Address, U256};
use reth_provider::{ProviderError, StateProvider};
use reth_storage_api::StateProviderFactory;

/// Read SGT balance for an account from contract storage.
///
/// Returns the SGT balance stored at slot 51 in the SGT predeploy contract.
pub fn read_sgt_balance<SP>(
    state: &SP,
    address: Address,
) -> Result<U256, ProviderError>
where
    SP: StateProvider,
{
    let slot = sgt_balance_slot(address);
    let value = state.storage(SGT_CONTRACT, slot.into())?
        .unwrap_or_default();
    Ok(value)
}

/// Read SGT balance using a [`StateProviderFactory`] (gets latest state).
pub fn read_sgt_balance_from_provider<F>(client: &F, address: Address) -> Result<U256, ProviderError>
where
    F: StateProviderFactory,
{
    let state = client.latest()?;
    read_sgt_balance(&state, address)
}
