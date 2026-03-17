package sgt

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-devstack/presets"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

var (
	dummyAddr = common.HexToAddress("0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF")
)

// TestSGT_GasPaymentScenarios tests comprehensive SGT gas payment behaviors
// This test validates that SGT is used for gas payment with correct priority:
// - Deduction: SGT first → Native second
// - Refund: Native first → SGT second
func TestSGT_GasPaymentScenarios(gt *testing.T) {
	t := devtest.SerialT(gt)
	sys := presets.NewMinimal(t)
	ctx := context.Background()

	sgt := NewSgtHelper(t, ctx, sys)

	tests := []struct {
		name   string
		action func(t devtest.T, sgt *SgtHelper)
	}{
		{
			name:   "NativeGasPaymentWithoutSGT",
			action: testNativeGasPaymentWithoutSGT,
		},
		{
			name:   "FullSGTGasPaymentWithoutNativeBalance",
			action: testFullSGTGasPaymentWithoutNativeBalance,
		},
		{
			name:   "FullSGTGasPaymentWithNativeBalance",
			action: testFullSGTGasPaymentWithNativeBalance,
		},
		{
			name:   "PartialSGTGasPayment",
			action: testPartialSGTGasPayment,
		},
		{
			name:   "FullSGTGasPaymentWithNonZeroTxValue",
			action: testFullSGTGasPaymentWithNonZeroTxValue,
		},
		{
			name:   "PartialSGTGasPaymentWithNonZeroTxValue",
			action: testPartialSGTGasPaymentWithNonZeroTxValue,
		},
		{
			name:   "InsufficientSGTForGas",
			action: testInsufficientSGTForGas,
		},
		{
			name:   "InsufficientNativeForGas",
			action: testInsufficientNativeForGas,
		},
		{
			name:   "InsufficientPartialSGTForGas",
			action: testInsufficientPartialSGTForGas,
		},
		{
			name:   "InsufficientNativeForTxValue",
			action: testInsufficientNativeForTxValue,
		},
		{
			name:   "InsufficientPartialSGTForTxValue",
			action: testInsufficientPartialSGTForTxValue,
		},
	}

	for _, tc := range tests {
		tc := tc // capture range variable
		t.Run(tc.name, func(subT devtest.T) {
			tc.action(subT, sgt)
		})
	}
}

// testNativeGasPaymentWithoutSGT verifies gas is paid using native balance when no SGT
func testNativeGasPaymentWithoutSGT(t devtest.T, sgt *SgtHelper) {
	// Setup: Account with only native balance, no SGT
	// Note: Use 10 ETH to account for L1 data fees in Optimism
	nativeAmount := new(big.Int).Mul(big.NewInt(10), big.NewInt(1e18)) // 10 ETH
	privKey, addr := sgt.CreateTestAccount(nil, nativeAmount)

	initialNative := sgt.GetNativeBalance(addr)
	initialSGT := sgt.GetSGTBalance(addr)
	t.Require().Equal( int64(0), initialSGT.Int64(), "should have no SGT")
	t.Require().Equal( nativeAmount.Int64(), initialNative.Int64())

	// Execute: Send transaction
	receipt, err := sgt.SendTransaction(privKey, dummyAddr, big.NewInt(0))
	t.Require().NoError( err)
	t.Require().Equal( types.ReceiptStatusSuccessful, receipt.Status)

	// Verify: Native balance decreased by gas cost, SGT unchanged
	finalNative := sgt.GetNativeBalance(addr)
	finalSGT := sgt.GetSGTBalance(addr)

	// Calculate total gas cost (L2 execution + L1 data fee)
	gasCost := new(big.Int).Mul(big.NewInt(int64(receipt.GasUsed)), receipt.EffectiveGasPrice)
	if receipt.L1Fee != nil {
		gasCost.Add(gasCost, receipt.L1Fee)
	}
	expectedNative := new(big.Int).Sub(initialNative, gasCost)

	t.Require().Equal( int64(0), finalSGT.Int64(), "SGT should remain zero")
	t.Require().Equal( expectedNative.Int64(), finalNative.Int64(), "native should decrease by gas cost (L2 + L1)")
}

