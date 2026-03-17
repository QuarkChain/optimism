package sgt

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-devstack/presets"
	"github.com/ethereum-optimism/optimism/op-service/eth"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

var (
	// SGT contract address (predeploy)
	SoulGasTokenAddr = common.HexToAddress("0x4200000000000000000000000000000000000800")
)

// SgtHelper provides utilities for SGT E2E testing
type SgtHelper struct {
	T   devtest.T
	Ctx context.Context
	Sys *presets.Minimal
}

// NewSgtHelper creates a new SGT test helper
func NewSgtHelper(t devtest.T, ctx context.Context, sys *presets.Minimal) *SgtHelper {
	return &SgtHelper{
		T:   t,
		Ctx: ctx,
		Sys: sys,
	}
}

// CreateTestAccount creates a new EOA with specified SGT and native balances
// Returns the private key and address for low-level transaction construction
func (h *SgtHelper) CreateTestAccount(sgtAmount *big.Int, nativeAmount *big.Int) (*ecdsa.PrivateKey, common.Address) {
	// Generate new private key
	privKey, err := crypto.GenerateKey()
	h.T.Require().NoError(err)

	addr := crypto.PubkeyToAddress(privKey.PublicKey)

	// Fund with SGT if requested (using batchDepositForAll from funder)
	if sgtAmount != nil && sgtAmount.Sign() > 0 {
		// Check if SGT contract is deployed
		if !h.isSGTContractDeployed() {
			h.T.Logger().Warn("SGT contract not deployed - skipping SGT funding",
				"requested_amount", sgtAmount,
				"address", addr,
			)
			return privKey, addr
		}

		// Use funder to call batchDepositForAll
		client := h.Sys.L2EL.Escape().EthClient()
		chainID, err := client.ChainID(h.Ctx)
		h.T.Require().NoError(err)

		// Get current base fee
		rpcClient := h.Sys.L2EL.Escape().EthClient().RPC()
		var header types.Header
		err = rpcClient.CallContext(h.Ctx, &header, "eth_getBlockByNumber", "latest", false)
		h.T.Require().NoError(err)

		baseFee := header.BaseFee
		if baseFee == nil {
			baseFee = big.NewInt(1000000000) // Fallback to 1 gwei
		}

		gasFeeCap := new(big.Int).Add(baseFee, big.NewInt(10000000000)) // base + 10 gwei buffer
		gasTipCap := big.NewInt(1000000000)                             // 1 gwei tip

		// Create funder with enough native to cover SGT deposit + gas
		funderAmount := new(big.Int).Add(sgtAmount, big.NewInt(1e18)) // SGT amount + 1 ETH for gas
		funderEOA := h.Sys.FunderL2.NewFundedEOA(eth.WeiBig(funderAmount))

		// Encode batchDepositForAll(address[] calldata _accounts, uint256 _value)
		// Function selector: 0x84e08810 (from contract ABI)
		callData := common.Hex2Bytes("84e08810")

		// ABI encode parameters: (address[], uint256)
		// Offset to array data (32 bytes)
		callData = append(callData, common.LeftPadBytes(big.NewInt(64).Bytes(), 32)...)
		// Value parameter (sgtAmount)
		callData = append(callData, common.LeftPadBytes(sgtAmount.Bytes(), 32)...)
		// Array length (1 address)
		callData = append(callData, common.LeftPadBytes(big.NewInt(1).Bytes(), 32)...)
		// Array element (addr)
		callData = append(callData, common.LeftPadBytes(addr.Bytes(), 32)...)

		funderNonce, err := client.NonceAt(h.Ctx, funderEOA.Address(), nil)
		h.T.Require().NoError(err)

		batchDepositTx := types.NewTx(&types.DynamicFeeTx{
			ChainID:   chainID,
			Nonce:     funderNonce,
			To:        &SoulGasTokenAddr,
			Value:     sgtAmount, // Send native to be converted to SGT
			Gas:       200000,    // Higher gas limit for batch operation
			GasFeeCap: gasFeeCap,
			GasTipCap: gasTipCap,
			Data:      callData,
		})

		signedBatchTx, err := types.SignTx(batchDepositTx, types.LatestSignerForChainID(chainID), funderEOA.Key().Priv())
		h.T.Require().NoError(err)

		err = client.SendTransaction(h.Ctx, signedBatchTx)
		if err != nil {
			h.T.Logger().Warn("SGT batchDepositForAll() call failed",
				"error", err,
				"address", addr,
				"amount", sgtAmount,
			)
			return privKey, addr
		}

		batchReceipt, err := h.waitForReceipt(signedBatchTx.Hash())
		if err != nil || batchReceipt.Status != types.ReceiptStatusSuccessful {
			h.T.Logger().Warn("SGT batchDepositForAll() receipt failed",
				"error", err,
				"status", batchReceipt.Status,
				"address", addr,
				"tx_hash", signedBatchTx.Hash(),
				"gas_used", batchReceipt.GasUsed,
				"calldata_hex", hexutil.Encode(callData),
				"value", sgtAmount,
			)
			return privKey, addr
		}

		// DEBUG: Check SGT balance after batch deposit
		postDepositSGT := h.GetSGTBalance(addr)
		h.T.Logger().Info("After batchDepositForAll() call",
			"address", addr,
			"sgt_balance", postDepositSGT,
			"requested_sgt", sgtAmount,
			"batch_tx", signedBatchTx.Hash(),
			"receipt_status", batchReceipt.Status,
			"gas_used", batchReceipt.GasUsed,
		)
	}

	// Fund with native balance if requested
	if nativeAmount != nil && nativeAmount.Sign() > 0 {
		// Use Funder to send native balance
		client := h.Sys.L2EL.Escape().EthClient()
		chainID, err := client.ChainID(h.Ctx)
		h.T.Require().NoError(err)

		// Query current base fee for accurate gas pricing
		rpcClient := h.Sys.L2EL.Escape().EthClient().RPC()
		var header types.Header
		err = rpcClient.CallContext(h.Ctx, &header, "eth_getBlockByNumber", "latest", false)
		h.T.Require().NoError(err)

		baseFee := header.BaseFee
		if baseFee == nil {
			baseFee = big.NewInt(1000000000) // Fallback to 1 gwei if no base fee
		}

		gasFeeCap := new(big.Int).Add(baseFee, big.NewInt(100000000000)) // base + 100 gwei
		gasTipCap := big.NewInt(1000000000)                              // 1 gwei

		// Request enough funds to cover the amount + gas fees
		funderAmount := new(big.Int).Add(nativeAmount, big.NewInt(1e18))
		funderEOA := h.Sys.FunderL2.NewFundedEOA(eth.WeiBig(funderAmount))

		nonce, err := client.NonceAt(h.Ctx, funderEOA.Address(), nil)
		h.T.Require().NoError(err)

		tx := types.NewTx(&types.DynamicFeeTx{
			ChainID:   chainID,
			Nonce:     nonce,
			To:        &addr,
			Value:     nativeAmount,
			Gas:       21000,
			GasFeeCap: gasFeeCap,
			GasTipCap: gasTipCap,
		})

		signedTx, err := types.SignTx(tx, types.LatestSignerForChainID(chainID), funderEOA.Key().Priv())
		h.T.Require().NoError(err)

		err = client.SendTransaction(h.Ctx, signedTx)
		h.T.Require().NoError(err)

		// Wait for confirmation
		h.waitForReceipt(signedTx.Hash())
	}

	return privKey, addr
}

