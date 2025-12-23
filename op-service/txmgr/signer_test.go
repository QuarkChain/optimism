package txmgr

import (
	"context"
	"math/big"
	"net/http"
	"testing"

	opsigner "github.com/ethereum-optimism/optimism/op-service/signer"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	optls "github.com/ethereum-optimism/optimism/op-service/tls"
	"github.com/ethereum-optimism/optimism/op-service/txmgr/metrics"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

func TestRemoteSigner(t *testing.T) {
	l1EthRpcValue := "http://65.108.230.142:8545"
	defaultCfg := NewCLIConfig(l1EthRpcValue, DefaultBatcherFlagValues)
	defaultCfg.SignerCLIConfig = opsigner.CLIConfig{
		// Endpoint: "https://op-signer2.testnet.l2.quarkchain.io:8080", // signer on 65.109.50.145
		Endpoint: "https://op-signer3.testnet.l2.quarkchain.io:8080", // backup signer on 65.108.236.27
		Address:  "0x74D3b2A1c7cD4Aea7AF3Ce8C08Cf5132ECBA64ED",
		Headers:  http.Header{},
		TLSConfig: optls.CLIConfig{
			TLSCaCert: "tls/ca.crt",
			TLSCert:   "tls/tls.crt",
			TLSKey:    "tls/tls.key",
			Enabled:   true,
		},
	}
	t.Log("Using config", "endpoint", defaultCfg.SignerCLIConfig.Endpoint)
	txMgr, err := NewSimpleTxManager("TEST", testlog.Logger(t, log.LevelDebug), &metrics.NoopTxMetrics{}, defaultCfg)
	require.NoError(t, err)
	to := common.HexToAddress("0x74D3b2A1c7cD4Aea7AF3Ce8C08Cf5132ECBA64ED")
	receipt, err := txMgr.Send(context.Background(), TxCandidate{
		Value:    big.NewInt(123),
		To:       &to,
		TxData:   nil,
		GasLimit: uint64(21000),
	})
	require.NoError(t, err)

	if receipt.Status == types.ReceiptStatusFailed {
		t.Fatal("tx failed")
	} else {
		t.Logf("tx succeeded: %+v", receipt)
	}
}
