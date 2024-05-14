#!/bin/bash
set -euo pipefail

main () {
    chart_files_to_release="${1:-"charts/*/Chart.yaml"}"

    for chart_file in ${chart_files_to_release}; do
        chart_name=$(grep -Po "(?<=^name: ).+" ${chart_file})
        chart_version=$(grep -Po "(?<=^version: ).+" ${chart_file})
        chart_tag="${chart_name}-${chart_version}"
        chart_path="charts/${chart_name}"

        #
        # Set Helm CLI version.
        helm_cli_version="$(grep "helm" .tool-versions | cut -d " " -f2)" \
            yq -i '.annotations."helm.sh/cliVersion" = env(helm_cli_version)' "${chart_file}"

        #
        # Generate RELEASE-NOTES.md file (used for Github release notes and ArtifactHub "changes" annotation).
        git-chglog                                    \
            --output "${chart_path}/RELEASE-NOTES.md" \
            --tag-filter-pattern "${chart_tag%-*}"    \
            --next-tag "${chart_tag}"                 \
            --path "${chart_path}" "${chart_tag}"

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

        artifacthub_changes_tmp="/tmp/changes-for-artifacthub.yaml.tmp"
        echo -e 'annotations:\n  artifacthub.io/changes: |' > "${artifacthub_changes_tmp}"

        for change_type in ${change_types}; do
            change_type_section=$(sed -rn "/^\#+\s${change_type^}/,/^#/p" "${chart_path}/RELEASE-NOTES.md")
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
            ${chart_path}/Chart.yaml "${artifacthub_changes_tmp}" > \
            /tmp/Chart-with-artifacthub-changes.yaml.tmp
        cat /tmp/Chart-with-artifacthub-changes.yaml.tmp > ${chart_path}/Chart.yaml
        rm "${artifacthub_changes_tmp}"
    done
}

release_notes_footer () {
    export VERSION_MATRIX_RELEASE_HEADER="false"
    export VERSION_MATRIX_RELEASE_INFO="$(make release.generate-version-matrix-unreleased)"
    gomplate --file scripts/templates/release-notes/RELEASE-NOTES-FOOTER.md.tpl |
        tee --append charts/camunda-platform/RELEASE-NOTES.md
}

# Parse script input args.
while test -n "${1}"; do
    case "${1}" in
        --main)
          main
          ;;
        --footer)
          release_notes_footer
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
