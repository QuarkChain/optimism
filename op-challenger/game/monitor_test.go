package game

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-challenger/game/types"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/wait"
	"github.com/ethereum-optimism/optimism/op-service/clock"
)

// TestMonitorGames tests that the monitor can handle a new head event
// and resubscribe to new heads if the subscription errors.
func TestMonitorGames(t *testing.T) {
	t.Run("Schedules games", func(t *testing.T) {
		addr1 := common.Address{0xaa}
		addr2 := common.Address{0xbb}
		monitor, source, sched, mockHeadSource, preimages, _ := setupMonitorTest(t, []common.Address{}, 0)
		source.games = []types.GameMetadata{newFDG(addr1, 9999), newFDG(addr2, 9999)}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			headerNotSent := true
			for {
				if len(sched.Scheduled()) >= 1 {
					break
				}
				sub := mockHeadSource.Sub()
				if sub == nil {
					continue
				}
				if headerNotSent {
					select {
					case sub.headers <- &ethtypes.Header{
						Number: big.NewInt(1),
					}:
						headerNotSent = false
					case <-ctx.Done():
						return
					default:
					}
				}
				// Just to avoid a tight loop
				time.Sleep(100 * time.Millisecond)
			}
			mockHeadSource.SetErr(fmt.Errorf("eth subscribe test error"))
			cancel()
		}()

		monitor.StartMonitoring()
		<-ctx.Done()
		monitor.StopMonitoring()
		require.Len(t, sched.Scheduled(), 1)
		require.Equal(t, []common.Address{addr1, addr2}, sched.Scheduled()[0])
		require.GreaterOrEqual(t, preimages.ScheduleCount(), 1, "Should schedule preimage checks")
	})

	t.Run("Resubscribes on error", func(t *testing.T) {
		addr1 := common.Address{0xaa}
		addr2 := common.Address{0xbb}
		monitor, source, sched, mockHeadSource, preimages, _ := setupMonitorTest(t, []common.Address{}, 0)
		source.games = []types.GameMetadata{newFDG(addr1, 9999), newFDG(addr2, 9999)}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			// Wait for the subscription to be created
			waitErr := wait.For(context.Background(), 5*time.Second, func() (bool, error) {
				return mockHeadSource.Sub() != nil, nil
			})
			require.NoError(t, waitErr)
			mockHeadSource.Sub().errChan <- fmt.Errorf("test error")
			for {
				if len(sched.Scheduled()) >= 1 {
					break
				}
				sub := mockHeadSource.Sub()
				if sub == nil {
					continue
				}
				select {
				case sub.headers <- &ethtypes.Header{
					Number: big.NewInt(1),
				}:
				case <-ctx.Done():
					return
				default:
				}
				// Just to avoid a tight loop
				time.Sleep(100 * time.Millisecond)
			}
			mockHeadSource.SetErr(fmt.Errorf("eth subscribe test error"))
			cancel()
		}()

		monitor.StartMonitoring()
		<-ctx.Done()
		monitor.StopMonitoring()
		require.NotEmpty(t, sched.Scheduled()) // We might get more than one update scheduled.
		require.Equal(t, []common.Address{addr1, addr2}, sched.Scheduled()[0])
		require.GreaterOrEqual(t, preimages.ScheduleCount(), 1, "Should schedule preimage checks")
	})
}

func TestMonitorGlamsterdamHead(t *testing.T) {
	data, err := os.ReadFile("../../op-service/sources/testdata/data/headers/sepolia-glamsterdam-11867140.json")
	require.NoError(t, err)
	var header ethtypes.Header
	require.NoError(t, json.Unmarshal(data, &header))
	expectedHash := common.HexToHash("0xce79aa06c0f7165f224a49779a19310d7e55816d47c149d7f3acfc9d2f379a73")

	monitor, source, sched, headSource, preimages, _ := setupMonitorTest(t, nil, 0)
	source.games = []types.GameMetadata{newFDG(common.Address{0xaa}, header.Time)}
	sub, err := monitor.resubscribeFunction()(context.Background(), nil)
	require.NoError(t, err)
	t.Cleanup(sub.Unsubscribe)
	headSource.Sub().headers <- &header

	require.Eventually(t, func() bool {
		return preimages.ScheduleCount() == 1
	}, 5*time.Second, 10*time.Millisecond)
	sub.Unsubscribe()

	require.Equal(t, expectedHash, source.blockHash, "game queries must use the canonical L1 hash")
	require.Equal(t, expectedHash, preimages.blockHash, "preimage checks must use the canonical L1 hash")
	require.Len(t, sched.Scheduled(), 1)
}

