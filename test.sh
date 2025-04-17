ls -d charts/camunda-platform* | while read chart_dir; do
  camunda_version="$(basename ${chart_dir} | sed 's/camunda-platform-//')"
  # scenario_enabled="$(yq '.integration.scenarios.nightly.multitenancy.enabled' --indent=0 --output-format json ${chart_dir})/test/ci-test-config.yaml"
  # echo "Camunda version: ${camunda_version}"
  # echo "${camunda_version}" >>matrix_versions.txt
  echo $camunda_version
done