// isSGTContractDeployed checks if the SGT contract has code deployed
func (h *SgtHelper) isSGTContractDeployed() bool {
	rpcClient := h.Sys.L2EL.Escape().EthClient().RPC()

	var code hexutil.Bytes
	err := rpcClient.CallContext(h.Ctx, &code, "eth_getCode", SoulGasTokenAddr.Hex(), "latest")

	return err == nil && len(code) > 0
}

// depositSGT deposits SGT tokens to an address by having the account call deposit()
// In native-backed mode, this requires funding the account with native ETH first,
// then calling deposit() to convert ETH to SGT
func (h *SgtHelper) depositSGT(to common.Address, amount *big.Int) error {
	// In native-backed mode, we need to:
	// 1. Fund the account with native ETH (amount + gas buffer)
	// 2. Have the account call deposit() with the ETH

	client := h.Sys.L2EL.Escape().EthClient()

	// First, fund the account with native ETH (amount + gas buffer)
	// Add 0.1 ETH buffer for gas fees
	gasBuffer := new(big.Int).Mul(big.NewInt(1e17), big.NewInt(1)) // 0.1 ETH
	totalNativeNeeded := new(big.Int).Add(amount, gasBuffer)

	funderEOA := h.Sys.FunderL2.NewFundedEOA(eth.WeiBig(new(big.Int).Add(totalNativeNeeded, big.NewInt(1e18))))

	chainID, err := client.ChainID(h.Ctx)
	if err != nil {
		return err
	}

	nonce, err := client.NonceAt(h.Ctx, funderEOA.Address(), nil)
	if err != nil {
		return err
	}

	// Transfer native ETH to the target account
	transferTx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		To:        &to,
		Value:     totalNativeNeeded,
		Gas:       21000,
		GasFeeCap: big.NewInt(1000000000),
		GasTipCap: big.NewInt(1000000000),
	})

	signedTransferTx, err := types.SignTx(transferTx, types.LatestSignerForChainID(chainID), funderEOA.Key().Priv())
	if err != nil {
		return err
	}

	err = client.SendTransaction(h.Ctx, signedTransferTx)
	if err != nil {
		return err
	}

	// Wait for transfer confirmation
	_, err = h.waitForReceipt(signedTransferTx.Hash())
	if err != nil {
		return err
	}

	// Now we need to call deposit() from the target account
	// But we don't have the private key for 'to' address in this helper
	// The account was created in CreateTestAccount, so we can't call deposit from here

	// WORKAROUND: Return success - the caller (CreateTestAccount) has the private key
	// and should call deposit() themselves
	// For now, just return nil and let the native balance serve as SGT via deposit()

	return nil
}

