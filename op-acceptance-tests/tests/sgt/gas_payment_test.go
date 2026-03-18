package sgt

import (
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-devstack/dsl"
	"github.com/ethereum-optimism/optimism/op-devstack/presets"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/txplan"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// largeSGT is a value large enough to fully cover gas costs on devnet (including L1 data fees).
var largeSGT = new(big.Int).Mul(big.NewInt(10), big.NewInt(1e18)) // 10 ETH

// largeNative is a value large enough to fully cover gas costs on devnet.
var largeNative = new(big.Int).Mul(big.NewInt(5), big.NewInt(1e18)) // 5 ETH

// smallBalance is too small to cover gas by itself.
var smallBalance = big.NewInt(10_000)

// TestSGT_NativaGasPaymentWithoutSGTSuccess verifies gas is paid entirely from native
// balance when the account has no SGT.
func TestSGT_NativaGasPaymentWithoutSGTSuccess(gt *testing.T) {
	t := devtest.SerialT(gt)
	sys := presets.NewMinimal(t)
	h := NewSgtHelper(t, sys)

	account := setupAccountWithBalances(t, h, big.NewInt(0), largeNative)

	preSGT := h.GetSGTBalance(account.Address())
	preNative := h.GetNativeBalance(account.Address())
	vaultBefore := h.CalcVaultBalance()

	receipt := executeTransfer(t, h, account, dummyAddr, big.NewInt(0))
	gasCost := CalcGasFee(receipt)

	postSGT := h.GetSGTBalance(account.Address())
	vaultAfter := h.CalcVaultBalance()

	// Vault increased by gas cost
	vaultDiff := new(big.Int).Sub(vaultAfter, vaultBefore)
	t.Require().Equal(0, vaultDiff.Cmp(gasCost), "vault balance diff should equal gas cost")

	// SGT should remain zero
	t.Require().Equal(0, postSGT.Sign(), "SGT balance should remain zero")

	// Balance invariant
	h.InvariantBalanceCheck(account.Address(), gasCost, big.NewInt(0), preSGT, preNative, postSGT, false)
}

// TestSGT_FullSGTGasPaymentWithoutNativeBalanceSuccess verifies gas is paid entirely
// from SGT when the account has no native balance.
func TestSGT_FullSGTGasPaymentWithoutNativeBalanceSuccess(gt *testing.T) {
	t := devtest.SerialT(gt)
	sys := presets.NewMinimal(t)
	h := NewSgtHelper(t, sys)

	account := setupAccountWithBalances(t, h, largeSGT, big.NewInt(0))

	preSGT := h.GetSGTBalance(account.Address())
	preNative := h.GetNativeBalance(account.Address())
	vaultBefore := h.CalcVaultBalance()

	receipt := executeTransfer(t, h, account, dummyAddr, big.NewInt(0))
	gasCost := CalcGasFee(receipt)

	postSGT := h.GetSGTBalance(account.Address())
	vaultAfter := h.CalcVaultBalance()

	// Vault increased by gas cost
	vaultDiff := new(big.Int).Sub(vaultAfter, vaultBefore)
	t.Require().Equal(0, vaultDiff.Cmp(gasCost), "vault balance diff should equal gas cost")

	// SGT should decrease by gas cost
	expectedSGT := new(big.Int).Sub(preSGT, gasCost)
	t.Require().Equal(0, expectedSGT.Cmp(postSGT), "SGT should be reduced by gas cost")

	h.InvariantBalanceCheck(account.Address(), gasCost, big.NewInt(0), preSGT, preNative, postSGT, true)
}

// TestSGT_FullSGTGasPaymentWithNativeBalanceSuccess verifies that when both SGT and
// native balances are available, SGT is used first for gas payment.
func TestSGT_FullSGTGasPaymentWithNativeBalanceSuccess(gt *testing.T) {
	t := devtest.SerialT(gt)
	sys := presets.NewMinimal(t)
	h := NewSgtHelper(t, sys)

	account := setupAccountWithBalances(t, h, largeSGT, largeNative)

	preSGT := h.GetSGTBalance(account.Address())
	preNative := h.GetNativeBalance(account.Address())
	vaultBefore := h.CalcVaultBalance()

	receipt := executeTransfer(t, h, account, dummyAddr, big.NewInt(0))
	gasCost := CalcGasFee(receipt)

	postSGT := h.GetSGTBalance(account.Address())
	postNative := h.GetNativeBalance(account.Address())
	vaultAfter := h.CalcVaultBalance()

	// Vault increased by gas cost
	vaultDiff := new(big.Int).Sub(vaultAfter, vaultBefore)
	t.Require().Equal(0, vaultDiff.Cmp(gasCost), "vault balance diff should equal gas cost")

	// SGT should be used first (deduction: SGT first -> native second)
	expectedSGT := new(big.Int).Sub(preSGT, gasCost)
	t.Require().Equal(0, expectedSGT.Cmp(postSGT), "SGT should be reduced by gas cost")

	// Native should remain unchanged (SGT covers all gas)
	t.Require().Equal(0, preNative.Cmp(postNative), "native should not change (SGT priority)")

	h.InvariantBalanceCheck(account.Address(), gasCost, big.NewInt(0), preSGT, preNative, postSGT, true)
}

// TestSGT_PartialSGTGasPaymentSuccess verifies that when SGT balance is less than gas
// cost, SGT is fully spent and the remainder is paid from native balance.
func TestSGT_PartialSGTGasPaymentSuccess(gt *testing.T) {
	t := devtest.SerialT(gt)
	sys := presets.NewMinimal(t)
	h := NewSgtHelper(t, sys)

	account := setupAccountWithBalances(t, h, big.NewInt(1000), largeNative)

	preSGT := h.GetSGTBalance(account.Address())
	preNative := h.GetNativeBalance(account.Address())
	vaultBefore := h.CalcVaultBalance()

	receipt := executeTransfer(t, h, account, dummyAddr, big.NewInt(0))
	gasCost := CalcGasFee(receipt)

	postSGT := h.GetSGTBalance(account.Address())
	vaultAfter := h.CalcVaultBalance()

	// Vault increased by gas cost
	vaultDiff := new(big.Int).Sub(vaultAfter, vaultBefore)
	t.Require().Equal(0, vaultDiff.Cmp(gasCost), "vault balance diff should equal gas cost")

	// SGT should be fully spent
	t.Require().Equal(0, postSGT.Sign(), "SGT should be fully spent in partial payment")

	h.InvariantBalanceCheck(account.Address(), gasCost, big.NewInt(0), preSGT, preNative, postSGT, true)
}

// TestSGT_FullSGTGasPaymentAndNonZeroTxValueWithSufficientNativeBalanceSuccess verifies
// that SGT covers gas while tx value is paid from native balance.
func TestSGT_FullSGTGasPaymentAndNonZeroTxValueWithSufficientNativeBalanceSuccess(gt *testing.T) {
	t := devtest.SerialT(gt)
	sys := presets.NewMinimal(t)
	h := NewSgtHelper(t, sys)

	txValue := big.NewInt(10000)
	account := setupAccountWithBalances(t, h, largeSGT, largeNative)

	preSGT := h.GetSGTBalance(account.Address())
	preNative := h.GetNativeBalance(account.Address())
	vaultBefore := h.CalcVaultBalance()

	receipt := executeTransfer(t, h, account, dummyAddr, txValue)
	gasCost := CalcGasFee(receipt)

	postSGT := h.GetSGTBalance(account.Address())
	vaultAfter := h.CalcVaultBalance()

	// Vault increased by gas cost
	vaultDiff := new(big.Int).Sub(vaultAfter, vaultBefore)
	t.Require().Equal(0, vaultDiff.Cmp(gasCost), "vault balance diff should equal gas cost")

	// SGT used for gas
	expectedSGT := new(big.Int).Sub(preSGT, gasCost)
	t.Require().Equal(0, expectedSGT.Cmp(postSGT), "SGT should be reduced by gas cost")

	h.InvariantBalanceCheck(account.Address(), gasCost, txValue, preSGT, preNative, postSGT, true)
}

// TestSGT_PartialSGTGasPaymentAndNonZeroTxValueWithSufficientNativeBalanceSuccess verifies
// partial SGT gas payment with a non-zero tx value.
func TestSGT_PartialSGTGasPaymentAndNonZeroTxValueWithSufficientNativeBalanceSuccess(gt *testing.T) {
	t := devtest.SerialT(gt)
	sys := presets.NewMinimal(t)
	h := NewSgtHelper(t, sys)

	txValue := big.NewInt(10000)
	account := setupAccountWithBalances(t, h, big.NewInt(1000), largeNative)

	preSGT := h.GetSGTBalance(account.Address())
	preNative := h.GetNativeBalance(account.Address())
	vaultBefore := h.CalcVaultBalance()

	receipt := executeTransfer(t, h, account, dummyAddr, txValue)
	gasCost := CalcGasFee(receipt)

	postSGT := h.GetSGTBalance(account.Address())
	vaultAfter := h.CalcVaultBalance()

	// Vault increased by gas cost
	vaultDiff := new(big.Int).Sub(vaultAfter, vaultBefore)
	t.Require().Equal(0, vaultDiff.Cmp(gasCost), "vault balance diff should equal gas cost")

	// SGT fully spent
	t.Require().Equal(0, postSGT.Sign(), "SGT should be fully spent")

	h.InvariantBalanceCheck(account.Address(), gasCost, txValue, preSGT, preNative, postSGT, true)
}

// TestSGT_FullSGTInsufficientGasPaymentFail verifies that a transaction fails when
// the account has only a small SGT balance and no native balance.
func TestSGT_FullSGTInsufficientGasPaymentFail(gt *testing.T) {
	t := devtest.SerialT(gt)
	sys := presets.NewMinimal(t)
	h := NewSgtHelper(t, sys)

	account := setupAccountWithBalances(t, h, new(big.Int).Set(smallBalance), big.NewInt(0))
	assertTransferFails(t, account, dummyAddr, big.NewInt(0))
}

// TestSGT_FullNativeInsufficientGasPaymentFail verifies that a transaction fails when
// the account has only a small native balance and no SGT balance.
func TestSGT_FullNativeInsufficientGasPaymentFail(gt *testing.T) {
	t := devtest.SerialT(gt)
	sys := presets.NewMinimal(t)
	h := NewSgtHelper(t, sys)

	account := setupAccountWithBalances(t, h, big.NewInt(0), new(big.Int).Set(smallBalance))
	assertTransferFails(t, account, dummyAddr, big.NewInt(0))
}

// TestSGT_PartialSGTInsufficientGasPaymentFail verifies that a transaction fails
// when the combined SGT and native balance is insufficient to cover gas.
func TestSGT_PartialSGTInsufficientGasPaymentFail(gt *testing.T) {
	t := devtest.SerialT(gt)
	sys := presets.NewMinimal(t)
	h := NewSgtHelper(t, sys)

	account := setupAccountWithBalances(t, h, new(big.Int).Set(smallBalance), new(big.Int).Set(smallBalance))
	assertTransferFails(t, account, dummyAddr, big.NewInt(0))
}

// TestSGT_FullSGTGasPaymentAndNonZeroTxValueWithInsufficientNativeBalanceFail verifies
// failure when SGT covers gas but native balance is insufficient to cover tx value.
func TestSGT_FullSGTGasPaymentAndNonZeroTxValueWithInsufficientNativeBalanceFail(gt *testing.T) {
	t := devtest.SerialT(gt)
	sys := presets.NewMinimal(t)
	h := NewSgtHelper(t, sys)

	account := setupAccountWithBalances(t, h, largeSGT, new(big.Int).Set(smallBalance))
	txValue := new(big.Int).Add(smallBalance, big.NewInt(1))
	assertTransferFails(t, account, dummyAddr, txValue)
}

// TestSGT_PartialSGTGasPaymentAndNonZeroTxValueWithInsufficientNativeBalanceFail verifies
// failure when partial SGT covers some gas but native is insufficient for tx value after gas remainder.
func TestSGT_PartialSGTGasPaymentAndNonZeroTxValueWithInsufficientNativeBalanceFail(gt *testing.T) {
	t := devtest.SerialT(gt)
	sys := presets.NewMinimal(t)
	h := NewSgtHelper(t, sys)

	account := setupAccountWithBalances(t, h, new(big.Int).Set(smallBalance), largeNative)
	txValue := new(big.Int).Sub(largeNative, smallBalance)
	assertTransferFails(t, account, dummyAddr, txValue)
}

// TestSGT_BalanceInvariant explicitly tests the balance invariant:
// preSGT + preNative == postSGT + postNative + gasCost + txValue
func TestSGT_BalanceInvariant(gt *testing.T) {
	t := devtest.SerialT(gt)
	sys := presets.NewMinimal(t)
	h := NewSgtHelper(t, sys)

	txValue := big.NewInt(42000)
	account := setupAccountWithBalances(t, h, largeSGT, largeNative)

	preSGT := h.GetSGTBalance(account.Address())
	preNative := h.GetNativeBalance(account.Address())

	receipt := executeTransfer(t, h, account, dummyAddr, txValue)
	gasCost := CalcGasFee(receipt)

	postSGT := h.GetSGTBalance(account.Address())
	postNative := h.GetNativeBalance(account.Address())

	preBal := new(big.Int).Add(preSGT, preNative)
	postBal := new(big.Int).Add(postSGT, postNative)
	postBal = postBal.Add(postBal, gasCost)
	postBal = postBal.Add(postBal, txValue)

	t.Require().Equal(0, preBal.Cmp(postBal),
		"balance invariant: pre(%s) != post+gas+value(%s)", preBal, postBal)
}

// setupAccountWithBalances creates a fresh account with exact SGT and native balances.
func setupAccountWithBalances(t devtest.T, h *SgtHelper, sgtDeposit, nativeDeposit *big.Int) *dsl.EOA {
	setupGas := new(big.Int).Mul(big.NewInt(1), big.NewInt(1e18)) // 1 ETH for setup gas
	totalNeeded := new(big.Int).Add(sgtDeposit, nativeDeposit)
	totalNeeded = totalNeeded.Add(totalNeeded, setupGas)

	funder := h.sys.FunderL2.NewFundedEOA(eth.WeiBig(totalNeeded))

	// Create the test account (fresh, zero balances)
	testAccount := h.sys.Wallet.NewEOA(h.sys.L2EL)

	// Deposit SGT if needed (funder deposits to testAccount via batchDepositForAll)
	if sgtDeposit.Sign() > 0 {
		h.DepositSGT(funder, testAccount.Address(), sgtDeposit)
	}

	// Transfer native if needed
	if nativeDeposit.Sign() > 0 {
		funder.Transfer(testAccount.Address(), eth.WeiBig(nativeDeposit))
	}

	// Verify SGT balance
	actualSGT := h.GetSGTBalance(testAccount.Address())
	t.Require().Equal(0, sgtDeposit.Cmp(actualSGT),
		"SGT deposit mismatch: got %s, want %s", actualSGT, sgtDeposit)

	// Verify native balance
	actualNative := h.GetNativeBalance(testAccount.Address())
	t.Require().Equal(0, nativeDeposit.Cmp(actualNative),
		"native deposit mismatch: got %s, want %s", actualNative, nativeDeposit)

	return testAccount
}

// executeTransfer sends a native transfer from the account to the target.
func executeTransfer(t devtest.T, h *SgtHelper, account *dsl.EOA, target common.Address, txValue *big.Int) *types.Receipt {
	plannedTx := account.Transact(
		account.PlanTransfer(target, eth.WeiBig(txValue)),
	)
	receipt, err := plannedTx.Included.Get()
	t.Require().NoError(err, "failed to get receipt")
	t.Require().Equal(uint64(1), receipt.Status, "tx should succeed")
	return receipt
}

// assertTransferFails verifies that a transfer tx fails (e.g., insufficient funds).
func assertTransferFails(t devtest.T, account *dsl.EOA, target common.Address, txValue *big.Int) {
	tx := txplan.NewPlannedTx(
		account.PlanTransfer(target, eth.WeiBig(txValue)),
	)
	_, err := tx.Success.Eval(t.Ctx())
	t.Require().Error(err, "transaction should fail due to insufficient funds")
}
