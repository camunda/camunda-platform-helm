#!/bin/bash
set -euo pipefail

main () {
    test "$(git branch --show-current)" != "main" && git fetch origin main:main

    chart_dir="${1}"
    chart_file="${chart_dir}/Chart.yaml"
    chart_name="$(yq '.name' ${chart_file})"
    chart_version="$(yq '.version' ${chart_file})"
    chart_version_previous="$(git show main:${chart_file} | yq '.version')"
    chart_tag="${chart_name}-${chart_version}"
    chart_tag_previous="${chart_name}-${chart_version_previous}"

    #
    # Early exit if the tag already exists.
    git tag -l | grep -q "${chart_tag}" &&
        echo "[WARN] The tag ${chart_tag} already exists, nothing to do..."

    #
    # Set Helm CLI version.
    helm_cli_version="$(grep "helm " .tool-versions | cut -d " " -f2)" \
        yq -i '.annotations."camunda.io/helmCLIVersion" = env(helm_cli_version)' "${chart_file}"

    #
    # Generate RELEASE-NOTES.md file (used for Github release notes and ArtifactHub "changes" annotation).
    git-chglog                                      \
        --output "${chart_dir}/RELEASE-NOTES.md"    \
        --tag-filter-pattern "${chart_tag}"         \
        --next-tag "${chart_tag}"                   \
        --path "${chart_dir}"                       \
        "${chart_tag_previous}".."${chart_tag}"

    #
    # Update ArtifactHub "changes" annotation in the Chart.yaml file.
    # https://artifacthub.io/docs/topics/annotations/helm/#supported-annotations
    change_types="$(yq e '.options.commits.filters.Type | join(" ")' .chglog/config.yml)"

    # 
    declare -A kac_map
    kac_map+=(
        ["feat"]=added
        ["refactor"]=changed
        ["fix"]=fixed
    )

    # Workaround to rest with empty literal block which is not possible in yq.
    yq -i '.annotations."artifacthub.io/changes" = "placeholder" | .annotations."artifacthub.io/changes" style="literal"' \
        "${chart_file}"

    # Generate temp file with changes to be merged with the Chart.yaml file.
    artifacthub_changes_tmp="/tmp/changes-for-artifacthub.yaml.tmp"
    echo -e 'annotations:\n  artifacthub.io/changes: |' > "${artifacthub_changes_tmp}"

    for change_type in ${change_types}; do
        change_type_section=$(sed -rn "/^\#+\s${change_type^}/,/^#/p" "${chart_dir}/RELEASE-NOTES.md")
        if [[ -n "${change_type_section}" && "${!kac_map[@]}" =~ "${change_type}" ]]; then
            echo "${change_type_section}" | egrep '^\*' | sed 's/^* //g' | while read commit_message; do
                echo "    - kind: ${kac_map[${change_type}]}"
                echo "      description: \"$(echo ${commit_message} | sed -r "s/ \(.+\)$//")\""
            done >> "${artifacthub_changes_tmp}"
        fi
    done

    cat "${artifacthub_changes_tmp}"

    if [[ $(cat "${artifacthub_changes_tmp}" | wc -l) -eq 1 ]]; then
        echo "[ERROR] Somthing is wrong, no changes detected to generate Artifact Hub changelog."
        exit 1
    fi

    # Merge changes back to the Chart.yaml file.
    # https://mikefarah.gitbook.io/yq/operators/reduce#merge-all-yaml-files-together
    yq eval-all '. as $item ireduce ({}; . * $item )' \
        "${chart_file}" "${artifacthub_changes_tmp}" > \
        /tmp/Chart-with-artifacthub-changes.yaml.tmp
    cat /tmp/Chart-with-artifacthub-changes.yaml.tmp > "${chart_file}"
    rm "${artifacthub_changes_tmp}"
}

release_notes_footer () {
    chart_dir="${1}"
    export CHART_NAME_WITH_VERSION="$(yq '[.name, .version] | join("-")' "${chart_dir}/Chart.yaml")"
    export VERSION_MATRIX_RELEASE_HEADER="false"
    export VERSION_MATRIX_RELEASE_INFO="$(CHART_DIR=${chart_dir} make release.generate-version-matrix-unreleased)"
    echo "\nChart dir: $${chart_dir}";\
    gomplate --file scripts/templates/release-notes/RELEASE-NOTES-FOOTER.md.tpl |
        tee --append "${chart_dir}/RELEASE-NOTES.md"
}

# Parse script input args.
while test -n "${1}"; do
    case "${1}" in
        --main)
          test -n "${2:-}" || (
            echo "[ERROR] Chart dir is needed as an arg for this option";
            exit 1
          )
          main "${2}"
          shift
          ;;
        --footer)
          test -n "${2:-}" || (
            echo "[ERROR] Chart dir is needed as an arg for this option";
            exit 1
          )
          release_notes_footer "${2}"
          shift
          ;;
        *)
          echo "Unsupported option"
          exit 1
          ;;
    esac
    shift

    # Handling exit if no more script args to avoid "unbound variable" error.
    test -z "${1:-}" && exit 0
done
