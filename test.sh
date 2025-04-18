touch matrix_versions.txt
echo "matrix:" >>matrix_versions.txt
ls -d charts/camunda-platform* | while read chart_dir; do
  camunda_version="$(basename ${chart_dir} | sed 's/camunda-platform-//')"

  # skip chart version with "alpha" in the chrt version
  case "$camunda_version" in
  *alpha*)
    continue
    ;;
  esac

  cat >>matrix_versions.txt <<-EOL
  - version: ${camunda_version}
    type: nightly
EOL

  #reads the array from the yaml file
  readarray nightlyScenarios < <(yq e -o=j -I=0 '.integration.scenarios.nightly.[]' ${chart_dir}/test/ci-test-config.yaml)

  echo "scenarios:" | sed 's/^/    /' >>matrix_versions.txt
  # loop through scenarios and see which one is enabled
  # nightlyScenario is a yaml snippet representing a single entry
  for nightlyScenario in "${nightlyScenarios[@]}"; do
    enabled=$(echo "$nightlyScenario" | yq e '.enabled' -)
    # skip if scenario is not enabled
    if [ "$enabled" = "false" ]; then
      continue
    fi
    name=$(echo "$nightlyScenario" | yq e '.name' -)
    # indent name so it complies with yaml formatting
    echo - $name | sed 's/^/      /' >>matrix_versions.txt

  done

done

cat matrix_versions.txt
