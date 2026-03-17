package presets

import (
	"github.com/ethereum-optimism/optimism/op-devstack/stack"
	"github.com/ethereum-optimism/optimism/op-devstack/sysgo"
)

// WithSGT enables Soul Gas Token deployment in L2 genesis.
// enabled: whether to deploy SGT contract
// nativeBacked: whether SGT is 1:1 backed by native token (true for QuarkChain's SoulQKC)
func WithSGT(enabled bool, nativeBacked bool) stack.CommonOption {
	return stack.MakeCommon(sysgo.WithDeployerOptions(
		sysgo.WithSoulGasToken(enabled, nativeBacked),
	))
}
