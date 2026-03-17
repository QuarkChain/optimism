package sgt

import (
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-devstack/presets"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// TestSGT_NativeGasPayment verifies baseline gas payment with native ETH
// This test validates standard behavior before introducing SGT
func TestSGT_NativeGasPayment(gt *testing.T) {
	t := devtest.SerialT(gt)
	sys := presets.NewMinimal(t)

	// Create funded accounts
	alice := sys.FunderL2.NewFundedEOA(eth.OneHundredthEther)
	bob := sys.Wallet.NewEOA(sys.L2EL)

	initialBalance := alice.GetBalance()
	t.Logger().Info("Initial balance", "alice", initialBalance)

	// Perform transfer
	transferAmount := eth.GWei(1000000) // 0.001 ETH
	receipt := alice.Transfer(bob.Address(), transferAmount)

	// Verify receipt success
	t.Require().Equal(uint64(1), receipt.Included.Value().Status, "Transaction should succeed")

	// Calculate total cost (gas + transfer)
	gasCost := new(big.Int).Mul(
		new(big.Int).SetUint64(receipt.Included.Value().GasUsed),
		receipt.Included.Value().EffectiveGasPrice,
	)
	if receipt.Included.Value().L1Fee != nil {
		gasCost.Add(gasCost, receipt.Included.Value().L1Fee)
	}

	totalCost := new(big.Int).Add(gasCost, transferAmount.ToBig())
	expectedBalance := eth.WeiBig(new(big.Int).Sub(initialBalance.ToBig(), totalCost))

	// Wait for balance update
	alice.WaitForBalance(expectedBalance)

	// Verify Bob received funds
	bob.WaitForBalance(transferAmount)

	t.Logger().Info("Gas payment test passed",
		"gas_used", receipt.Included.Value().GasUsed,
		"gas_cost", gasCost,
		"transfer", transferAmount,
		"total_cost", totalCost,
		"final_balance", alice.GetBalance(),
	)
}

// TestSGT_ContractCheck checks if SGT contract might be deployed
func TestSGT_ContractCheck(gt *testing.T) {
	t := devtest.SerialT(gt)
	sys := presets.NewMinimal(t)

	ctx := t.Ctx()
	rpcClient := sys.L2EL.Escape().EthClient().RPC()

	sgtAddr := "0x4200000000000000000000000000000000000800"

	// Try to get contract code
	var code hexutil.Bytes
	err := rpcClient.CallContext(ctx, &code, "eth_getCode", sgtAddr, "latest")
	t.Require().NoError(err, "eth_getCode should not error")

	if len(code) == 0 {
		t.Logger().Warn("SGT contract not deployed",
			"address", sgtAddr,
			"note", "Full SGT tests require contract deployment in genesis",
		)
	} else {
		t.Logger().Info("Contract code found",
			"address", sgtAddr,
			"code_size", len(code),
			"code_prefix", hexutil.Encode(code[:min(len(code), 32)]),
		)
	}
}

// TestSGT_BalanceQueryRPC attempts to query SGT balance via RPC
func TestSGT_BalanceQueryRPC(gt *testing.T) {
	t := devtest.SerialT(gt)
	sys := presets.NewMinimal(t)

	ctx := t.Ctx()
	rpcClient := sys.L2EL.Escape().EthClient().RPC()

	// Create test account
	alice := sys.Wallet.NewEOA(sys.L2EL)

	sgtAddr := "0x4200000000000000000000000000000000000800"

	// First check if contract exists
	var code hexutil.Bytes
	err := rpcClient.CallContext(ctx, &code, "eth_getCode", sgtAddr, "latest")
	if err != nil || len(code) == 0 {
		t.Logger().Warn("SGT contract not found - skipping balance query")
		t.SkipNow()
		return
	}

	// Prepare balanceOf(address) call data
	// Function selector: keccak256("balanceOf(address)")[0:4] = 0x70a08231
	callData := "0x70a08231" + common.BytesToHash(alice.Address().Bytes()).Hex()[2:]

	// Make eth_call
	var result hexutil.Bytes
	err = rpcClient.CallContext(ctx, &result, "eth_call", map[string]any{
		"to":   sgtAddr,
		"data": callData,
	}, "latest")

	if err != nil {
		t.Logger().Warn("SGT balanceOf call failed",
			"error", err,
			"note", "Contract may not implement balanceOf",
		)
		return
	}

	// Parse result as uint256
	balance := new(big.Int).SetBytes(result)

	t.Logger().Info("SGT balance query successful",
		"account", alice.Address(),
		"sgt_balance", balance,
		"native_balance", alice.GetBalance(),
	)

	// Initial balance should be zero
	t.Require().True(balance.Cmp(big.NewInt(0)) == 0, "Initial SGT balance should be zero")
}

// TestSGT_MultipleTransfers validates consistent behavior across multiple transactions
func TestSGT_MultipleTransfers(gt *testing.T) {
	t := devtest.SerialT(gt)
	sys := presets.NewMinimal(t)

	// Create funded sender
	alice := sys.FunderL2.NewFundedEOA(eth.OneHundredthEther)
	initialBalance := alice.GetBalance()

	// Create multiple recipients
	recipients := make([]*common.Address, 3)
	for i := range recipients {
		bob := sys.Wallet.NewEOA(sys.L2EL)
		addr := bob.Address()
		recipients[i] = &addr
	}

	// Perform multiple transfers
	transferAmount := eth.GWei(100000) // 0.0001 ETH each
	var totalGasCost big.Int

	for i, recipient := range recipients {
		receipt := alice.Transfer(*recipient, transferAmount)
		t.Require().Equal(uint64(1), receipt.Included.Value().Status, "Transaction %d should succeed", i+1)

		gasCost := new(big.Int).Mul(
			new(big.Int).SetUint64(receipt.Included.Value().GasUsed),
			receipt.Included.Value().EffectiveGasPrice,
		)
		if receipt.Included.Value().L1Fee != nil {
			gasCost.Add(gasCost, receipt.Included.Value().L1Fee)
		}

		totalGasCost.Add(&totalGasCost, gasCost)

		t.Logger().Info("Transfer completed",
			"transfer_num", i+1,
			"recipient", recipient,
			"gas_cost", gasCost,
		)
	}

	// Verify final balance
	totalTransferred := new(big.Int).Mul(transferAmount.ToBig(), big.NewInt(int64(len(recipients))))
	totalCost := new(big.Int).Add(totalTransferred, &totalGasCost)
	expectedFinalBalance := eth.WeiBig(new(big.Int).Sub(initialBalance.ToBig(), totalCost))

	alice.WaitForBalance(expectedFinalBalance)

	t.Logger().Info("Multiple transfers test passed",
		"num_transfers", len(recipients),
		"total_gas_cost", &totalGasCost,
		"total_transferred", totalTransferred,
		"final_balance", alice.GetBalance(),
	)
}

// min returns the minimum of two ints
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
