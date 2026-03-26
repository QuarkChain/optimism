package l2blob

import (
	"bytes"
	"context"
	"fmt"
	mrand "math/rand"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-chain-ops/devkeys"
	opforks "github.com/ethereum-optimism/optimism/op-core/forks"
	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-devstack/presets"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/intentbuilder"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/txmgr"
	"github.com/ethereum-optimism/optimism/op-service/txplan"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	daclient "github.com/ethstorage/da-server/pkg/da/client"
	"github.com/holiman/uint256"
)

const (
	dacPort = 39777
)

var (
	dacUrl = fmt.Sprintf("http://127.0.0.1:%d", dacPort)
)

// WithL2BlobAtGenesis enables L2 blob support at genesis for all L2 chains.
func WithL2BlobAtGenesis(_ devtest.P, _ devkeys.Keys, builder intentbuilder.Builder) {
	offset := uint64(0)
	for _, l2Cfg := range builder.L2s() {
		l2Cfg.WithForkAtGenesis(opforks.Isthmus)
	}
	// Set L2GenesisBlobTimeOffset directly via global deploy overrides
	// since l2BlobTime is not a standard fork.
	builder.WithGlobalOverride("l2GenesisBlobTimeOffset", (*hexutil.Uint64)(&offset))
}

// withL2Blobs is like txplan.WithBlobs but computes the blob base fee from the
// header's ExcessBlobGas instead of BlobScheduleConfig (which is not populated
// for L2 chains).
func withL2Blobs(blobs []*eth.Blob) txplan.Option {
	return func(tx *txplan.PlannedTx) {
		tx.Type.Set(types.BlobTxType)
		tx.BlobFeeCap.DependOn(&tx.AgainstBlock)
		tx.BlobFeeCap.Fn(func(_ context.Context) (*uint256.Int, error) {
			header := tx.AgainstBlock.Value()
			if ebg := header.ExcessBlobGas(); ebg != nil && *ebg > 0 {
				fee := eth.CalcBlobFeeCancun(*ebg)
				return uint256.MustFromBig(fee), nil
			}
			// Genesis or no excess blob gas — use minimum
			return uint256.NewInt(1), nil
		})
		var blobHashes []common.Hash
		tx.Sidecar.Fn(func(_ context.Context) (*types.BlobTxSidecar, error) {
			sidecar, hashes, err := txmgr.MakeSidecar(blobs, false)
			if err != nil {
				return nil, fmt.Errorf("make blob tx sidecar: %w", err)
			}
			blobHashes = hashes
			return sidecar, nil
		})
		tx.BlobHashes.DependOn(&tx.Sidecar)
		tx.BlobHashes.Fn(func(_ context.Context) ([]common.Hash, error) {
			return blobHashes, nil
		})
	}
}

// TestSubmitL2BlobTransaction tests that blob transactions can be submitted on L2
// and that the blobs are retrievable from the DAC server.
// Mirrors op-e2e/l2blob/l2blob_test.go::TestSubmitTXWithBlobsFunctionSuccess.
func TestSubmitL2BlobTransaction(gt *testing.T) {
	t := devtest.SerialT(gt)
	sys := presets.NewMinimal(t)

	t.Require().True(sys.L2Chain.IsForkActive(opforks.Isthmus), "Isthmus fork must be active")

	alice := sys.FunderL2.NewFundedEOA(eth.OneEther)

	// Create random blobs
	numBlobs := 3
	blobs := make([]*eth.Blob, numBlobs)
	for i := range blobs {
		b := getRandBlob(t, int64(i))
		blobs[i] = &b
	}

	// Send a blob transaction
	planned := alice.Transact(
		alice.Plan(),
		withL2Blobs(blobs),
		txplan.WithTo(&common.Address{}), // blob tx requires a 'to' address
	)

	receipt, err := planned.Included.Eval(t.Ctx())
	t.Require().NoError(err, "blob transaction must be included")
	t.Require().NotNil(receipt, "receipt must not be nil")
	t.Require().Equal(uint64(1), receipt.Status, "blob transaction must succeed")

	// Verify the transaction has blob hashes
	tx, err := planned.Signed.Eval(t.Ctx())
	t.Require().NoError(err, "must get signed transaction")
	blobHashes := tx.BlobHashes()
	t.Require().Equal(numBlobs, len(blobHashes), "transaction must have correct number of blob hashes")

	// Verify blob gas usage in the block
	blockNum := receipt.BlockNumber
	client := sys.L2EL.Escape().L2EthClient()
	header, err := client.InfoByNumber(t.Ctx(), blockNum.Uint64())
	t.Require().NoError(err, "must get block header")
	blobGasUsed := header.BlobGasUsed()
	t.Require().NotZero(blobGasUsed, "blob gas used must be non-zero for block with blob transactions")

	// Download blobs from DAC server and verify content
	dacCtx, cancel := context.WithTimeout(t.Ctx(), 5*time.Second)
	defer cancel()
	dacClient := daclient.New([]string{dacUrl})
	dblobs, err := dacClient.GetBlobs(dacCtx, blobHashes)
	t.Require().NoError(err, "must download blobs from DAC server")
	t.Require().Equal(len(blobHashes), len(dblobs), "downloaded blobs count must match blob hashes")

	for i, blob := range dblobs {
		t.Require().Equal(eth.BlobSize, len(blob), "downloaded blob %d must have correct size", i)
		t.Require().True(bytes.Equal(blob, blobs[i][:]),
			"blob %d content mismatch: got %s vs expected %s",
			i, common.Bytes2Hex(blob[:32]), common.Bytes2Hex(blobs[i][:32]))
	}

	t.Logf("L2 blob transaction included: block=%d, blobGasUsed=%d, blobHashes=%d",
		blockNum, blobGasUsed, len(blobHashes))
}

// getRandBlob generates a random blob with the given seed.
func getRandBlob(t devtest.T, seed int64) eth.Blob {
	r := mrand.New(mrand.NewSource(seed))
	bigData := eth.Data(make([]byte, eth.MaxBlobDataSize))
	_, err := r.Read(bigData)
	t.Require().NoError(err)
	var b eth.Blob
	err = b.FromData(bigData)
	t.Require().NoError(err)
	return b
}
