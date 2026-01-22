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

if [ -z "${MISE_SHELL:-}" ]; then
    if [ -n "${ZSH_VERSION:-}" ]; then
        eval "$(mise activate zsh)"
    elif [ -n "${BASH_VERSION:-}" ]; then
        eval "$(mise activate bash)"
    fi
fi

# Ensure required tools exist.
if command -v mise >/dev/null 2>&1; then
    for tool in forge cast just anvil; do
        if ! command -v "$tool" >/dev/null 2>&1; then
            echo "Notice: '$tool' not found; attempting to install via mise..." >&2
            mise install "$tool" || halt "Failed to install '$tool' via mise. Try running: mise install"
        fi
    done
else
    for tool in cast just anvil; do
        command -v "$tool" >/dev/null 2>&1 || halt "Required tool '$tool' not found. Install via mise (recommended): mise install ${tool}"
    done
fi

# Ensure gotestsum exists for Go test targets (used by Makefile).
if ! command -v gotestsum >/dev/null 2>&1; then
    if command -v go >/dev/null 2>&1; then
        echo "Notice: 'gotestsum' not found; installing via go install..." >&2
        GO_BIN="$(go env GOPATH)/bin"
        go install gotest.tools/gotestsum@latest || halt "Failed to install 'gotestsum'. Try running: go install gotest.tools/gotestsum@latest"
        export PATH="$GO_BIN:$PATH"
    else
        halt "Required tool 'gotestsum' not found and 'go' is not available. Install Go or gotestsum first."
    fi
fi

echo "Current branch: $(git rev-parse --abbrev-ref HEAD)" >&2
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

# Updating dependencies in contracts lib
pushd packages/contracts-bedrock > /dev/null
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
just clean && just forge-build --deny-warnings --skip test
popd > /dev/null

# op-deployer embedded artifacts (required by op-deployer Go tests)
echo "==========Packing op-deployer artifacts..."
just -f op-deployer/justfile copy-contract-artifacts
echo "==========Artifacts packed."

# cannon-prestate-quick
echo "==========Starting cannon-prestates-quick..."
make cannon-prestates
echo "==========Cannon-prestates-quick done."


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

# op-e2e-tests
echo "==========Starting op-e2e-tests..."
cd op-e2e
make test-actions
make test-ws
cd ..
echo "==========Op-e2e-tests done."

# go-tests-full
echo "==========Starting go-tests-full..."
export TEST_TIMEOUT=90m
make go-tests-ci
echo "==========Go-tests-full done."


echo "Execution time: $((SECONDS / 60)) minute(s) and $((SECONDS % 60)) second(s)"