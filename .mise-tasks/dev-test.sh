#!/usr/bin/env bash

#MISE description="Developers' local tests"
#MISE alias="dt"

set -e
# Inherit ERR trap in functions/subshells and catch pipeline failures
set -E -o pipefail
SECONDS=0

error_handler() {
    local rc=$?
    local cmd=${BASH_COMMAND:-unknown}
    local where
    where=$(caller 0 2>/dev/null || true)
    halt "Command failed (exit $rc): ${cmd} | at: ${where}"
}

trap 'error_handler' ERR

# Graceful halt helper: print error, show total time, stop script without closing terminal
halt() {
    echo "Error: $*" >&2
    echo "Execution time: ${SECONDS} seconds"
    # remove ERR trap to avoid double messaging
    trap - ERR
    # If sourced use return, else exit with success code (0) to not signal failure
    return 0 2>/dev/null || exit 0
}

# Environment verify
echo "==========Checking environment..."
# mise install

if [ -z "${MISE_SHELL:-}" ]; then
    if [ -n "${ZSH_VERSION:-}" ]; then
        eval "$(mise activate zsh)"
    elif [ -n "${BASH_VERSION:-}" ]; then
        eval "$(mise activate bash)"
    fi
fi

if [ -n "$(git status --porcelain)" ]; then
  echo "WARN: Working tree not clean. Commit/stash changes first." >&2
  git status --porcelain
  exit 1
fi

for var in SEPOLIA_RPC_URL MAINNET_RPC_URL; do
    if [ -z "${!var}" ]; then
        echo "Error: $var is not set."
        return 0 2>/dev/null || exit 0
    fi
done
echo "==========Checking environment done"

echo "==========Cleaning workspace..."
make nuke
echo "==========Workspace cleaned."

# Updating dependencies in contracts lib
cd packages/contracts-bedrock
forge install

# contracts-bedrock-tests & contracts-bedrock-tests-preimage-oracle
echo "==========Starting contracts-bedrock tests..."
just build-go-ffi
for _spec in \
    "-name '*.t.sol' -not -name 'PreimageOracle.t.sol'" \
    "-name 'PreimageOracle.t.sol'"; do
    TEST_FILES=$(eval find test ${_spec})
    if [ -z "$TEST_FILES" ]; then
        echo "No tests matched spec: ${_spec}; skipping"
        continue
    fi
    TEST_FILES=$(echo "$TEST_FILES" | sed 's|^test/||')
    MATCH_PATH="./test/{$(echo "$TEST_FILES" | paste -sd "," -)}"
    echo "Running forge test --match-path $MATCH_PATH"
    forge test --match-path "$MATCH_PATH"
done
echo "==========Contracts-bedrock tests done."

# contracts-bedrock-build
just forge-build --deny-warnings --skip test
forge script "scripts/deploy/DeployImplementations.s.sol" \
    --skip "/**/test/**" \
    --sig "idonotexist()" \
    --skip-simulation \
    2>/dev/null || true
ls forge-artifacts/DeployImplementations.s.sol/DeployImplementations.json
cd ../..

# cannon-prestate-quick
echo "==========Starting cannon-prestates-quick..."
make cannon-prestates
echo "==========Cannon-prestates-quick done."

# go-tests-full
echo "==========Starting go-tests-full..."
export TEST_TIMEOUT=90m
make go-tests-ci
echo "==========Go-tests-full done."

# op-e2e-fuzz
echo "==========Starting op-e2e-fuzz..."
cd op-e2e && make fuzz && cd ..
echo "==========Op-e2e-fuzz done."

# cannon-fuzz
echo "==========Starting cannon-fuzz..."
cd cannon && make fuzz && cd ..
echo "==========Cannon-fuzz done."

# op-program-compat
echo "==========Starting op-program-compat..."
cd op-program && make verify-compat && cd ..
echo "==========Op-program-compat done."

# fuzz-golang
echo "==========Starting fuzz-golang..."
if ! command -v parallel >/dev/null 2>&1; then
    echo "Notice: GNU parallel not found; stopping before fuzz and later steps." >&2
    echo "Install it to enable fuzzing. Examples:" >&2
    echo "  macOS:   brew install parallel" >&2
    echo "  Ubuntu:  apt-get update && apt-get install -y parallel" >&2
    return 0 2>/dev/null || exit 0
fi
for dir in op-challenger op-node op-service op-chain-ops; do
    (cd "$dir" && just fuzz && cd ..)
done
echo "==========Fuzz-golang done."

echo "Execution time: $((SECONDS / 60)) minute(s) and $((SECONDS % 60)) second(s)"