func TestMonitorCreateAndProgressGameAgents(t *testing.T) {
	monitor, source, sched, _, _, _ := setupMonitorTest(t, []common.Address{}, 0)

	addr1 := common.Address{0xaa}
	addr2 := common.Address{0xbb}
	source.games = []types.GameMetadata{newFDG(addr1, 9999), newFDG(addr2, 9999)}

	require.NoError(t, monitor.progressGames(context.Background(), common.Hash{0x01}, 0))

	require.Len(t, sched.Scheduled(), 1)
	require.Equal(t, []common.Address{addr1, addr2}, sched.Scheduled()[0])
}

func TestMonitorOnlyScheduleSpecifiedGame(t *testing.T) {
	addr1 := common.Address{0xaa}
	addr2 := common.Address{0xbb}
	monitor, source, sched, _, _, stubClaimer := setupMonitorTest(t, []common.Address{addr2}, 0)
	source.games = []types.GameMetadata{newFDG(addr1, 9999), newFDG(addr2, 9999)}

	require.NoError(t, monitor.progressGames(context.Background(), common.Hash{0x01}, 0))

	require.Len(t, sched.Scheduled(), 1)
	require.Equal(t, []common.Address{addr2}, sched.Scheduled()[0])
	require.Equal(t, 1, stubClaimer.scheduledGames)
}

func TestMinUpdatePeriod(t *testing.T) {
	tests := []struct {
		name                   string
		minUpdatePeriodSeconds int64
		processBlock2          bool
		processBlock3          bool
	}{
		{name: "ZeroUpdatePeriod", minUpdatePeriodSeconds: 0, processBlock2: true, processBlock3: true},
		{name: "SmallUpdatePeriod", minUpdatePeriodSeconds: 1, processBlock2: true, processBlock3: true},
		{name: "SkipBlockUpdatePeriod", minUpdatePeriodSeconds: 1000, processBlock2: false, processBlock3: true},
		{name: "LongUpdatePeriod", minUpdatePeriodSeconds: 1000000, processBlock2: false, processBlock3: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			block1 := eth.L1BlockRef{
				Hash:   common.HexToHash("0x1"),
				Number: 1,
				Time:   1_000_000,
			}
			block2 := eth.L1BlockRef{
				Hash:   common.HexToHash("0x2"),
				Number: 2,
				Time:   1_000_500,
			}
			block3 := eth.L1BlockRef{
				Hash:   common.HexToHash("0x2"),
				Number: 2,
				Time:   1_001_000,
			}
			addr1 := common.Address{0xaa}
			addr2 := common.Address{0xbb}
			monitor, source, sched, _, _, _ := setupMonitorTest(t, []common.Address{addr2}, test.minUpdatePeriodSeconds)
			source.games = []types.GameMetadata{newFDG(addr1, 9999), newFDG(addr2, 9999)}
			monitor.onNewL1Head(context.Background(), block1)
			expectedScheduleCount := 1
			require.Len(t, sched.Scheduled(), expectedScheduleCount, "Should schedule update on first new block")

			monitor.onNewL1Head(context.Background(), block2)
			if test.processBlock2 {
				expectedScheduleCount++
			}
			require.Len(t, sched.Scheduled(), expectedScheduleCount, "Should not schedule update prior to min update period being reached")

			monitor.onNewL1Head(context.Background(), block3)
			if test.processBlock3 {
				expectedScheduleCount++
			}
			require.Len(t, sched.Scheduled(), expectedScheduleCount, "Should schedule update once min update period is reached")
		})
	}
}