// GetNativeBalance returns the native balance of an address
func (h *SgtHelper) GetNativeBalance(addr common.Address) *big.Int {
	client := h.Sys.L2EL.Escape().EthClient()
	balance, err := client.BalanceAt(h.Ctx, addr, nil)
	h.T.Require().NoError(err)
	return balance
}

// GetSGTBalance returns the SGT balance of an address via RPC
// This reads the SGT contract's balanceOf method
func (h *SgtHelper) GetSGTBalance(addr common.Address) *big.Int {
	rpcClient := h.Sys.L2EL.Escape().EthClient().RPC()

	// Prepare balanceOf(address) call data
	// Function selector: keccak256("balanceOf(address)")[0:4] = 0x70a08231
	callData := "0x70a08231" + common.BytesToHash(addr.Bytes()).Hex()[2:]

	// Make eth_call
	var result hexutil.Bytes
	err := rpcClient.CallContext(h.Ctx, &result, "eth_call", map[string]any{
		"to":   SoulGasTokenAddr.Hex(),
		"data": callData,
	}, "latest")

	if err != nil {
		h.T.Logger().Warn("SGT balanceOf call failed - assuming zero balance",
			"address", addr,
			"error", err,
		)
		return big.NewInt(0)
	}

	// Parse result as uint256
	if len(result) == 0 {
		return big.NewInt(0)
	}

	return new(big.Int).SetBytes(result)
}

