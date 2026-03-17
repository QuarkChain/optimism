package sgt

import (
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-devstack/presets"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// TestSGT_GenesisDeployment verifies that SGT contract is deployed when enabled via preset
func TestSGT_GenesisDeployment(gt *testing.T) {
	t := devtest.SerialT(gt)

	// Create system (SGT enabled via TestMain)
	sys := presets.NewMinimal(t)

	ctx := t.Ctx()
	rpcClient := sys.L2EL.Escape().EthClient().RPC()

	sgtAddr := "0x4200000000000000000000000000000000000800"

	// Check contract code
	var code hexutil.Bytes
	err := rpcClient.CallContext(ctx, &code, "eth_getCode", sgtAddr, "latest")
	t.Require().NoError(err, "eth_getCode should not error")

	// SGT contract must be deployed
	t.Require().NotEmpty(code, "SGT contract code should not be empty")
	t.Logger().Info("SGT contract deployed successfully",
		"address", sgtAddr,
		"code_size", len(code),
	)

	// Verify contract is initialized by calling name() function
	// Function selector: keccak256("name()")[0:4] = 0x06fdde03
	nameCallData := "0x06fdde03"

	var nameResult hexutil.Bytes
	err = rpcClient.CallContext(ctx, &nameResult, "eth_call", map[string]any{
		"to":   sgtAddr,
		"data": nameCallData,
	}, "latest")
	t.Require().NoError(err, "name() call should not error")
	t.Require().NotEmpty(nameResult, "name() should return result")

	t.Logger().Info("SGT contract initialized",
		"name_result_length", len(nameResult),
	)

	// Verify balanceOf works for a test account
	alice := sys.Wallet.NewEOA(sys.L2EL)

	// balanceOf(address) - keccak256("balanceOf(address)")[0:4] = 0x70a08231
	callData := "0x70a08231" + common.BytesToHash(alice.Address().Bytes()).Hex()[2:]

	var balanceResult hexutil.Bytes
	err = rpcClient.CallContext(ctx, &balanceResult, "eth_call", map[string]any{
		"to":   sgtAddr,
		"data": callData,
	}, "latest")
	t.Require().NoError(err, "balanceOf() call should not error")

	balance := new(big.Int).SetBytes(balanceResult)
	t.Logger().Info("SGT balanceOf query successful",
		"account", alice.Address(),
		"sgt_balance", balance,
	)

	// Initial balance should be zero for new account
	t.Require().True(balance.Cmp(big.NewInt(0)) == 0, "Initial SGT balance should be zero")

	t.Logger().Info("✅ SGT genesis deployment test passed")
}
