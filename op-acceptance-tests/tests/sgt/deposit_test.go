package sgt

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-devstack/presets"
)

// TestSGT_DepositFunctionality tests SGT deposit scenarios
func TestSGT_DepositFunctionality(gt *testing.T) {
	t := devtest.SerialT(gt)
	sys := presets.NewMinimal(t)
	ctx := context.Background()
	sgt := NewSgtHelper(t, ctx, sys)

	t.Run("BasicDeposit", func(t devtest.T) {
		testBasicDeposit(t, sgt)
	})

	t.Run("MultipleDepositsAccumulation", func(t devtest.T) {
		testMultipleDepositsAccumulation(t, sgt)
	})

	t.Run("LargeAmountDeposit", func(t devtest.T) {
		testLargeAmountDeposit(t, sgt)
	})

	t.Run("MultiAccountIndependence", func(t devtest.T) {
		testMultiAccountIndependence(t, sgt)
	})
}

// testBasicDeposit verifies basic SGT deposit functionality
func testBasicDeposit(t devtest.T, sgt *SgtHelper) {
	// Create account with only native balance (no SGT)
	sgtAmount := big.NewInt(0)
	nativeAmount := new(big.Int).Mul(big.NewInt(10), big.NewInt(1e18)) // 10 ETH
	privKey, addr := sgt.CreateTestAccount(sgtAmount, nativeAmount)

	// Verify initial balances
	initialNative := sgt.GetNativeBalance(addr)
	initialSGT := sgt.GetSGTBalance(addr)
	t.Require().Equal(nativeAmount.Int64(), initialNative.Int64(), "Initial native balance mismatch")
	t.Require().Equal(int64(0), initialSGT.Int64(), "Initial SGT should be 0")

	// Deposit 0.5 ETH worth of SGT
	depositAmount := new(big.Int).Mul(big.NewInt(5), big.NewInt(1e17)) // 0.5 ETH
	err := sgt.DepositSGT(privKey, depositAmount)
	t.Require().NoError(err, "Deposit should succeed")

	// Verify balances after deposit
	finalNative := sgt.GetNativeBalance(addr)
	finalSGT := sgt.GetSGTBalance(addr)

	// In native-backed mode:
	// - Native balance should decrease by depositAmount (+ gas)
	// - SGT balance should equal depositAmount (first deposit, no SGT used for gas)
	t.Require().Greater(initialNative.Int64(), finalNative.Int64(), "Native should decrease")

	// Log detailed balance information
	t.Logger().Info("BasicDeposit balance details",
		"address", addr.Hex(),
		"initial_native", initialNative,
		"final_native", finalNative,
		"native_decrease", new(big.Int).Sub(initialNative, finalNative),
		"initial_sgt", initialSGT,
		"final_sgt", finalSGT,
		"deposit_amount", depositAmount,
		"sgt_difference", new(big.Int).Sub(finalSGT, depositAmount),
	)

	t.Require().Equal(depositAmount.Int64(), finalSGT.Int64(), "SGT should equal deposit amount")

	t.Logger().Info("Basic deposit test passed",
		"address", addr.Hex(),
		"deposit_amount", depositAmount,
		"final_sgt", finalSGT,
	)
}

// testMultipleDepositsAccumulation verifies SGT deposits accumulate correctly
func testMultipleDepositsAccumulation(t devtest.T, sgt *SgtHelper) {
	// Create account with sufficient native for multiple deposits
	sgtAmount := big.NewInt(0)
	nativeAmount := new(big.Int).Mul(big.NewInt(10), big.NewInt(1e18)) // 10 ETH
	privKey, addr := sgt.CreateTestAccount(sgtAmount, nativeAmount)

	// First deposit: 1 ETH
	firstDeposit := new(big.Int).Mul(big.NewInt(1), big.NewInt(1e18))
	err := sgt.DepositSGT(privKey, firstDeposit)
	t.Require().NoError(err, "First deposit should succeed")

	balanceAfterFirst := sgt.GetSGTBalance(addr)
	t.Require().Equal(firstDeposit.Int64(), balanceAfterFirst.Int64(), "SGT after first deposit mismatch")

	// Second deposit: 2 ETH
	// Note: This deposit will use SGT from first deposit to pay gas
	secondDeposit := new(big.Int).Mul(big.NewInt(2), big.NewInt(1e18))
	err = sgt.DepositSGT(privKey, secondDeposit)
	t.Require().NoError(err, "Second deposit should succeed")

	balanceAfterSecond := sgt.GetSGTBalance(addr)
	expectedTotal := new(big.Int).Add(firstDeposit, secondDeposit)

	// Allow tolerance for gas used from SGT (about 0.0002-0.0003 ETH per tx)
	tolerance := new(big.Int).Mul(big.NewInt(5), big.NewInt(1e14)) // 0.0005 ETH
	diff := new(big.Int).Sub(expectedTotal, balanceAfterSecond)
	if diff.Sign() < 0 {
		diff.Neg(diff)
	}
	t.Require().True(diff.Cmp(tolerance) <= 0,
		"SGT accumulation check: expected=%s, actual=%s, diff=%s",
		expectedTotal, balanceAfterSecond, diff)

	// Third deposit: 0.5 ETH
	thirdDeposit := new(big.Int).Mul(big.NewInt(5), big.NewInt(1e17))
	err = sgt.DepositSGT(privKey, thirdDeposit)
	t.Require().NoError(err, "Third deposit should succeed")

	finalSGT := sgt.GetSGTBalance(addr)
	expectedFinal := new(big.Int).Add(expectedTotal, thirdDeposit)

	// Allow tolerance for accumulated gas from multiple transactions
	tolerance2 := new(big.Int).Mul(big.NewInt(1), big.NewInt(1e15)) // 0.001 ETH
	diff2 := new(big.Int).Sub(expectedFinal, finalSGT)
	if diff2.Sign() < 0 {
		diff2.Neg(diff2)
	}
	t.Require().True(diff2.Cmp(tolerance2) <= 0,
		"Final SGT check: expected=%s, actual=%s, diff=%s",
		expectedFinal, finalSGT, diff2)

	t.Logger().Info("Multiple deposits accumulation test passed",
		"address", addr.Hex(),
		"deposit_1", firstDeposit,
		"deposit_2", secondDeposit,
		"deposit_3", thirdDeposit,
		"final_sgt", finalSGT,
	)
}

