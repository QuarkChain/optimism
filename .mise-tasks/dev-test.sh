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
    # If sourced, return non-zero; otherwise exit non-zero.
    return 1 2>/dev/null || exit 1
}

run_step() {
    local label="$1"
    shift
    echo "==========Starting ${label}..."
    "$@"
    echo "===================${label} done."
}

skip_step() {
    local label="$1"
    local reason="$2"
    echo "==========Skipping ${label}: ${reason}"
}

require_command() {
    local cmd="$1"
    local help_msg="$2"

    if ! command -v "$cmd" >/dev/null 2>&1; then
        halt "Missing required command: $cmd. ${help_msg}"
    fi
}

install_system_package() {
    local pkg="$1"

    case "$(uname -s)" in
        Darwin)
            if ! command -v brew >/dev/null 2>&1; then
                halt "Homebrew is required to install $pkg on macOS."
            fi
            if [ "$pkg" = "docker" ]; then
                brew install --cask docker
            else
                brew install "$pkg"
            fi
            ;;
        Linux)
            if command -v apt-get >/dev/null 2>&1; then
                apt-get update
                if [ "$pkg" = "docker" ]; then
                    apt-get install -y docker.io
                else
                    apt-get install -y "$pkg"
                fi
            elif command -v dnf >/dev/null 2>&1; then
                if [ "$pkg" = "docker" ]; then
                    dnf install -y docker
                else
                    dnf install -y "$pkg"
                fi
            elif command -v apk >/dev/null 2>&1; then
                if [ "$pkg" = "docker" ]; then
                    apk add --no-cache docker
                else
                    apk add --no-cache "$pkg"
                fi
            else
                halt "No supported package manager found to install $pkg."
            fi
            ;;
        *)
            halt "Unsupported OS for automatic installation: $(uname -s)"
            ;;
    esac
}

require_clang_c_headers() {
    local probe='#include <stdarg.h>
#include <stdbool.h>
int main(void){return 0;}'

    if ! printf "%s\n" "$probe" | clang -x c - -fsyntax-only >/dev/null 2>&1; then
        halt "Missing C toolchain headers for clang (stdarg.h/stdbool.h). Install system deps manually (e.g. Ubuntu/Debian: build-essential clang libc6-dev libclang-dev)."
    fi
}

cleanup_kona_tmp() {
    # Known local artifacts from kona sysgo/action tests.
    rm -f rust/kona/out.bin.gz
    rm -f rust/kona/._*

    if [ -d "rust/kona/tmp" ]; then
        rm -rf rust/kona/tmp
        echo "==========Cleaned rust/kona/tmp"
    fi
}

# Always cleanup kona ephemeral artifacts on script exit.
trap cleanup_kona_tmp EXIT

# Environment verify
echo "==========Checking environment..."
require_command mise "Install mise first so dev-test.sh can provision repo-managed tools."

run_step "mise install" mise install

if [ -z "${MISE_SHELL:-}" ]; then
    if [ -n "${ZSH_VERSION:-}" ]; then
        eval "$(mise activate zsh)"
    elif [ -n "${BASH_VERSION:-}" ]; then
        eval "$(mise activate bash)"
    fi
fi

echo "Current branch: $(git rev-parse --abbrev-ref HEAD)" >&2
# if [ -n "$(git status --porcelain)" ]; then
#   echo "WARN: Working tree not clean. Commit/stash changes first." >&2
#   git status --porcelain
#   exit 1
# fi

# Only check dependencies not managed by mise.toml.
command -v m4 >/dev/null 2>&1 || install_system_package m4
command -v m4 >/dev/null 2>&1 || halt "Failed to install m4."

command -v clang >/dev/null 2>&1 || install_system_package clang
command -v clang >/dev/null 2>&1 || halt "Failed to install clang."

command -v docker >/dev/null 2>&1 || install_system_package docker
command -v docker >/dev/null 2>&1 || halt "Failed to install docker."

require_clang_c_headers
command -v cargo-binstall >/dev/null 2>&1 || cargo install cargo-binstall --locked
command -v cargo-nextest >/dev/null 2>&1 || cargo binstall --no-confirm cargo-nextest
if ! cargo nextest --version >/dev/null 2>&1; then
    halt "Failed to install cargo-nextest."
