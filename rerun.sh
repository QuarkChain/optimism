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

# op-e2e/actions/interop

run_case "op-e2e/actions/interop TestExecMsgDifferTxIndex" \
	go test ./op-e2e/actions/interop -run '^TestExecMsgDifferTxIndex$'

# op-deployer/pkg/deployer/integration_test/cli

run_case "op-deployer/pkg/deployer/integration_test/cli TestCLIBootstrap" \
	go test ./op-deployer/pkg/deployer/integration_test/cli -run '^TestCLIBootstrap($|/bootstrap_superchain$|/bootstrap_superchain_with_custom_protocol_versions$|/bootstrap_superchain_paused$|/bootstrap_implementations$)'

run_case "op-deployer/pkg/deployer/integration_test/cli TestCLIEndToEndApply" \
	go test ./op-deployer/pkg/deployer/integration_test/cli -run '^TestCLIEndToEndApply$'

run_case "op-deployer/pkg/deployer/integration_test/cli TestManageAddGameTypeV2_CLI" \
	go test ./op-deployer/pkg/deployer/integration_test/cli -run '^TestManageAddGameTypeV2_CLI($|/missing_required_flag_--config$|/invalid_config_file_path$|/invalid_JSON_config_file$|/config_file_missing_required_fields$)'

run_case "op-deployer/pkg/deployer/integration_test/cli TestManageAddGameTypeV2_Integration" \
	go test ./op-deployer/pkg/deployer/integration_test/cli -run '^TestManageAddGameTypeV2_Integration$'

run_case "op-deployer/pkg/deployer/integration_test/cli TestCLIApplyNoOp" \
	go test ./op-deployer/pkg/deployer/integration_test/cli -run '^TestCLIApplyNoOp$'

run_case "op-deployer/pkg/deployer/integration_test/cli TestCLIUpgrade" \
	go test ./op-deployer/pkg/deployer/integration_test/cli -run '^TestCLIUpgrade($|/op-contracts/v2\.0\.0$|/op-contracts/v3\.0\.0$|/op-contracts/v4\.0\.0-rc\.7$|/op-contracts/v4\.1\.0$|/op-contracts/v5\.0\.0$)'

run_case "op-deployer/pkg/deployer/integration_test/cli TestCLIVerify" \
	go test ./op-deployer/pkg/deployer/integration_test/cli -run '^TestCLIVerify$'

# op-deployer/pkg/deployer/bootstrap

run_case "op-deployer/pkg/deployer/bootstrap TestImplementations" \
	go test ./op-deployer/pkg/deployer/bootstrap -run '^TestImplementations($|/mainnet$|/sepolia$)'

run_case "op-deployer/pkg/deployer/bootstrap TestSuperchain" \
	go test ./op-deployer/pkg/deployer/bootstrap -run '^TestSuperchain($|/mainnet$|/sepolia$)'

# op-deployer/pkg/deployer/manage

run_case "op-deployer/pkg/deployer/manage TestAddGameType" \
	go test ./op-deployer/pkg/deployer/manage -run '^TestAddGameType$'

# op-deployer/pkg/deployer/integration_test

run_case "op-deployer/pkg/deployer/integration_test TestEndToEndBootstrapApply" \
	go test ./op-deployer/pkg/deployer/integration_test -run '^TestEndToEndBootstrapApply$'

run_case "op-deployer/pkg/deployer/integration_test TestEndToEndApply" \
	go test ./op-deployer/pkg/deployer/integration_test -run '^TestEndToEndApply$'

run_case "op-deployer/pkg/deployer/integration_test TestEndToEndBootstrapApplyWithUpgrade" \
	go test ./op-deployer/pkg/deployer/integration_test -run '^TestEndToEndBootstrapApplyWithUpgrade($|/default$|/opcm-v2$)'

# op-e2e/actions/batcher

run_case "op-e2e/actions/batcher TestL2BatcherBatchType" \
	go test ./op-e2e/actions/batcher -run '^TestL2BatcherBatchType($|/BigL2Txs_SpanBatch$)'

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