func newFDG(proxy common.Address, timestamp uint64) types.GameMetadata {
	return types.GameMetadata{
		Proxy:     proxy,
		Timestamp: timestamp,
	}
}

func setupMonitorTest(
	t *testing.T,
	allowedGames []common.Address,
	minUpdatePeriodSeconds int64,
) (*gameMonitor, *stubGameSource, *stubScheduler, *mockNewHeadSource, *stubPreimageScheduler, *mockScheduler) {
	logger := testlog.Logger(t, log.LevelDebug)
	source := &stubGameSource{}
	sched := &stubScheduler{}
	preimages := &stubPreimageScheduler{}
	mockHeadSource := &mockNewHeadSource{}
	stubClaimer := &mockScheduler{}
	monitor := newGameMonitor(
		logger,
		clock.NewSimpleClock(),
		source,
		sched,
		preimages,
		time.Duration(0),
		stubClaimer,
		allowedGames,
		mockHeadSource,
		time.Duration(minUpdatePeriodSeconds)*time.Second,
	)
	return monitor, source, sched, mockHeadSource, preimages, stubClaimer
}

type mockNewHeadSource struct {
	sync.Mutex
	sub *mockSubscription
	err error
}

func (m *mockNewHeadSource) Sub() *mockSubscription {
	m.Lock()
	defer m.Unlock()
	return m.sub
}

func (m *mockNewHeadSource) SetSub(sub *mockSubscription) {
	m.Lock()
	defer m.Unlock()
	m.sub = sub
}

func (m *mockNewHeadSource) SetErr(err error) {
	m.Lock()
	defer m.Unlock()
	m.err = err
}

func (m *mockNewHeadSource) Subscribe(
	_ context.Context,
	namespace string,
	ch any,
	_ ...any,
) (ethereum.Subscription, error) {
	m.Lock()
	defer m.Unlock()
	if namespace != "eth" {
		return nil, fmt.Errorf("only support eth RPC subscription, got %q", namespace)
	}
	errChan := make(chan error)
	m.sub = &mockSubscription{errChan, (ch).(chan<- *ethtypes.Header)}
	if m.err != nil {
		return nil, m.err
	}
	return m.sub, nil
}

type mockScheduler struct {
	scheduleErr    error
	scheduledGames int
}

func (m *mockScheduler) Schedule(_ uint64, games []types.GameMetadata) error {
	m.scheduledGames += len(games)
	return m.scheduleErr
}

type mockSubscription struct {
	errChan chan error
	headers chan<- *ethtypes.Header
}

func (m *mockSubscription) Unsubscribe() {}

func (m *mockSubscription) Err() <-chan error {
	return m.errChan
}

type stubGameSource struct {
	fetchErr  error
	games     []types.GameMetadata
	blockHash common.Hash
}

func (s *stubGameSource) GetGamesAtOrAfter(
	_ context.Context,
	blockHash common.Hash,
	_ uint64,
) ([]types.GameMetadata, error) {
	s.blockHash = blockHash
	if s.fetchErr != nil {
		return nil, s.fetchErr
	}
	return s.games, nil
}

type stubScheduler struct {
	sync.Mutex
	scheduled [][]common.Address
}

func (s *stubScheduler) Scheduled() [][]common.Address {
	s.Lock()
	defer s.Unlock()
	return s.scheduled
}

func (s *stubScheduler) Schedule(games []types.GameMetadata, blockNumber uint64) error {
	s.Lock()
	defer s.Unlock()
	var addrs []common.Address
	for _, game := range games {
		addrs = append(addrs, game.Proxy)
	}
	s.scheduled = append(s.scheduled, addrs)
	return nil
}

type stubPreimageScheduler struct {
	sync.Mutex
	scheduleCount int
	blockHash     common.Hash
}

func (s *stubPreimageScheduler) Schedule(blockHash common.Hash, _ uint64) error {
	s.Lock()
	defer s.Unlock()
	s.scheduleCount++
	s.blockHash = blockHash
	return nil
}

func (s *stubPreimageScheduler) ScheduleCount() int {
	s.Lock()
	defer s.Unlock()
	return s.scheduleCount
}