fi

echo "==========Checking environment done"

# # Clean kona temp dir again before running tests.
# cleanup_kona_tmp

# # contracts-bedrock-tests / contracts-bedrock-build (from .circleci/continue/main.yml)
# pushd packages/contracts-bedrock > /dev/null
# forge install

# # run_step "contracts-bedrock tests setup (go-ffi)" just build-go-ffi

# # temporarily skip failed tests that block CI process
# SKIP_PATH="test/universal/OptimismMintableERC20Factory.t.sol"

# for _spec in \
#     "-name '*.t.sol' -not -name 'PreimageOracle.t.sol'" \
#     "-name 'PreimageOracle.t.sol'"; do
#     TEST_FILES=$(eval find test ${_spec})
#     if [ -z "$TEST_FILES" ]; then
#         echo "No tests matched spec: ${_spec}; skipping"
#         continue
#     fi
#     TEST_FILES=$(echo "$TEST_FILES" | sed 's|^test/||')
#     MATCH_PATH="./test/{$(echo "$TEST_FILES" | paste -sd "," -)}"
#     echo "Running forge test --match-path $MATCH_PATH"
#     forge test --match-path "$MATCH_PATH" --no-match-path "$SKIP_PATH"
# done

# run_step "contracts-bedrock build" bash -c "just clean && just forge-build --deny-warnings --skip test"
# popd > /dev/null

# required for kona supervisor sysgo e2e artifact resolution
run_step "op-deployer artifact sync" just -f op-deployer/justfile copy-contract-artifacts

# # cannon-prestate (from .circleci/continue/main.yml)
# run_step "cannon prestate build" make -j reproducible-prestate

# # op-program-compat (from .circleci/continue/main.yml)
# run_step "op-program compatibility" bash -c "cd op-program && make verify-compat"

# # rust-ci functional tests (from .circleci/continue/rust-ci.yml)
# run_step "rust workspace tests" bash -c "cd rust && cargo nextest run --workspace --all-features --no-fail-fast -E '!test(test_online)'"
# run_step "op-reth integration tests" bash -c "cd rust && just --justfile op-reth/justfile test-integration"
# run_step "op-reth edge tests" bash -c "cd rust && just --justfile op-reth/justfile test edge"

# # rust-e2e prerequisites (from .circleci/continue/rust-e2e.yml)
# run_step "rust e2e binary build" bash -c "cd rust && cargo build --release --bin kona-node --bin kona-host --bin kona-supervisor --bin op-reth"

# # Run node/common sysgo e2e across all CI devnet variants.
# for devnet in simple-kona simple-kona-geth simple-kona-sequencer large-kona-sequencer; do
#     run_step "kona sysgo node/common (${devnet})" bash -c "
#         export RUST_BINARY_PATH_KONA_NODE='$(pwd)/rust/target/release/kona-node'
#         export RUST_BINARY_PATH_OP_RETH='$(pwd)/rust/target/release/op-reth'
#         export KONA_NODE_EXEC_PATH='$(pwd)/rust/target/release/kona-node'
#         export OP_RETH_EXEC_PATH='$(pwd)/rust/target/release/op-reth'
#         cd rust/kona && just test-e2e-sysgo-run node node/common ${devnet}
#     "
# done

# # Run node restart recovery scenario on sysgo.
# run_step "kona sysgo node/restart" bash -c "
#     export RUST_BINARY_PATH_KONA_NODE='$(pwd)/rust/target/release/kona-node'
#     export RUST_BINARY_PATH_OP_RETH='$(pwd)/rust/target/release/op-reth'
#     export KONA_NODE_EXEC_PATH='$(pwd)/rust/target/release/kona-node'
#     export OP_RETH_EXEC_PATH='$(pwd)/rust/target/release/op-reth'
#     cd rust/kona && just test-e2e-sysgo-run node node/restart simple-kona
# "

