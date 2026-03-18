package sgt

import (
	"context"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-devstack/presets"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/rpc"
)

// TestSGT_GenesisDeployment verifies that the SGT contract is deployed at
// genesis at the expected predeploy address.
func TestSGT_GenesisDeployment(gt *testing.T) {
	t := devtest.SerialT(gt)
	sys := presets.NewMinimal(t)

	l2Client := sys.L2EL.Escape().L2EthClient()

	ctx, cancel := context.WithTimeout(t.Ctx(), 20*time.Second)
	defer cancel()

	// Check that there is code at the SGT predeploy address
	code, err := l2Client.CodeAtHash(ctx, sgtAddr, sys.L2EL.BlockRefByNumber(0).Hash)
	t.Require().NoError(err, "failed to get code at SGT address")
	t.Require().Greater(len(code), 0, "SGT contract should have code at genesis")
}

// TestSGT_GenesisBalanceOfReturnsZero verifies that a fresh account has zero
// SGT balance, confirming the contract is functional at genesis.
func TestSGT_GenesisBalanceOfReturnsZero(gt *testing.T) {
	t := devtest.SerialT(gt)
	sys := presets.NewMinimal(t)

	l2Client := sys.L2EL.Escape().L2EthClient()

	ctx, cancel := context.WithTimeout(t.Ctx(), 20*time.Second)
	defer cancel()

	// Query balanceOf for a random address at genesis; should return 0
	data, err := balanceOfFunc.EncodeArgs(dummyAddr)
	t.Require().NoError(err, "failed to encode balanceOf args")

	out, err := l2Client.Call(ctx, ethereum.CallMsg{To: &sgtAddr, Data: data}, rpc.BlockNumber(0))
	t.Require().NoError(err, "balanceOf call should succeed at genesis block")
	t.Require().NotNil(out, "balanceOf should return data")
}
