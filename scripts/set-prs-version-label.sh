# A script to label PRs based on the app and chart version.
# It meant to run in the post-release workflow and before merging the release PR.

#!/bin/bash
set -euo pipefail

# Init.
# Check dependencies.
dep_names="ct gh git git-cliff grep jq yq"
for dep_name in ${dep_names}; do
    test -n "$(which ${dep_name})" || (
      echo "Missing dependency: ${dep_name}";
      echo "Dependencies list: ${dep_names}";
      exit 1
    )
done
# Ensure that the main branch is there.
test "$(git branch --show-current)" != "main" &&
    git fetch origin main:main --no-tags

# Vars.
release_please_config=".github/config/release-please/release-please-config.json"
latest_release_commit="$(git show main:${release_please_config} | jq -r '."bootstrap-sha"')"
cliff_config_file=".github/config/cliff.toml"

# Get PRs from commits in a certain chart dir.
get_prs_per_chart_dir () {
    chart_dir="${1}"
    git-cliff --context \
        --config "${cliff_config_file}" \
        --include-path "${chart_dir}/**" \
        ${latest_release_commit}.. |
            jq '.[].commits[].message' | grep -Po '(?<=#)\d+'
}

# Get the issues fixed by a PR.
# Note: GH CLI doesn't support this so we use the API call directly.
# https://github.com/cli/cli/discussions/7097#discussioncomment-5229031
get_issues_per_pr () {
    pr_nubmer="${1}"
    gh api graphql -F owner='camunda' -F repo='camunda-platform-helm' -F pr="${pr_nubmer}" -f query='
        query ($owner: String!, $repo: String!, $pr: Int!) {
            repository(owner: $owner, name: $repo) {
                pullRequest(number: $pr) {
                    closingIssuesReferences(first: 100) {
                        nodes {
                            number
                        }
                    }
                }
            }
        }' --jq '.data.repository.pullRequest.closingIssuesReferences.nodes[].number'
}

# Run.
# Label PRs with the app and chart version only if there is a change in the chart.
ct list-changed | while read chart_dir; do
    app_version="$(yq '.appVersion | sub("\..$", "")' ${chart_dir}/Chart.yaml)"
    chart_version="$(yq '.version' ${chart_dir}/Chart.yaml)"
    app_version_label="version/${app_version}"
    # The "version:x.y.z" label format is used by the support team,
    # it must not changed without checking with the support team.
    chart_version_label="version:${chart_version}"

    echo -e "\nChart dir: ${chart_dir}"
    echo "Apps version: ${app_version}"
    echo "Chart version: ${chart_version}"
    get_prs_per_chart_dir "${chart_dir}" | while read pr_nubmer; do
        # Label PR.
        gh pr edit "${pr_nubmer}" --add-label "${app_version_label},${chart_version_label}"
        # Label the PR's corresponding issue.
        # TODO: Update the logic to work with multi issues (usually we don't have that).
        issue_nubmer="$(get_issues_per_pr ${pr_nubmer})"
        test -n "${issue_nubmer}" &&
            gh issue edit "${issue_nubmer}" --add-label "${app_version_label},${chart_version_label}"
    done
done
