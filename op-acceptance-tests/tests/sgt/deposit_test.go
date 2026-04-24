package sgt

import (
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-devstack/presets"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/txplan"
)

// TestSGT_DepositVia_BatchDepositForAll verifies that SGT can be deposited
// using the batchDepositForAll function and the balance is correctly reflected.
func TestSGT_DepositFunctionSuccess(gt *testing.T) {
	t := devtest.SerialT(gt)
	sys := presets.NewMinimal(t)
	h := NewSgtHelper(t, sys)

	// Create a funder with native balance
	funder := sys.FunderL2.NewFundedEOA(eth.Ether(1))
	recipient := sys.Wallet.NewEOA(sys.L2EL)

	// Check recipient starts with zero SGT
	preSGT := h.GetSGTBalance(recipient.Address())
	t.Require().Equal(0, preSGT.Sign(), "recipient should start with zero SGT")

	// Deposit SGT to recipient
	depositAmount := big.NewInt(10000)
	h.DepositSGT(funder, recipient.Address(), depositAmount)

	// Verify SGT balance after deposit
	postSGT := h.GetSGTBalance(recipient.Address())
	t.Require().Equal(0, depositAmount.Cmp(postSGT),
		"SGT balance should equal deposit amount: got %s, want %s", postSGT, depositAmount)
}

// TestSGT_DepositAccumulation verifies that multiple SGT deposits accumulate correctly.
func TestSGT_DepositAccumulation(gt *testing.T) {
	t := devtest.SerialT(gt)
	sys := presets.NewMinimal(t)
	h := NewSgtHelper(t, sys)

	funder := sys.FunderL2.NewFundedEOA(eth.Ether(1))
	recipient := sys.Wallet.NewEOA(sys.L2EL)

	firstDeposit := big.NewInt(5000)
	secondDeposit := big.NewInt(3000)

	// First deposit
	h.DepositSGT(funder, recipient.Address(), firstDeposit)
	bal1 := h.GetSGTBalance(recipient.Address())
	t.Require().Equal(0, firstDeposit.Cmp(bal1), "after first deposit")

	// Second deposit
	h.DepositSGT(funder, recipient.Address(), secondDeposit)
	bal2 := h.GetSGTBalance(recipient.Address())
	expected := new(big.Int).Add(firstDeposit, secondDeposit)
	t.Require().Equal(0, expected.Cmp(bal2), "after second deposit: got %s, want %s", bal2, expected)
}

// TestSGT_MultiAccountIndependence verifies that SGT deposits to different
// accounts are independent and do not affect each other's balances.
func TestSGT_MultiAccountIndependence(gt *testing.T) {
	t := devtest.SerialT(gt)
	sys := presets.NewMinimal(t)
	h := NewSgtHelper(t, sys)

	funder := sys.FunderL2.NewFundedEOA(eth.Ether(1))
	alice := sys.Wallet.NewEOA(sys.L2EL)
	bob := sys.Wallet.NewEOA(sys.L2EL)

	aliceDeposit := big.NewInt(7000)
	bobDeposit := big.NewInt(3000)

	// Deposit different amounts to each account
	h.DepositSGT(funder, alice.Address(), aliceDeposit)
	h.DepositSGT(funder, bob.Address(), bobDeposit)

	// Verify each account has the correct balance
	aliceBal := h.GetSGTBalance(alice.Address())
	bobBal := h.GetSGTBalance(bob.Address())

	t.Require().Equal(0, aliceDeposit.Cmp(aliceBal), "Alice SGT balance mismatch")
	t.Require().Equal(0, bobDeposit.Cmp(bobBal), "Bob SGT balance mismatch")

	// Deposit more to Alice; Bob should be unaffected
	h.DepositSGT(funder, alice.Address(), aliceDeposit)

	aliceBal2 := h.GetSGTBalance(alice.Address())
	bobBal2 := h.GetSGTBalance(bob.Address())

	expectedAlice := new(big.Int).Add(aliceDeposit, aliceDeposit)
	t.Require().Equal(0, expectedAlice.Cmp(aliceBal2), "Alice SGT balance after second deposit")
	t.Require().Equal(0, bobDeposit.Cmp(bobBal2), "Bob SGT balance should be unchanged")
}

// TestSGT_DepositCreatesNativeBackedBalance verifies that depositing SGT also locks
// native ETH in the SGT contract (since nativeBacked=true in the preset).
func TestSGT_DepositNativeBackingVerification(gt *testing.T) {
	t := devtest.SerialT(gt)
	sys := presets.NewMinimal(t)
	h := NewSgtHelper(t, sys)

	funder := sys.FunderL2.NewFundedEOA(eth.Ether(1))

	// Get SGT contract's native balance before deposit
	preContractBalance := h.GetNativeBalance(sgtAddr)

	depositAmount := big.NewInt(100000)
	calldata := encodeBatchDepositForAll(funder.Address(), depositAmount)
	funder.Transact(
		funder.Plan(),
		txplan.WithTo(&sgtAddr),
		txplan.WithEth(depositAmount),
		txplan.WithData(calldata),
		txplan.WithGasLimit(200000),
	)

	// The SGT contract should have received the native ETH
	postContractBalance := h.GetNativeBalance(sgtAddr)
	diff := new(big.Int).Sub(postContractBalance, preContractBalance)
	t.Require().Equal(0, depositAmount.Cmp(diff),
		"SGT contract native balance should increase by deposit amount: got %s, want %s", diff, depositAmount)
}
