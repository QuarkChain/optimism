package presets

import (
	"github.com/ethereum-optimism/optimism/op-devstack/stack"
	"github.com/ethereum-optimism/optimism/op-devstack/sysgo"
)

// WithSGT configures the system to deploy the Soul Gas Token (SGT) contract.
// enabled controls whether SGT is deployed; nativeBacked controls whether SGT is 1:1 backed by native token.
func WithSGT(enabled bool, nativeBacked bool) stack.CommonOption {
	return stack.MakeCommon(sysgo.WithDeployerOptions(
		sysgo.WithSoulGasToken(enabled, nativeBacked),
	))
}
