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

run_step() {
    local label="$1"
    shift
    echo "==========Starting ${label}..."
    "$@"
    echo "==========${label} done."
}

skip_step() {
    local label="$1"
    local reason="$2"
    echo "==========Skipping ${label}: ${reason}"
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

for cmd in just forge cargo make go gotestsum; do
    if ! command -v "$cmd" >/dev/null 2>&1; then
        halt "Missing required command: $cmd"
    fi
done

if ! command -v cargo-nextest >/dev/null 2>&1; then
    halt "Missing cargo-nextest. Install with: cargo binstall --no-confirm cargo-nextest"
fi

echo "==========Checking environment done"

# contracts-bedrock-tests / contracts-bedrock-build (from .circleci/continue/main.yml)
pushd packages/contracts-bedrock > /dev/null
forge install

run_step "contracts-bedrock tests setup (go-ffi)" just build-go-ffi

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

run_step "contracts-bedrock build" bash -lc "just clean && just forge-build --deny-warnings --skip test"
popd > /dev/null

run_step "op-deployer artifact sync" just -f op-deployer/justfile copy-contract-artifacts

# cannon-prestate (from .circleci/continue/main.yml)
run_step "cannon prestate build" make -j reproducible-prestate

# op-program-compat (from .circleci/continue/main.yml)
run_step "op-program compatibility" bash -lc "cd op-program && make verify-compat"

# rust-ci functional tests (from .circleci/continue/rust-ci.yml)
run_step "rust workspace tests" bash -lc "cd rust && just test"
run_step "op-reth integration tests" bash -lc "cd rust && just --justfile op-reth/justfile test-integration"
run_step "op-reth edge tests" bash -lc "cd rust && just --justfile op-reth/justfile test edge"

# op-reth-compact-codec (from .circleci/continue/rust-ci.yml)
if git rev-parse --verify refs/remotes/origin/develop >/dev/null 2>&1 || git rev-parse --verify develop >/dev/null 2>&1; then
    run_step "op-reth compact codec" bash -lc '
        set -euo pipefail

        if git rev-parse --verify refs/remotes/origin/develop >/dev/null 2>&1; then
            BASE_REF="refs/remotes/origin/develop"
        else
            BASE_REF="develop"
        fi

        CURRENT_REF="$(git rev-parse --abbrev-ref HEAD)"
        if [ "$CURRENT_REF" = "HEAD" ]; then
            CURRENT_REF="$(git rev-parse HEAD)"
        fi

        trap '"'"'git checkout "$CURRENT_REF" >/dev/null 2>&1 || true'"'"' EXIT

        git checkout "$BASE_REF"
        cargo run --bin op-reth --features dev --manifest-path rust/op-reth/bin/Cargo.toml -- test-vectors compact --write

        git checkout "$CURRENT_REF"
        trap - EXIT

        cargo run --bin op-reth --features dev --manifest-path rust/op-reth/bin/Cargo.toml -- test-vectors compact --read
    '
else
    skip_step "op-reth compact codec" "missing local develop or origin/develop ref"
fi

# rust-e2e prerequisites (from .circleci/continue/rust-e2e.yml)
run_step "rust e2e binary build" bash -lc "cd rust && cargo build --release --bin kona-node --bin kona-host --bin kona-supervisor --bin op-reth"

for devnet in simple-kona simple-kona-geth simple-kona-sequencer large-kona-sequencer; do
    run_step "kona sysgo node/common (${devnet})" bash -lc "
        export KONA_NODE_EXEC_PATH='$(pwd)/rust/target/release/kona-node'
        export OP_RETH_EXEC_PATH='$(pwd)/rust/target/release/op-reth'
        cd rust/kona && just test-e2e-sysgo-run node node/common ${devnet}
    "
done

run_step "kona sysgo node/restart" bash -lc "
    export KONA_NODE_EXEC_PATH='$(pwd)/rust/target/release/kona-node'
    export OP_RETH_EXEC_PATH='$(pwd)/rust/target/release/op-reth'
    cd rust/kona && just test-e2e-sysgo-run node node/restart simple-kona
"

run_step "kona proof action single" bash -lc "
    export KONA_HOST_PATH='$(pwd)/rust/target/release/kona-host'
    cd rust/kona && just action-tests-single-run
"

for supervisor_pkg in /supervisor/pre_interop /supervisor/l1reorg/sysgo; do
    run_step "kona supervisor e2e (${supervisor_pkg})" bash -lc "
        cd rust/kona && just test-e2e-sysgo supervisor ${supervisor_pkg}
    "
done

# kona-host-client-offline (adapted from .circleci/continue/rust-ci.yml)
run_step "kona host/client offline" bash -lc '
    set -euo pipefail

    ROOT_DIR="$(pwd)"
    WITNESS_TAR_NAME="holocene-op-sepolia-26215604-witness.tar.zst"

    export BLOCK_NUMBER=26215604
    export L2_CLAIM=0x7415d942f80a34f77d344e4bccb7050f14e593f5ea33669d27ea01dce273d72d
    export L2_OUTPUT_ROOT=0xaa34b62993bd888d7a2ad8541935374e39948576fce12aa8179a0aa5b5bc787b
    export L2_HEAD=0xf4adf5790bad1ffc9eee315dc163df9102473c5726a2743da27a8a10dc16b473
    export L1_HEAD=0x010cfdb22eaa13e8cdfbf66403f8de2a026475e96a6635d53c31f853a0e3ae25
    export L2_CHAIN_ID=11155420

    cd cannon && make
    export PATH="$ROOT_DIR/cannon/bin:$PATH"

    cd "$ROOT_DIR/rust/kona"
    tar --zstd -xvf "./bin/client/testdata/$WITNESS_TAR_NAME" -C .

    cd "$ROOT_DIR/rust/kona/bin/client"
    just run-client-cannon-offline \
        "$BLOCK_NUMBER" \
        "$L2_CLAIM" \
        "$L2_OUTPUT_ROOT" \
        "$L2_HEAD" \
        "$L1_HEAD" \
        "$L2_CHAIN_ID"
'


echo "Execution time: $((SECONDS / 60)) minute(s) and $((SECONDS % 60)) second(s)"
