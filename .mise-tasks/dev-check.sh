#!/usr/bin/env bash

#MISE description="Developers' local tests"
#MISE alias="dc"

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

echo "Current branch: $(git rev-parse --abbrev-ref HEAD)" >&2
if [ -n "$(git status --porcelain)" ]; then
  echo "WARN: Working tree not clean. Commit/stash changes first." >&2
  git status --porcelain
  exit 1
fi

echo "==========Checking environment done"

# Updating dependencies in contracts lib
pushd packages/contracts-bedrock > /dev/null
forge install

# contracts-bedrock-build
just clean && just forge-build --deny-warnings --skip test

# contracts-bedrock-checks
for cmd in \
    check-kontrol-summaries-unchanged \
    semgrep-test-validity-check \
    semgrep \
    semver-lock-no-build \
    semver-diff-check-no-build \
    validate-deploy-configs \
    lint \
    snapshots-check-no-build \
    interfaces-check-no-build \
    reinitializer-check-no-build \
    size-check \
    unused-imports-check-no-build \
    validate-spacers-no-build \
    opcm-upgrade-checks-no-build; do
    git reset --hard
    git clean -df
    echo "==========Running just $cmd..."
    just $cmd
    echo "==========just $cmd done"
    git status --porcelain
    [ -z "$(git status --porcelain)" ] || exit 1
done

popd > /dev/null

# diff-fetcher-forge-artifacts
echo "==========Running diff-fetcher-forge-artifacts..."
git reset --hard
git clean -df
pushd op-fetcher > /dev/null
just build-contracts
popd > /dev/null
diff -qr "packages/contracts-bedrock/forge-artifacts/FetchChainInfo.s.sol" \
        "op-fetcher/pkg/fetcher/fetch/forge-artifacts/FetchChainInfo.s.sol"

if [ $? -ne 0 ]; then
  echo "ERROR: The checked-in forge artifacts for FetchChainInfo.s.sol do not match the ci build."
  echo "Please run 'cd op-fetcher && just build-contracts' and commit the changes."
  exit 1
fi

echo "==========diff-fetcher-forge-artifacts check done"

# diff-asterisc-bytecode
echo "==========Running diff-asterisc-bytecode..."
git reset --hard
git clean -df

pushd packages/contracts-bedrock > /dev/null

# Clone asterisc @ the pinned version to fetch remote `RISCV.sol`
ASTERISC_REV="v$(yq '.tools.asterisc' ../../mise.toml)"
REMOTE_ASTERISC_PATH="./src/vendor/asterisc/RISCV_Remote.sol"

git -c advice.detachedHead=false clone https://github.com/ethereum-optimism/asterisc \
  -b "$ASTERISC_REV" \
  ./asterisc

cp ./asterisc/rvsol/src/RISCV.sol "$REMOTE_ASTERISC_PATH"

# Replace import paths
sed -i -e 's/@optimism\///' "$REMOTE_ASTERISC_PATH"
# Replace legacy interface paths
sed -i -e 's/src\/cannon\/interfaces\//interfaces\/cannon\//g' "$REMOTE_ASTERISC_PATH"
sed -i -e 's/src\/dispute\/interfaces\//interfaces\/dispute\//g' "$REMOTE_ASTERISC_PATH"
# Replace contract name
sed -i -e 's/contract RISCV/contract RISCV_Remote/' "$REMOTE_ASTERISC_PATH"

# Install deps
forge install

# Diff bytecode, with both contracts compiled in the local environment.
REMOTE_ASTERISC_CODE="$(forge inspect RISCV_Remote bytecode | tr -d '\n')"
LOCAL_ASTERISC_CODE="$(forge inspect RISCV bytecode | tr -d '\n')"

if [ "$REMOTE_ASTERISC_CODE" != "$LOCAL_ASTERISC_CODE" ]; then
  echo "Asterisc bytecode mismatch. Local version does not match remote. Diff:"
  diff <(echo "$REMOTE_ASTERISC_CODE") <(echo "$LOCAL_ASTERISC_CODE")
  popd > /dev/null
  exit 1
fi
rm -rf ./asterisc
popd > /dev/null
echo "==========diff-asterisc-bytecode check done"

git reset --hard
git clean -df

# semgrep-scan-local
echo "==========Running semgrep-scan-local..."
semgrep scan --timeout=100 --config .semgrep/rules/ --error .
echo "==========semgrep-scan-local done"

# semgrep-test
echo "==========Running semgrep-test..."
semgrep scan --test --config .semgrep/rules/ .semgrep/tests/
echo "==========semgrep-test done"

# go-lint
echo "==========Running go-lint (make lint-go)..."
make lint-go
echo "==========go-lint done"

echo "Execution time: $((SECONDS / 60)) minute(s) and $((SECONDS % 60)) second(s)"