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

run_case "op-deployer/pkg/deployer/pipeline TestPopulateSuperchainState" \
	go test ./op-deployer/pkg/deployer/pipeline -run '^TestPopulateSuperchainState($|/valid_OPCM_address_only$|/OPCM_address_with_SuperchainConfigProxy$|/output_mapping_validation$)'

run_case "op-deployer/pkg/deployer/pipeline TestPopulateSuperchainState_OPCMV2" \
	go test ./op-deployer/pkg/deployer/pipeline -run '^TestPopulateSuperchainState_OPCMV2($|/SuperchainConfigProxy_only$|/output_mapping_validation$)'

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