# # Run single-chain proof action tests using kona-host.
# run_step "kona proof action single" bash -c "
#     export RUST_BINARY_PATH_KONA_HOST='$(pwd)/rust/target/release/kona-host'
#     export KONA_HOST_PATH='$(pwd)/rust/target/release/kona-host'
#     # Fix "/run/user/0/just/just-HqgZ35/action-tests-single-run: line 212: cd: /root/dl/optimism/rust/kona/tests/../../op-e2e/actions/proofs: No such file or directory"
#     CREATED_RUST_OP_E2E_LINK=0
#     if [ ! -e 'rust/op-e2e' ]; then
#         ln -s ../op-e2e rust/op-e2e
#         CREATED_RUST_OP_E2E_LINK=1
#     fi
#     cleanup() {
#         if [ \$CREATED_RUST_OP_E2E_LINK -eq 1 ]; then
#             rm -f rust/op-e2e
#         fi
#     }
#     trap cleanup EXIT
#     cd rust/kona && just action-tests-single-run
# "

# Run kona supervisor sysgo e2e suites.
ROOT_DIR="$(pwd)"
OP_DEPLOYER_ARTIFACTS_DIR="$ROOT_DIR/rust/kona/tmp/op-deployer-artifacts"
mkdir -p "$OP_DEPLOYER_ARTIFACTS_DIR"
rm -rf "$OP_DEPLOYER_ARTIFACTS_DIR/src" "$OP_DEPLOYER_ARTIFACTS_DIR/forge-artifacts"
ln -s "$ROOT_DIR/packages/contracts-bedrock/forge-artifacts" "$OP_DEPLOYER_ARTIFACTS_DIR/src"
ln -s "$ROOT_DIR/packages/contracts-bedrock/forge-artifacts" "$OP_DEPLOYER_ARTIFACTS_DIR/forge-artifacts"

export OP_DEPLOYER_ARTIFACTS="$OP_DEPLOYER_ARTIFACTS_DIR"
export RUST_BINARY_PATH_KONA_SUPERVISOR="$ROOT_DIR/rust/target/release/kona-supervisor"
export KONA_SUPERVISOR_EXEC_PATH="$ROOT_DIR/rust/target/release/kona-supervisor"

for supervisor_pkg in /supervisor/pre_interop /supervisor/l1reorg/sysgo; do
    run_step "kona supervisor e2e (${supervisor_pkg})" bash -c "
        cd rust/kona && just test-e2e-sysgo supervisor ${supervisor_pkg}
    "
done

# # kona-host-client-offline (adapted from .circleci/continue/rust-ci.yml)
# run_step "kona host/client offline" bash -c '
#     set -euo pipefail

#     ROOT_DIR="$(pwd)"
#     WITNESS_TAR_NAME="holocene-op-sepolia-26215604-witness.tar.zst"

#     export BLOCK_NUMBER=26215604
#     export L2_CLAIM=0x7415d942f80a34f77d344e4bccb7050f14e593f5ea33669d27ea01dce273d72d
#     export L2_OUTPUT_ROOT=0xaa34b62993bd888d7a2ad8541935374e39948576fce12aa8179a0aa5b5bc787b
#     export L2_HEAD=0xf4adf5790bad1ffc9eee315dc163df9102473c5726a2743da27a8a10dc16b473
#     export L1_HEAD=0x010cfdb22eaa13e8cdfbf66403f8de2a026475e96a6635d53c31f853a0e3ae25
#     export L2_CHAIN_ID=11155420

#     cd cannon && make
#     export PATH="$ROOT_DIR/cannon/bin:$PATH"

#     cd "$ROOT_DIR/rust/kona"
#     tar --zstd -xvf "./bin/client/testdata/$WITNESS_TAR_NAME" -C .

#     cd "$ROOT_DIR/rust/kona/bin/client"
#     just run-client-cannon-offline \
#         "$BLOCK_NUMBER" \
#         "$L2_CLAIM" \
#         "$L2_OUTPUT_ROOT" \
#         "$L2_HEAD" \
#         "$L1_HEAD" \
#         "$L2_CHAIN_ID"
# '

echo "Execution time: $((SECONDS / 60)) minute(s) and $((SECONDS % 60)) second(s)"
