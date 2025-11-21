#!/usr/bin/env bash
set -euo pipefail

# init (only once)

# cd packages/contracts-bedrock
# just clean
# just forge-build --deny-warnings --skip test
# forge script "scripts/deploy/DeployImplementations.s.sol" \
#     --skip "/**/test/**" \
#     --sig "idonotexist()" \
#     --skip-simulation \
#     2>/dev/null || true
# ls forge-artifacts/DeployImplementations.s.sol/DeployImplementations.json
# cd ../..

# op-deployer embedded artifacts (required by op-deployer Go tests)
# echo "==========Packing op-deployer artifacts..."
# just -f op-deployer/justfile copy-contract-artifacts
# echo "==========Artifacts packed."

commands=(
  "go test ./op-acceptance-tests/tests/base -count=1"
  "go test ./op-acceptance-tests/tests/ecotone -count=1"
  "go test ./op-acceptance-tests/tests/interop/contract -count=1"
  "go test ./op-acceptance-tests/tests/interop/loadtest -count=1"
  "go test ./op-acceptance-tests/tests/interop/message -count=1"
  "go test ./op-acceptance-tests/tests/interop/prep -count=1"
  "go test ./op-acceptance-tests/tests/interop/proofs -count=1"
  "go test ./op-acceptance-tests/tests/interop/proofs/withdrawal -count=1"
  "go test ./op-acceptance-tests/tests/interop/reorgs -count=1"
  "go test ./op-acceptance-tests/tests/interop/seqwindow -count=1"
  "go test ./op-acceptance-tests/tests/interop/smoke -count=1"
  "go test ./op-acceptance-tests/tests/interop/sync/multisupervisor_interop -count=1"
  "go test ./op-acceptance-tests/tests/interop/sync/simple_interop -count=1"
  "go test ./op-acceptance-tests/tests/interop/upgrade -count=1"
  "go test ./op-acceptance-tests/tests/interop/upgrade-singlechain -count=1"
  "go test ./op-acceptance-tests/tests/isthmus/erc20_bridge -count=1"
  "go test ./op-acceptance-tests/tests/isthmus/operator_fee -count=1"
  "go test ./op-acceptance-tests/tests/isthmus/pectra -count=1"
  "go test ./op-acceptance-tests/tests/isthmus/withdrawal_root -count=1"
  "go test ./op-acceptance-tests/tests/safeheaddb_clsync -count=1"
  "go test ./op-acceptance-tests/tests/safeheaddb_elsync -count=1"
  "go test ./op-acceptance-tests/tests/sync_tester -count=1"
  "go test ./op-deployer/pkg/deployer/bootstrap -run '^TestImplementations$' -count=1"
  "go test ./op-deployer/pkg/deployer/bootstrap -run '^TestImplementations/mainnet$' -count=1"
  "go test ./op-deployer/pkg/deployer/bootstrap -run '^TestImplementations/sepolia$' -count=1"
  "go test ./op-deployer/pkg/deployer/bootstrap -run '^TestSuperchain$' -count=1"
  "go test ./op-deployer/pkg/deployer/bootstrap -run '^TestSuperchain/mainnet$' -count=1"
  "go test ./op-deployer/pkg/deployer/bootstrap -run '^TestSuperchain/sepolia$' -count=1"
  "go test ./op-deployer/pkg/deployer/integration_test -count=1"
  "go test ./op-deployer/pkg/deployer/manage -run '^TestAddGameType$' -count=1"
  "go test ./op-deployer/pkg/deployer/opcm -run '^TestSetDisputeGameImpl$' -count=1"
  "go test ./op-deployer/pkg/deployer/pipeline -run '^TestInitLiveStrategy_OPCMReuseLogicSepolia$' -count=1"
  "go test ./op-deployer/pkg/deployer/pipeline -run '^TestInitLiveStrategy_OPCMReuseLogicSepolia/embedded_L1_locator_with_standard_intent_types_and_standard_roles$' -count=1"
  "go test ./op-deployer/pkg/deployer/pipeline -run '^TestInitLiveStrategy_OPCMReuseLogicSepolia/tagged_L1_locator_with_standard_intent_types_and_modified_roles$' -count=1"
  "go test ./op-deployer/pkg/deployer/pipeline -run '^TestInitLiveStrategy_OPCMReuseLogicSepolia/tagged_locator_with_custom_intent_type$' -count=1"
  "go test ./op-deployer/pkg/deployer/pipeline -run '^TestInitLiveStrategy_OPCMReuseLogicSepolia/untagged_L1_locator$' -count=1"
  "go test ./op-deployer/pkg/deployer/pipeline -run '^TestPopulateSuperchainState$' -count=1"
  "go test ./op-deployer/pkg/deployer/upgrade/v2_0_0 -run '^TestUpgrader_Upgrade$' -count=1"
  "go test ./op-devstack/example -count=1"
  "go test ./op-devstack/sysgo -run '^TestControlPlane$' -count=1"
  "go test ./op-devstack/sysgo -run '^TestControlPlaneFakePoS$' -count=1"
  "go test ./op-devstack/sysgo -run '^TestSystem$' -count=1"
  "go test ./op-e2e/actions/altda -count=1"
  "go test ./op-e2e/actions/batcher -count=1"
  "go test ./op-e2e/actions/derivation -count=1"
  "go test ./op-e2e/actions/helpers -count=1"
  "go test ./op-e2e/actions/interop -count=1"
  "go test ./op-e2e/actions/proofs -count=1"
  "go test ./op-e2e/actions/proposer -count=1"
  "go test ./op-e2e/actions/safedb -count=1"
  "go test ./op-e2e/actions/sequencer -count=1"
  "go test ./op-e2e/actions/sgt -count=1"
  "go test ./op-e2e/actions/sync -count=1"
  "go test ./op-e2e/actions/upgrades -count=1"
  "go test ./op-e2e/e2eutils -count=1"
  "go test ./op-e2e/faultproofs -count=1"
  "go test ./op-e2e/interop -count=1"
  "go test ./op-e2e/opgeth -count=1"
  "go test ./op-e2e/system/altda -count=1"
  "go test ./op-e2e/system/bridge -count=1"
  "go test ./op-e2e/system/conductor -count=1"
  "go test ./op-e2e/system/contracts -count=1"
  "go test ./op-e2e/system/da -count=1"
  "go test ./op-e2e/system/fees -count=1"
  "go test ./op-e2e/system/fjord -count=1"
  "go test ./op-e2e/system/isthmus -count=1"
  "go test ./op-e2e/system/p2p -count=1"
  "go test ./op-e2e/system/proofs -count=1"
  "go test ./op-e2e/system/runcfg -count=1"
  "go test ./op-e2e/system/verifier -count=1"
  "go test ./op-validator/pkg/validations -count=1"
)

for cmd in "${commands[@]}"; do
  echo "\n>>> $cmd"
  eval "$cmd"
done
