package sources

import (
	"embed"
	"encoding/json"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum-optimism/optimism/op-service/bigs"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

//go:embed testdata
var blocksTestdata embed.FS

type testMetadata struct {
	Name   string `json:"name"`
	Fail   bool   `json:"fail,omitempty"`
	Reason string `json:"reason,omitempty"`
}

func readJsonTestdata(t *testing.T, name string, dest any) {
	f, err := blocksTestdata.Open(name)
	require.NoError(t, err, "must open %q", name)
	require.NoError(t, json.NewDecoder(f).Decode(dest), "must json-decode %q", name)
	require.NoError(t, f.Close(), "must close %q", name)
}

func TestBlockHeaderJSON(t *testing.T) {
	headersDir, err := blocksTestdata.ReadDir("testdata/data/headers")
	require.NoError(t, err)

	for _, entry := range headersDir {
		if !strings.HasSuffix(entry.Name(), "_metadata.json") {
			continue
		}

		var metadata testMetadata
		readJsonTestdata(t, "testdata/data/headers/"+entry.Name(), &metadata)
		t.Run(metadata.Name, func(t *testing.T) {
			var header RPCHeader
			readJsonTestdata(t, "testdata/data/headers/"+strings.Replace(entry.Name(), "_metadata.json", "_data.json", 1), &header)

			h := header.computeBlockHash()
			if metadata.Fail {
				require.NotEqual(t, h, header.Hash, "expecting verification error")
			} else {
				require.Equal(t, h, header.Hash, "blockhash should verify ok")
			}
		})
	}
}

// Sepolia block 11867140 reproduces https://github.com/QuarkChain/optimism/issues/153.
func TestGlamsterdamHeaderJSON(t *testing.T) {
	const fixture = "testdata/data/headers/sepolia-glamsterdam-11867140.json"
	expectedHash := common.HexToHash("0xce79aa06c0f7165f224a49779a19310d7e55816d47c149d7f3acfc9d2f379a73")

	t.Run("RPCHeader", func(t *testing.T) {
		var header RPCHeader
		readJsonTestdata(t, fixture, &header)
		info, err := header.Info(false, true)
		require.NoError(t, err)
		require.Equal(t, expectedHash, info.Hash())
	})

	t.Run("GethHeader", func(t *testing.T) {
		var header types.Header
		readJsonTestdata(t, fixture, &header)
		require.Equal(t, expectedHash, header.Hash())
	})
}

func TestBlockJSON(t *testing.T) {
	blocksDir, err := blocksTestdata.ReadDir("testdata/data/blocks")
	require.NoError(t, err)

	for _, entry := range blocksDir {
		if !strings.HasSuffix(entry.Name(), "_metadata.json") {
			continue
		}

		var metadata testMetadata
		readJsonTestdata(t, "testdata/data/blocks/"+entry.Name(), &metadata)
		t.Run(metadata.Name, func(t *testing.T) {
			var block RPCBlock
			readJsonTestdata(t, "testdata/data/blocks/"+strings.Replace(entry.Name(), "_metadata.json", "_data.json", 1), &block)

			err := block.Verify()
			if metadata.Fail {
				require.NotNil(t, err, "expecting verification error")
				require.ErrorContains(t, err, metadata.Reason, "validation failed for incorrect reason")
			} else {
				require.NoError(t, err, "verification should pass")
			}
		})
	}
}

func TestBlockToExecutionPayloadIncludesEcotoneProperties(t *testing.T) {
	zero := uint64(0)

	hdr := &types.Header{
		ParentHash:       randHash(),
		UncleHash:        types.EmptyUncleHash,
		Coinbase:         common.Address{},
		Root:             randHash(),
		TxHash:           types.EmptyTxsHash,
		ReceiptHash:      randHash(),
		Bloom:            types.Bloom{},
		Difficulty:       big.NewInt(0),
		Number:           big.NewInt(1234),
		GasLimit:         0,
		GasUsed:          0,
		Time:             123456,
		Extra:            make([]byte, 0),
		MixDigest:        randHash(),
		Nonce:            types.BlockNonce{},
		BaseFee:          big.NewInt(100),
		WithdrawalsHash:  &types.EmptyWithdrawalsHash,
		ExcessBlobGas:    &zero,
		BlobGasUsed:      &zero,
		ParentBeaconRoot: &common.Hash{},
	}
	rhdr := RPCHeader{
		ParentBeaconRoot: hdr.ParentBeaconRoot,
		ParentHash:       hdr.ParentHash,
		WithdrawalsRoot:  hdr.WithdrawalsHash,
		UncleHash:        hdr.UncleHash,
		Coinbase:         hdr.Coinbase,
		Root:             hdr.Root,
		TxHash:           hdr.TxHash,
		ReceiptHash:      hdr.ReceiptHash,
		Bloom:            eth.Bytes256(hdr.Bloom),
		Difficulty:       *(*hexutil.Big)(hdr.Difficulty),
		Number:           hexutil.Uint64(bigs.Uint64Strict(hdr.Number)),
		GasLimit:         hexutil.Uint64(hdr.GasLimit),
		GasUsed:          hexutil.Uint64(hdr.GasUsed),
		Time:             hexutil.Uint64(hdr.Time),
		Extra:            hdr.Extra,
		MixDigest:        hdr.MixDigest,
		Nonce:            hdr.Nonce,
		BaseFee:          (*hexutil.Big)(hdr.BaseFee),
		Hash:             hdr.Hash(),
		BlobGasUsed:      (*hexutil.Uint64)(hdr.BlobGasUsed),
		ExcessBlobGas:    (*hexutil.Uint64)(hdr.ExcessBlobGas),
	}

	block := RPCBlock{
		RPCHeader:    rhdr,
		Transactions: types.Transactions{},
		Withdrawals:  &types.Withdrawals{},
	}

	envelope, err := block.ExecutionPayloadEnvelope(false)
	require.NoError(t, err)

	require.NotNil(t, envelope.ParentBeaconBlockRoot)
	require.Equal(t, *envelope.ParentBeaconBlockRoot, *hdr.ParentBeaconRoot)
	require.NotNil(t, envelope.ExecutionPayload.ExcessBlobGas)
	require.Equal(t, *envelope.ExecutionPayload.ExcessBlobGas, *rhdr.ExcessBlobGas)
	require.NotNil(t, envelope.ExecutionPayload.BlobGasUsed)
	require.Equal(t, *envelope.ExecutionPayload.BlobGasUsed, *rhdr.BlobGasUsed)
}
