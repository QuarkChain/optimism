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

# op-deployer/pkg/deployer/integration_test/cli

run_case "op-deployer/pkg/deployer/integration_test/cli TestCLIUpgrade" \
	go test ./op-deployer/pkg/deployer/integration_test/cli -run '^TestCLIUpgrade($|/op-contracts/v2\.0\.0$|/op-contracts/v3\.0\.0$|/op-contracts/v4\.0\.0-rc\.7$|/op-contracts/v4\.1\.0$|/op-contracts/v5\.0\.0$)'

# op-deployer/pkg/deployer/pipeline

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