// testFullSGTGasPaymentWithoutNativeBalance verifies gas is paid entirely using SGT when minimal native balance
//
// NOTE: With op-reth, transaction pool validation requires some native balance to accept transactions.
// We provide minimal native balance (0.001 ETH) for pool acceptance, but verify SGT is used for gas payment.
func testFullSGTGasPaymentWithoutNativeBalance(t devtest.T, sgt *SgtHelper) {
	// Setup: Account with SGT and minimal native for pool acceptance
	// Note: Use 10 SGT to account for L1 data fees
	sgtAmount := new(big.Int).Mul(big.NewInt(10), big.NewInt(1e18))      // 10 SGT
	minNative := new(big.Int).Mul(big.NewInt(1), big.NewInt(1e15))        // 0.001 ETH for pool
	privKey, addr := sgt.CreateTestAccount(sgtAmount, minNative)

	initialNative := sgt.GetNativeBalance(addr)
	initialSGT := sgt.GetSGTBalance(addr)
	t.Require().Equal( minNative.Int64(), initialNative.Int64(), "should have minimal native balance")
	t.Require().Equal( sgtAmount.Int64(), initialSGT.Int64())

	// Execute: Send transaction
	receipt, err := sgt.SendTransaction(privKey, dummyAddr, big.NewInt(0))
	t.Require().NoError( err)
	t.Require().Equal( types.ReceiptStatusSuccessful, receipt.Status)

	// Verify: SGT decreased by gas cost, native unchanged
	finalNative := sgt.GetNativeBalance(addr)
	finalSGT := sgt.GetSGTBalance(addr)

	// Calculate total gas cost (L2 execution + L1 data fee)
	gasCost := new(big.Int).Mul(big.NewInt(int64(receipt.GasUsed)), receipt.EffectiveGasPrice)
	if receipt.L1Fee != nil {
		gasCost.Add(gasCost, receipt.L1Fee)
	}
	expectedSGT := new(big.Int).Sub(initialSGT, gasCost)

	// Native should remain at minNative (0.001 ETH) since gas was paid from SGT
	t.Require().Equal( minNative.Int64(), finalNative.Int64(), "native should remain at minimum (0.001 ETH)")
	t.Require().Equal( expectedSGT.Int64(), finalSGT.Int64(), "SGT should decrease by gas cost (L2 + L1)")
}

// testFullSGTGasPaymentWithNativeBalance verifies SGT is prioritized over native for gas payment
func testFullSGTGasPaymentWithNativeBalance(t devtest.T, sgt *SgtHelper) {
	// Setup: Account with both SGT and native (SGT sufficient for gas)
	// Note: Use 10 SGT and 5 ETH to account for L1 data fees
	sgtAmount := new(big.Int).Mul(big.NewInt(10), big.NewInt(1e18))   // 10 SGT
	nativeAmount := new(big.Int).Mul(big.NewInt(5), big.NewInt(1e18)) // 5 ETH
	privKey, addr := sgt.CreateTestAccount(sgtAmount, nativeAmount)

	initialNative := sgt.GetNativeBalance(addr)
	initialSGT := sgt.GetSGTBalance(addr)

	// Execute: Send transaction
	receipt, err := sgt.SendTransaction(privKey, dummyAddr, big.NewInt(0))
	t.Require().NoError( err)
	t.Require().Equal( types.ReceiptStatusSuccessful, receipt.Status)

	// Verify: Only SGT decreased (priority), native unchanged
	finalNative := sgt.GetNativeBalance(addr)
	finalSGT := sgt.GetSGTBalance(addr)

	// Calculate total gas cost (L2 execution + L1 data fee)
	gasCost := new(big.Int).Mul(big.NewInt(int64(receipt.GasUsed)), receipt.EffectiveGasPrice)
	if receipt.L1Fee != nil {
		gasCost.Add(gasCost, receipt.L1Fee)
	}
	expectedSGT := new(big.Int).Sub(initialSGT, gasCost)

	t.Require().Equal( initialNative.Int64(), finalNative.Int64(), "native should not change (SGT priority)")
	t.Require().Equal( expectedSGT.Int64(), finalSGT.Int64(), "SGT should decrease by gas cost (L2 + L1)")
}

