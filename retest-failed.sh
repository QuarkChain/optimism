#!/usr/bin/env bash
# Rerun script for failed test cases extracted from test.log
# Generated from test run on 2026-04-28
#
# Failed tests:
#   op-supernode/supernode:
#     - TestCleanShutdown
#   op-deployer/pkg/deployer/pipeline:
#     - TestPopulateSuperchainState/valid_OPCM_address_only
#     - TestPopulateSuperchainState/OPCM_address_with_SuperchainConfigProxy
#     - TestPopulateSuperchainState/output_mapping_validation
#     - TestPopulateSuperchainState_OPCMV2/SuperchainConfigProxy_only
#     - TestPopulateSuperchainState_OPCMV2/output_mapping_validation

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$REPO_ROOT"

# ---------- env checks ----------
if [ -z "${SEPOLIA_RPC_URL:-}" ]; then
  echo "ERROR: SEPOLIA_RPC_URL is not set"
  exit 1
fi
if [ -z "${MAINNET_RPC_URL:-}" ]; then
  echo "ERROR: MAINNET_RPC_URL is not set"
  exit 1
fi

# ---------- env vars (mirrors Makefile DEFAULT_TEST_ENV_VARS + CI_ENV_VARS) ----------
export ENABLE_KURTOSIS=true
export OP_E2E_CANNON_ENABLED="false"
export OP_E2E_USE_HTTP=true
export ENABLE_ANVIL=true
export PARALLEL=$(nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 4)
export NAT_INTEROP_LOADTEST_TARGET=10
export NAT_INTEROP_LOADTEST_TIMEOUT=30s

mkdir -p ./tmp/test-results ./tmp/testlogs
export OP_TESTLOG_FILE_LOGGER_OUTDIR=$(realpath ./tmp/testlogs)

TEST_TIMEOUT="${TEST_TIMEOUT:-90m}"
RERUN_FAILS="${RERUN_FAILS:-3}"

echo "=== Rerunning failed tests ==="
echo "    PARALLEL=$PARALLEL  TEST_TIMEOUT=$TEST_TIMEOUT  RERUN_FAILS=$RERUN_FAILS"
echo ""

# ---------- sync op-deployer artifacts (required for pipeline tests) ----------
echo "=== Syncing op-deployer contract artifacts ==="
just -f op-deployer/justfile copy-contract-artifacts

OVERALL_EXIT=0

# ---------- 1. op-supernode/supernode ----------
echo ""
echo "=== [1/2] op-supernode/supernode: TestCleanShutdown ==="
gotestsum --format=testname \
  --junitfile=./tmp/test-results/retest-supernode.xml \
  --jsonfile=./tmp/testlogs/retest-supernode.json \
  --rerun-fails="$RERUN_FAILS" \
  --rerun-fails-max-failures=10 \
  --packages="github.com/ethereum-optimism/optimism/op-supernode/supernode" \
  -- \
  -parallel="$PARALLEL" \
  -timeout="$TEST_TIMEOUT" \
  -tags="ci" \
  -run "TestCleanShutdown" \
|| OVERALL_EXIT=$?

# ---------- 2. op-deployer/pkg/deployer/pipeline ----------
echo ""
echo "=== [2/2] op-deployer/pkg/deployer/pipeline: TestPopulateSuperchainState* ==="
gotestsum --format=testname \
  --junitfile=./tmp/test-results/retest-pipeline.xml \
  --jsonfile=./tmp/testlogs/retest-pipeline.json \
  --rerun-fails="$RERUN_FAILS" \
  --rerun-fails-max-failures=10 \
  --packages="github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/pipeline" \
  -- \
  -parallel="$PARALLEL" \
  -timeout="$TEST_TIMEOUT" \
  -tags="ci" \
  -run "TestPopulateSuperchainState" \
|| OVERALL_EXIT=$?

# ---------- summary ----------
echo ""
if [ "$OVERALL_EXIT" -eq 0 ]; then
  echo "=== All failed tests PASSED on rerun ==="
else
  echo "=== Some tests still FAILING (exit code $OVERALL_EXIT) ==="
fi
exit "$OVERALL_EXIT"