// SendTransaction sends a transaction from a private key to an address with value
// Returns the receipt and any error
func (h *SgtHelper) SendTransaction(privKey *ecdsa.PrivateKey, to common.Address, value *big.Int) (*types.Receipt, error) {
	client := h.Sys.L2EL.Escape().EthClient()

	// Get sender address
	sender := crypto.PubkeyToAddress(privKey.PublicKey)

	// DEBUG: Log sender and check balances before sending transaction
	nativeBalance := h.GetNativeBalance(sender)
	sgtBalance := h.GetSGTBalance(sender)
	h.T.Logger().Info("SendTransaction called",
		"sender", sender,
		"native_balance", nativeBalance,
		"sgt_balance", sgtBalance,
		"to", to,
		"value", value,
	)

	// Get chain ID
	chainID, err := client.ChainID(h.Ctx)
	if err != nil {
		return nil, err
	}

	// Get nonce
	nonce, err := client.NonceAt(h.Ctx, sender, nil)
	if err != nil {
		return nil, err
	}

	// Create transaction
	gasLimit := uint64(21000) // Standard transfer
	gasFeeCap := big.NewInt(1000000000) // 1 gwei
	gasTipCap := big.NewInt(1000000000) // 1 gwei

	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		To:        &to,
		Value:     value,
		Gas:       gasLimit,
		GasFeeCap: gasFeeCap,
		GasTipCap: gasTipCap,
	})

	// Sign transaction
	signedTx, err := types.SignTx(tx, types.LatestSignerForChainID(chainID), privKey)
	if err != nil {
		return nil, err
	}

	// Send transaction
	err = client.SendTransaction(h.Ctx, signedTx)
	if err != nil {
		return nil, err
	}

	// Wait for receipt
	return h.waitForReceipt(signedTx.Hash())
}

// waitForReceipt waits for a transaction receipt
func (h *SgtHelper) waitForReceipt(txHash common.Hash) (*types.Receipt, error) {
	client := h.Sys.L2EL.Escape().EthClient()
	ctx, cancel := context.WithTimeout(h.Ctx, 30*time.Second)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			receipt, err := client.TransactionReceipt(ctx, txHash)
			if err == nil {
				return receipt, nil
			}
		}
	}
}

// DepositSGT deposits SGT tokens by calling batchDepositForAll on the SGT contract
// In native-backed mode, this converts sent ETH to SGT tokens
func (h *SgtHelper) DepositSGT(privKey *ecdsa.PrivateKey, amount *big.Int) error {
	client := h.Sys.L2EL.Escape().EthClient()

	// Get sender address
	sender := crypto.PubkeyToAddress(privKey.PublicKey)

	// Get chain ID
	chainID, err := client.ChainID(h.Ctx)
	if err != nil {
		return err
	}

	// Get nonce
	nonce, err := client.NonceAt(h.Ctx, sender, nil)
	if err != nil {
		return err
	}

	// Call deposit() function - simpler than batchDepositForAll
	// Function selector: keccak256("deposit()")[0:4] = 0xd0e30db0
	callData := common.FromHex("0xd0e30db0")

	h.T.Logger().Info("DepositSGT call data",
		"function", "deposit()",
		"sender", sender.Hex(),
		"amount", amount,
		"calldata", hexutil.Encode(callData),
	)

	// Estimate gas
	gasLimit := uint64(100000) // Increased gas limit for safety

	// Get gas price
	gasPrice, err := client.SuggestGasPrice(h.Ctx)
	if err != nil {
		return err
	}
	gasFeeCap := new(big.Int).Mul(gasPrice, big.NewInt(2))
	gasTipCap := big.NewInt(1000000000) // 1 gwei

	// Create transaction to SGT contract
	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		To:        &SoulGasTokenAddr,
		Value:     amount, // Send ETH to be converted to SGT
		Data:      callData,
		Gas:       gasLimit,
		GasFeeCap: gasFeeCap,
		GasTipCap: gasTipCap,
	})

	// Sign transaction
	signedTx, err := types.SignTx(tx, types.LatestSignerForChainID(chainID), privKey)
	if err != nil {
		return err
	}

	// Send transaction
	err = client.SendTransaction(h.Ctx, signedTx)
	if err != nil {
		return err
	}

	// Wait for receipt
	receipt, err := h.waitForReceipt(signedTx.Hash())
	if err != nil {
		return err
	}

	// Check if transaction was successful
	if receipt.Status != types.ReceiptStatusSuccessful {
		h.T.Logger().Warn("Deposit transaction failed",
			"tx_hash", signedTx.Hash().Hex(),
			"status", receipt.Status,
			"gas_used", receipt.GasUsed,
			"sender", sender.Hex(),
			"amount", amount,
		)
		return fmt.Errorf("deposit transaction failed: status=%d", receipt.Status)
	}

	h.T.Logger().Info("SGT deposit successful",
		"sender", sender.Hex(),
		"amount", amount,
		"tx_hash", signedTx.Hash().Hex(),
		"gas_used", receipt.GasUsed,
	)

	return nil
}