// testPartialSGTGasPayment verifies gas is paid using both SGT and native when SGT insufficient
func testPartialSGTGasPayment(t devtest.T, sgt *SgtHelper) {
	// Setup: Account with small SGT (insufficient for full gas) + sufficient native
	sgtAmount := big.NewInt(1e13)   // 10 Gwei (small, insufficient for ~21 Gwei gas cost)
	// Note: Use 10 ETH to account for L1 data fees
	nativeAmount := new(big.Int).Mul(big.NewInt(10), big.NewInt(1e18)) // 10 ETH (large)
	privKey, addr := sgt.CreateTestAccount(sgtAmount, nativeAmount)

	initialNative := sgt.GetNativeBalance(addr)
	initialSGT := sgt.GetSGTBalance(addr)

	// Execute: Send transaction
	receipt, err := sgt.SendTransaction(privKey, dummyAddr, big.NewInt(0))
	t.Require().NoError( err)
	t.Require().Equal( types.ReceiptStatusSuccessful, receipt.Status)

	// Verify: SGT fully depleted, native decreased for remainder
	finalNative := sgt.GetNativeBalance(addr)
	finalSGT := sgt.GetSGTBalance(addr)

	// Calculate total gas cost (L2 execution + L1 data fee)
	gasCost := new(big.Int).Mul(big.NewInt(int64(receipt.GasUsed)), receipt.EffectiveGasPrice)
	if receipt.L1Fee != nil {
		gasCost.Add(gasCost, receipt.L1Fee)
	}
	nativeCost := new(big.Int).Sub(gasCost, initialSGT) // Remainder after SGT depleted
	expectedNative := new(big.Int).Sub(initialNative, nativeCost)

	t.Require().Equal( int64(0), finalSGT.Int64(), "SGT should be fully depleted")
	t.Require().Equal( expectedNative.Int64(), finalNative.Int64(), "native should cover remainder")
}

// testFullSGTGasPaymentWithNonZeroTxValue verifies SGT used for gas, native for tx.value
func testFullSGTGasPaymentWithNonZeroTxValue(t devtest.T, sgt *SgtHelper) {
	// Setup: Account with sufficient SGT for gas + native for tx.value
	// Note: Use 10 SGT and 5 ETH to account for L1 data fees
	sgtAmount := new(big.Int).Mul(big.NewInt(10), big.NewInt(1e18))   // 10 SGT (for gas)
	nativeAmount := new(big.Int).Mul(big.NewInt(5), big.NewInt(1e18)) // 5 ETH (for tx.value)
	txValue := big.NewInt(1e17)      // 0.1 ETH to send
	privKey, addr := sgt.CreateTestAccount(sgtAmount, nativeAmount)

	initialNative := sgt.GetNativeBalance(addr)
	initialSGT := sgt.GetSGTBalance(addr)

	// Execute: Send transaction with value
	receipt, err := sgt.SendTransaction(privKey, dummyAddr, txValue)
	t.Require().NoError( err)
	t.Require().Equal( types.ReceiptStatusSuccessful, receipt.Status)

	// Verify: SGT decreased by gas cost, native decreased by tx.value only
	finalNative := sgt.GetNativeBalance(addr)
	finalSGT := sgt.GetSGTBalance(addr)

	// Calculate total gas cost (L2 execution + L1 data fee)
	gasCost := new(big.Int).Mul(big.NewInt(int64(receipt.GasUsed)), receipt.EffectiveGasPrice)
	if receipt.L1Fee != nil {
		gasCost.Add(gasCost, receipt.L1Fee)
	}
	expectedSGT := new(big.Int).Sub(initialSGT, gasCost)
	expectedNative := new(big.Int).Sub(initialNative, txValue)

	t.Require().Equal( expectedSGT.Int64(), finalSGT.Int64(), "SGT should pay gas")
	t.Require().Equal( expectedNative.Int64(), finalNative.Int64(), "native should pay tx.value only")
}

// testPartialSGTGasPaymentWithNonZeroTxValue verifies partial SGT + native for gas, native for tx.value
func testPartialSGTGasPaymentWithNonZeroTxValue(t devtest.T, sgt *SgtHelper) {
	// Setup: Small SGT + sufficient native for both gas remainder and tx.value
	sgtAmount := big.NewInt(1e13)   // 10 Gwei (small, insufficient for ~21 Gwei gas cost)
	// Note: Use 10 ETH to account for L1 data fees
	nativeAmount := new(big.Int).Mul(big.NewInt(10), big.NewInt(1e18)) // 10 ETH (for gas remainder + tx.value)
	txValue := big.NewInt(1e17)     // 0.1 ETH to send
	privKey, addr := sgt.CreateTestAccount(sgtAmount, nativeAmount)

	initialNative := sgt.GetNativeBalance(addr)
	initialSGT := sgt.GetSGTBalance(addr)

	// Execute: Send transaction with value
	receipt, err := sgt.SendTransaction(privKey, dummyAddr, txValue)
	t.Require().NoError( err)
	t.Require().Equal( types.ReceiptStatusSuccessful, receipt.Status)

	// Verify: SGT fully depleted, native covers gas remainder + tx.value
	finalNative := sgt.GetNativeBalance(addr)
	finalSGT := sgt.GetSGTBalance(addr)

	// Calculate total gas cost (L2 execution + L1 data fee)
	gasCost := new(big.Int).Mul(big.NewInt(int64(receipt.GasUsed)), receipt.EffectiveGasPrice)
	if receipt.L1Fee != nil {
		gasCost.Add(gasCost, receipt.L1Fee)
	}
	gasRemainder := new(big.Int).Sub(gasCost, initialSGT)
	totalNativeSpent := new(big.Int).Add(gasRemainder, txValue)
	expectedNative := new(big.Int).Sub(initialNative, totalNativeSpent)

	t.Require().Equal( int64(0), finalSGT.Int64(), "SGT should be fully depleted")
	t.Require().Equal( expectedNative.Int64(), finalNative.Int64(), "native should cover gas remainder + tx.value")
}

