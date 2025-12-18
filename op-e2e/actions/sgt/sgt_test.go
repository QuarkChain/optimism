package sgt

import (
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/op-core/predeploys"
	"github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/bindings"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/require"
)

func TestDynamicSGT(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)
	dp := e2eutils.MakeDeployParams(t, helpers.DefaultRollupTestParams())
	dp.DeployConfig.DeploySoulGasToken = true
	timeOffset := new(hexutil.Uint64)
	*timeOffset = 100
	dp.DeployConfig.SoulGasTokenTimeOffset = timeOffset
	sd := e2eutils.Setup(t, dp, helpers.DefaultAlloc)
	log := testlog.Logger(t, log.LevelDebug)
	miner, engine, sequencer := helpers.SetupSequencerTest(t, sd, log)

	cl := engine.EthClient()
	depositSGT(gt, engine, sd, dp, dp.Addresses.Alice, e2eutils.Ether(2))

	sequencer.ActL2PipelineFull(t)

	genesisTime := sequencer.L2Unsafe().Time

	// Make L2 block
	sequencer.ActL2StartBlock(t)
	engine.ActL2IncludeTx(dp.Addresses.Alice)(t)
	sequencer.ActL2EndBlock(t)

	// Alice makes a L2 tx

	balance1, err := cl.BalanceAt(t.Ctx(), dp.Addresses.Alice, nil)
	require.NoError(t, err)
	n, err := cl.PendingNonceAt(t.Ctx(), dp.Addresses.Alice)
	require.NoError(t, err)
	signer := types.LatestSigner(sd.L2Cfg.Config)
	tx := types.MustSignNewTx(dp.Secrets.Alice, signer, &types.DynamicFeeTx{
		ChainID:   sd.L2Cfg.Config.ChainID,
		Nonce:     n,
		GasTipCap: big.NewInt(2 * params.GWei),
		GasFeeCap: new(big.Int).Add(miner.L1Chain().CurrentBlock().BaseFee, big.NewInt(2*params.GWei)),
		Gas:       params.TxGas,
		To:        &dp.Addresses.Bob,
	})
	require.NoError(t, cl.SendTransaction(t.Ctx(), tx))

	// Make L2 block
	sequencer.ActL2StartBlock(t)
	engine.ActL2IncludeTx(dp.Addresses.Alice)(t)
	sequencer.ActL2EndBlock(t)

	balance2, err := cl.BalanceAt(t.Ctx(), dp.Addresses.Alice, nil)
	require.NoError(t, err)
	// Check that the balance is different
	// because the SGT is not active yet
	require.True(t, balance2.Cmp(balance1) < 0)

	// advance to the block where the SGT is active
	sequencer.ActBuildL2ToTime(t, genesisTime+(uint64)(*dp.DeployConfig.SoulGasTokenTimeOffset))

	// Alice makes a L2 tx
	n, err = cl.PendingNonceAt(t.Ctx(), dp.Addresses.Alice)
	require.NoError(t, err)
	tx = types.MustSignNewTx(dp.Secrets.Alice, signer, &types.DynamicFeeTx{
		ChainID:   sd.L2Cfg.Config.ChainID,
		Nonce:     n,
		GasTipCap: big.NewInt(2 * params.GWei),
		GasFeeCap: new(big.Int).Add(miner.L1Chain().CurrentBlock().BaseFee, big.NewInt(2*params.GWei)),
		Gas:       params.TxGas,
		To:        &dp.Addresses.Bob,
	})
	require.NoError(t, cl.SendTransaction(t.Ctx(), tx))

	// Make L2 block
	sequencer.ActL2StartBlock(t)
	engine.ActL2IncludeTx(dp.Addresses.Alice)(t)
	sequencer.ActL2EndBlock(t)

	balance3, err := cl.BalanceAt(t.Ctx(), dp.Addresses.Alice, nil)
	require.NoError(t, err)

	// Check that the balance is the same as before
	// because the SGT is active
	require.True(t, balance3.Cmp(balance2) == 0)
}

func depositSGT(t *testing.T, engine *helpers.L2Engine, sd *e2eutils.SetupData, dp *e2eutils.DeployParams, target common.Address, depositSgtValue *big.Int) {

	sgtAddr := predeploys.SoulGasTokenAddr
	sgtContract, err := bindings.NewSoulGasToken(sgtAddr, engine.EthClient())
	require.NoError(t, err)

	txOpts, err := bind.NewKeyedTransactorWithChainID(dp.Secrets.Alice, sd.L2Cfg.Config.ChainID)
	require.NoError(t, err)
	txOpts.Value = depositSgtValue

	_, err = sgtContract.BatchDepositForAll(txOpts, []common.Address{target}, depositSgtValue)
	require.NoError(t, err)

}
