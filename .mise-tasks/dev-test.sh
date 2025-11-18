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
echo "Setting up mise environment..."
mise install

if [ -z "${MISE_SHELL:-}" ]; then
    if [ -n "${ZSH_VERSION:-}" ]; then
        eval "$(mise activate zsh)"
    elif [ -n "${BASH_VERSION:-}" ]; then
        eval "$(mise activate bash)"
    fi
fi
echo "Setting up mise environment done"

for var in SEPOLIA_RPC_URL MAINNET_RPC_URL; do
    if [ -z "${!var}" ]; then
        echo "Error: $var is not set."
        return 0 2>/dev/null || exit 0
    fi
done

# Sync binary from op-es and contracts from merge_op_contracts_v4.1.0

# 1) verify clean tree (including submodules)
if [ -n "$(git status --porcelain)" ]; then
  echo "Working tree not clean. Commit/stash changes first." >&2
  git status --porcelain
  exit 1
fi
# 2) fetch
git fetch origin --prune
# 3) move dl-ci-local to op-es (fast, destructive to prior dl-ci-local commits)
git checkout -B dl-ci-local origin/op-es
# 4) overlay contracts-bedrock from merge_op_contracts_v4.1.0
git restore --source merge_op_contracts_v4.1.0 -- packages/contracts-bedrock
# 5) show status and commit if there are changes
if [ -n "$(git status --porcelain packages/contracts-bedrock)" ]; then
  git add -A packages/contracts-bedrock
  git commit -m "Base from op-es; sync packages/contracts-bedrock from merge_op_contracts_v4.1.0"
else
  echo "No differences in packages/contracts-bedrock vs target branch."
fi
# show summary
printf "\nSync code done! Current HEAD: %s\n"


cd packages/contracts-bedrock

# contracts-bedrock-tests & contracts-bedrock-tests-preimage-oracle
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

# contracts-bedrock-build
just forge-build --deny-warnings --skip test

cd ../..

# Switch to binary branch
git checkout op-es
git pull --ff-only

# cannon-prestate-quick
make cannon-prestates

# go-tests-full
export TEST_TIMEOUT=90m
make go-tests-ci

# op-e2e-fuzz
cd op-e2e && make fuzz && cd ..

# cannon-fuzz
cd cannon && make fuzz && cd ..

# op-program-compat
cd op-program && make verify-compat && cd ..

# fuzz-golang
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

echo "Execution time: $((SECONDS / 60)) minute(s) and $((SECONDS % 60)) second(s)"