// testInsufficientSGTForGas verifies transaction fails when total (SGT + native) insufficient for gas
func testInsufficientSGTForGas(t devtest.T, sgt *SgtHelper) {
	// Setup: Only small SGT, no native (insufficient for gas)
	sgtAmount := big.NewInt(100) // Tiny amount, won't cover gas
	privKey, addr := sgt.CreateTestAccount(sgtAmount, nil)

	t.Require().Equal( sgtAmount.Int64(), sgt.GetSGTBalance(addr).Int64())
	t.Require().Equal( int64(0), sgt.GetNativeBalance(addr).Int64())

	// Execute: Transaction should fail
	_, err := sgt.SendTransaction(privKey, dummyAddr, big.NewInt(0))
	t.Require().Error( err, "transaction should fail due to insufficient funds")
}

// testInsufficientNativeForGas verifies transaction fails when only native insufficient for gas
func testInsufficientNativeForGas(t devtest.T, sgt *SgtHelper) {
	// Setup: Only tiny native balance, no SGT
	nativeAmount := big.NewInt(100) // Tiny, won't cover gas
	privKey, addr := sgt.CreateTestAccount(nil, nativeAmount)

	t.Require().Equal( int64(0), sgt.GetSGTBalance(addr).Int64())
	t.Require().Equal( nativeAmount.Int64(), sgt.GetNativeBalance(addr).Int64())

	// Execute: Transaction should fail
	_, err := sgt.SendTransaction(privKey, dummyAddr, big.NewInt(0))
	t.Require().Error( err, "transaction should fail due to insufficient funds")
}

// testInsufficientPartialSGTForGas verifies transaction fails when total insufficient despite partial SGT
func testInsufficientPartialSGTForGas(t devtest.T, sgt *SgtHelper) {
	// Setup: Small SGT + small native (total insufficient for gas)
	sgtAmount := big.NewInt(500)
	nativeAmount := big.NewInt(500)
	privKey, addr := sgt.CreateTestAccount(sgtAmount, nativeAmount)

	_ = addr  // Unused in this test
	// Execute: Transaction should fail
	_, err := sgt.SendTransaction(privKey, dummyAddr, big.NewInt(0))
	t.Require().Error( err, "transaction should fail due to insufficient total funds")
}

// testInsufficientNativeForTxValue verifies transaction fails when native insufficient for tx.value
func testInsufficientNativeForTxValue(t devtest.T, sgt *SgtHelper) {
	// Setup: Sufficient SGT for gas, but insufficient native for tx.value
	// Note: Use 10 SGT to account for L1 data fees
	sgtAmount := new(big.Int).Mul(big.NewInt(10), big.NewInt(1e18))  // 10 SGT (sufficient for gas)
	nativeAmount := big.NewInt(100) // Tiny native (insufficient for tx.value)
	txValue := big.NewInt(1e17)    // 0.1 ETH to send (> native)
	privKey, addr := sgt.CreateTestAccount(sgtAmount, nativeAmount)

	_ = addr  // Unused in this test
	// Execute: Transaction should fail (tx.value requires native)
	_, err := sgt.SendTransaction(privKey, dummyAddr, txValue)
	t.Require().Error( err, "transaction should fail due to insufficient native for tx.value")
}

// testInsufficientPartialSGTForTxValue verifies transaction fails when total insufficient for gas + tx.value
func testInsufficientPartialSGTForTxValue(t devtest.T, sgt *SgtHelper) {
	// Setup: Small SGT + small native (total insufficient for gas + tx.value)
	sgtAmount := big.NewInt(1e15)   // 0.001 SGT
	nativeAmount := big.NewInt(1e15) // 0.001 ETH
	txValue := big.NewInt(1e17)     // 0.1 ETH to send (much larger)
	privKey, addr := sgt.CreateTestAccount(sgtAmount, nativeAmount)

	_ = addr  // Unused in this test
	// Execute: Transaction should fail
	_, err := sgt.SendTransaction(privKey, dummyAddr, txValue)
	t.Require().Error( err, "transaction should fail due to insufficient funds for gas + tx.value")
}
