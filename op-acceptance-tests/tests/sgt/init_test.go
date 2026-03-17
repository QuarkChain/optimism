package sgt

import (
	"testing"

	"github.com/ethereum-optimism/optimism/op-devstack/presets"
)

// TestMain creates the test-setups for SGT E2E tests
//
// SGT (Soul Gas Token) is enabled via genesis deployment.
// The WithSGT preset configures the system to deploy the SGT contract
// at address 0x4200000000000000000000000000000000000800 during genesis.
//
// Configuration:
// - enabled: true (deploy SGT contract)
// - nativeBacked: true (SGT 1:1 backed by native token, like QuarkChain's SoulQKC)
func TestMain(m *testing.M) {
	presets.DoMain(m,
		presets.WithMinimal(),
		presets.WithSGT(true, true), // Enable SGT with native backing
	)
}
