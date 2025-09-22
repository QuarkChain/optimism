package processors

import (
	"fmt"

	ethTypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type EventDecoderFn func(*ethTypes.Log, depset.ChainIndexFromID) (*types.ExecutingMessage, error)

func DecodeExecutingMessageLog(l *ethTypes.Log, depSet depset.ChainIndexFromID) (*types.ExecutingMessage, error) {
	return nil, fmt.Errorf("interop is disabled for gamma testnet")
}
