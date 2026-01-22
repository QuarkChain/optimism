#!/usr/bin/env bash

set -uo pipefail

# Re-run failed cases extracted from test.log (explicit list)

failures=()

run_case() {
	local label="$1"
	shift
	echo "==== Running: $label"
	if ! "$@"; then
		failures+=("$label")
	fi
}

# export SEPOLIA_RPC_URL=https://sepolia.drpc.org

run_case "op-e2e/system/conductor TestSequencerFailover_DisasterRecovery_OverrideLeader" \
	go test -v ./op-e2e/system/conductor -run '^TestSequencerFailover_DisasterRecovery_OverrideLeader$'

run_case "op-e2e/system/da TestBatcherAutoDA" \
	go test -v ./op-e2e/system/da -run '^TestBatcherAutoDA$'

run_case "op-devstack/sysgo TestControlPlane/test-SupervisorRestart" \
	go test -v ./op-devstack/sysgo -run '^TestControlPlane/test-SupervisorRestart$'

if [[ ${#failures[@]} -gt 0 ]]; then
	echo ""
	echo "==== Failed cases:"
	for f in "${failures[@]}"; do
		echo "- $f"
	done
	exit 1
fi

echo ""
echo "All cases passed."
