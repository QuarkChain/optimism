package sgt

import (
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-devstack/presets"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

// TestSGT_SmokeNativeTransfer verifies that a simple native ETH transfer works
// on an SGT-enabled chain. The sender pays gas (via SGT if it has SGT balance,
// otherwise via native) and the recipient receives the correct amount.
func TestSGT_SmokeNativeTransfer(gt *testing.T) {
	t := devtest.SerialT(gt)
	sys := presets.NewMinimal(t)
	h := NewSgtHelper(t, sys)

	sender := sys.FunderL2.NewFundedEOA(eth.Ether(1))
	recipient := sys.Wallet.NewEOA(sys.L2EL)

	amount := eth.OneHundredthEther
	beforeR := recipient.GetBalance()
	beforeS := sender.GetBalance()

	sender.Transfer(recipient.Address(), amount)

	// Recipient should have received exactly the amount
	recipient.WaitForBalance(beforeR.Add(amount))
	afterR := recipient.GetBalance()
	t.Require().Equal(beforeR.Add(amount), afterR, "recipient balance mismatch")

	// Sender should have lost at least the amount (plus gas)
	afterS := sender.GetBalance()
	t.Require().True(beforeS.Sub(afterS).Gt(amount),
		"sender must have paid more than the transfer amount (gas was charged)")

	// SGT balance of sender should be 0 since we didn't deposit SGT
	sgtBal := h.GetSGTBalance(sender.Address())
	t.Require().Equal(0, sgtBal.Sign(), "sender SGT balance should be zero without deposit")
}

// TestSGT_SmokeBalanceQuery verifies that SGT balance can be queried for fresh accounts.
func TestSGT_SmokeBalanceQuery(gt *testing.T) {
	t := devtest.SerialT(gt)
	sys := presets.NewMinimal(t)
	h := NewSgtHelper(t, sys)

	// A fresh EOA should have zero SGT balance
	fresh := sys.Wallet.NewEOA(sys.L2EL)
	bal := h.GetSGTBalance(fresh.Address())
	t.Require().Equal(0, bal.Sign(), "fresh account should have zero SGT")

	// After depositing, balance should be non-zero
	funder := sys.FunderL2.NewFundedEOA(eth.Ether(1))
	depositAmount := big.NewInt(50000)
	h.DepositSGT(funder, fresh.Address(), depositAmount)

	bal = h.GetSGTBalance(fresh.Address())
	t.Require().Equal(0, depositAmount.Cmp(bal), "SGT balance should match deposit")
}
