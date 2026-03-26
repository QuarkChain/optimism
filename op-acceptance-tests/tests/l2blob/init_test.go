package l2blob

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-devstack/presets"
	"github.com/ethereum-optimism/optimism/op-devstack/stack"
	"github.com/ethereum-optimism/optimism/op-devstack/sysgo"
	"github.com/ethstorage/da-server/pkg/da"
)

func TestMain(m *testing.M) {
	// Start DAC server before the system — required by op-node when l2BlobTime is set.
	dacCfg := da.Config{
		SequencerIP: "127.0.0.1",
		ListenAddr:  fmt.Sprintf("0.0.0.0:%d", dacPort),
		StorePath:   os.TempDir(),
	}
	dacServer := da.NewServer(&dacCfg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := dacServer.Start(ctx); err != nil {
		panic(fmt.Sprintf("failed to start DAC server: %v", err))
	}
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer stopCancel()
		_ = dacServer.Stop(stopCtx)
	}()

	presets.DoMain(m,
		presets.WithMinimal(),
		stack.MakeCommon(stack.Combine[*sysgo.Orchestrator](
			sysgo.WithDeployerOptions(WithL2BlobAtGenesis),
			sysgo.WithGlobalL2CLOption(sysgo.L2CLDACUrls([]string{dacUrl})),
		)),
	)
}