// testLargeAmountDeposit verifies deposit works with large amounts
func testLargeAmountDeposit(t devtest.T, sgt *SgtHelper) {
	// Create account with large native balance
	sgtAmount := big.NewInt(0)
	nativeAmount := new(big.Int).Mul(big.NewInt(1000), big.NewInt(1e18)) // 1000 ETH
	privKey, addr := sgt.CreateTestAccount(sgtAmount, nativeAmount)

	// Deposit 500 ETH worth of SGT
	depositAmount := new(big.Int).Mul(big.NewInt(500), big.NewInt(1e18))
	err := sgt.DepositSGT(privKey, depositAmount)
	t.Require().NoError(err, "Large deposit should succeed")

	finalSGT := sgt.GetSGTBalance(addr)
	t.Require().Equal(depositAmount.Int64(), finalSGT.Int64(), "SGT should equal large deposit amount")

	t.Logger().Info("Large amount deposit test passed",
		"address", addr.Hex(),
		"deposit_amount", depositAmount,
		"final_sgt", finalSGT,
	)
}

// testMultiAccountIndependence verifies deposits to different accounts are independent
func testMultiAccountIndependence(t devtest.T, sgt *SgtHelper) {
	// Create three accounts
	nativeAmount := new(big.Int).Mul(big.NewInt(5), big.NewInt(1e18)) // 5 ETH each

	privKey1, addr1 := sgt.CreateTestAccount(big.NewInt(0), nativeAmount)
	privKey2, addr2 := sgt.CreateTestAccount(big.NewInt(0), nativeAmount)
	privKey3, addr3 := sgt.CreateTestAccount(big.NewInt(0), nativeAmount)

	// Deposit different amounts to each account
	deposit1 := new(big.Int).Mul(big.NewInt(1), big.NewInt(1e18)) // 1 ETH
	deposit2 := new(big.Int).Mul(big.NewInt(2), big.NewInt(1e18)) // 2 ETH
	deposit3 := new(big.Int).Mul(big.NewInt(3), big.NewInt(1e18)) // 3 ETH

	err := sgt.DepositSGT(privKey1, deposit1)
	t.Require().NoError(err, "Account 1 deposit should succeed")

	err = sgt.DepositSGT(privKey2, deposit2)
	t.Require().NoError(err, "Account 2 deposit should succeed")

	err = sgt.DepositSGT(privKey3, deposit3)
	t.Require().NoError(err, "Account 3 deposit should succeed")

	// Verify each account has independent balance
	balance1 := sgt.GetSGTBalance(addr1)
	balance2 := sgt.GetSGTBalance(addr2)
	balance3 := sgt.GetSGTBalance(addr3)

	t.Require().Equal(deposit1.Int64(), balance1.Int64(), "Account 1 SGT mismatch")
	t.Require().Equal(deposit2.Int64(), balance2.Int64(), "Account 2 SGT mismatch")
	t.Require().Equal(deposit3.Int64(), balance3.Int64(), "Account 3 SGT mismatch")

	// Additional deposit to account 1 should not affect others
	additionalDeposit := new(big.Int).Mul(big.NewInt(5), big.NewInt(1e17)) // 0.5 ETH
	err = sgt.DepositSGT(privKey1, additionalDeposit)
	t.Require().NoError(err, "Additional deposit to account 1 should succeed")

	balance1After := sgt.GetSGTBalance(addr1)
	balance2After := sgt.GetSGTBalance(addr2)
	balance3After := sgt.GetSGTBalance(addr3)

	expectedBalance1 := new(big.Int).Add(deposit1, additionalDeposit)

	// Allow tolerance for gas paid with SGT (about 0.0002-0.0003 ETH per tx)
	// Account 1 made 2 deposits, so used SGT for gas on the 2nd deposit
	tolerance := new(big.Int).Mul(big.NewInt(5), big.NewInt(1e14)) // 0.0005 ETH
	diff1 := new(big.Int).Sub(expectedBalance1, balance1After)
	if diff1.Sign() < 0 {
		diff1.Neg(diff1)
	}
	t.Require().True(diff1.Cmp(tolerance) <= 0,
		"Account 1 balance check: expected=%s, actual=%s, diff=%s",
		expectedBalance1, balance1After, diff1)

	t.Require().Equal(balance2.Int64(), balance2After.Int64(), "Account 2 should be unchanged")
	t.Require().Equal(balance3.Int64(), balance3After.Int64(), "Account 3 should be unchanged")

	t.Logger().Info("Multi-account independence test passed",
		"account_1", addr1.Hex(), "balance_1", balance1After,
		"account_2", addr2.Hex(), "balance_2", balance2After,
		"account_3", addr3.Hex(), "balance_3", balance3After,
	)
}
