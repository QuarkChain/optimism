package sgt

import (
	"context"
	"math/big"
	"time"

	"github.com/ethereum-optimism/optimism/op-core/predeploys"
	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-devstack/dsl"
	"github.com/ethereum-optimism/optimism/op-devstack/presets"
	"github.com/ethereum-optimism/optimism/op-service/txplan"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/lmittmann/w3"
)

var (
	sgtAddr   = predeploys.SoulGasTokenAddr
	seqVault  = predeploys.SequencerFeeVaultAddr
	baseVault = predeploys.BaseFeeVaultAddr
	l1Vault   = predeploys.L1FeeVaultAddr
	dummyAddr = common.Address{0xff, 0xff}

	balanceOfFunc        = w3.MustNewFunc("balanceOf(address)", "uint256")
	batchDepositSelector = common.FromHex("0x84e08810") // batchDepositForAll(address[],uint256)
)

// SgtHelper provides utilities for SGT acceptance tests.
type SgtHelper struct {
	t   devtest.T
	sys *presets.Minimal
}

// NewSgtHelper creates a new SGT test helper.
func NewSgtHelper(t devtest.T, sys *presets.Minimal) *SgtHelper {
	return &SgtHelper{
		t:   t,
		sys: sys,
	}
}

// GetSGTBalance returns the SGT balance of the given address.
func (h *SgtHelper) GetSGTBalance(addr common.Address) *big.Int {
	l2Client := h.sys.L2EL.Escape().L2EthClient()

	ctx, cancel := context.WithTimeout(h.t.Ctx(), 20*time.Second)
	defer cancel()

	data, err := balanceOfFunc.EncodeArgs(addr)
	h.t.Require().NoError(err, "failed to encode balanceOf args")

	out, err := l2Client.Call(ctx, ethereum.CallMsg{To: &sgtAddr, Data: data}, rpc.LatestBlockNumber)
	h.t.Require().NoError(err, "failed to call balanceOf on SGT contract")

	var balance *big.Int
	err = balanceOfFunc.DecodeReturns(out, &balance)
	h.t.Require().NoError(err, "failed to decode balanceOf return")

	return balance
}

// GetNativeBalance returns the native (ETH) balance of the given address.
func (h *SgtHelper) GetNativeBalance(addr common.Address) *big.Int {
	l2Client := h.sys.L2EL.Escape().L2EthClient()

	ctx, cancel := context.WithTimeout(h.t.Ctx(), 20*time.Second)
	defer cancel()

	balance, err := l2Client.BalanceAt(ctx, addr, nil)
	h.t.Require().NoError(err, "failed to get native balance")

	return balance
}

// DepositSGT deposits SGT to the target address using batchDepositForAll via a funder EOA.
// The funder must have enough native balance to cover msg.value (= sgtAmount).
func (h *SgtHelper) DepositSGT(funder *dsl.EOA, target common.Address, sgtAmount *big.Int) {
	calldata := encodeBatchDepositForAll(target, sgtAmount)
	funder.Transact(
		funder.Plan(),
		txplan.WithTo(&sgtAddr),
		txplan.WithEth(sgtAmount),
		txplan.WithData(calldata),
		txplan.WithGasLimit(200000),
	)
}

// encodeBatchDepositForAll ABI-encodes batchDepositForAll(address[], uint256).
// Layout: selector(4) + offset(32) + value(32) + array_length(32) + address(32)
func encodeBatchDepositForAll(target common.Address, value *big.Int) []byte {
	data := make([]byte, 0, 4+32*4)
	data = append(data, batchDepositSelector...)

	// offset to dynamic array: 0x40 (64 bytes, after the value param)
	offset := common.LeftPadBytes(big.NewInt(64).Bytes(), 32)
	data = append(data, offset...)

	// value (uint256)
	valBytes := common.LeftPadBytes(value.Bytes(), 32)
	data = append(data, valBytes...)

	// array length = 1
	lenBytes := common.LeftPadBytes(big.NewInt(1).Bytes(), 32)
	data = append(data, lenBytes...)

	// address element (left-padded to 32 bytes)
	addrBytes := common.LeftPadBytes(target.Bytes(), 32)
	data = append(data, addrBytes...)

	return data
}

// CalcVaultBalance returns the sum of fee vault balances (sequencer + base + L1 fee vaults).
func (h *SgtHelper) CalcVaultBalance() *big.Int {
	seqBal := h.GetNativeBalance(seqVault)
	baseBal := h.GetNativeBalance(baseVault)
	l1Bal := h.GetNativeBalance(l1Vault)
	total := new(big.Int).Add(seqBal, baseBal)
	return total.Add(total, l1Bal)
}

// CalcGasFee computes the total gas cost from a receipt: effectiveGasPrice * gasUsed + L1Fee.
func CalcGasFee(receipt *types.Receipt) *big.Int {
	fees := new(big.Int).Mul(receipt.EffectiveGasPrice, big.NewInt(int64(receipt.GasUsed)))
	fees = fees.Add(fees, receipt.L1Fee)
	return fees
}

// InvariantBalanceCheck verifies: preSGT + preNative = postSGT + postNative + gasCost + txValue
func (h *SgtHelper) InvariantBalanceCheck(
	addr common.Address,
	gasCost *big.Int,
	txValue *big.Int,
	preSGT *big.Int,
	preNative *big.Int,
	postSGT *big.Int,
	sgtShouldChange bool,
) {
	if sgtShouldChange {
		h.t.Require().NotEqual(0, preSGT.Cmp(postSGT), "SGT balance should have changed")
	} else {
		h.t.Require().Equal(0, preSGT.Cmp(postSGT), "SGT balance should not have changed")
	}

	postNative := h.GetNativeBalance(addr)
	preBal := new(big.Int).Add(preSGT, preNative)
	postBal := new(big.Int).Add(postSGT, postNative)
	postBal = postBal.Add(postBal, gasCost)
	postBal = postBal.Add(postBal, txValue)

	h.t.Require().Equal(0, preBal.Cmp(postBal),
		"balance invariant violated: pre(%s) != post(%s) + gas(%s) + value(%s)",
		preBal, postBal, gasCost, txValue)
}
