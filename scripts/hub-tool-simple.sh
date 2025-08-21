#!/bin/bash

# Simple hub-tool.sh test - Update values-digest.yaml with real SHA256 digests from Docker Hub

set -euo pipefail

print_info() { echo -e "\033[0;34m[INFO]\033[0m $1" >&2; }
print_success() { echo -e "\033[0;32m[SUCCESS]\033[0m $1" >&2; }
print_error() { echo -e "\033[0;31m[ERROR]\033[0m $1" >&2; }

get_image_digest() {
    local repository="$1"
    local tag="$2"
    
    print_info "Fetching digest for ${repository}:${tag}"
    
    # Get Docker Hub token with timeout
    local token
    token=$(timeout 30 curl -s "https://auth.docker.io/token?service=registry.docker.io&scope=repository:${repository}:pull" 2>/dev/null | jq -r '.token' 2>/dev/null || echo "")
    
    if [[ -z "$token" || "$token" == "null" ]]; then
        print_error "Failed to get Docker Hub token for ${repository}"
        return 1
    fi
    
    # Get digest from manifest with timeout
    local digest
    digest=$(timeout 30 curl -s -I \
        -H "Authorization: Bearer ${token}" \
        -H "Accept: application/vnd.docker.distribution.manifest.v2+json" \
        "https://registry-1.docker.io/v2/${repository}/manifests/${tag}" 2>/dev/null | \
        grep -i "docker-content-digest" | awk '{print $2}' | tr -d '\r' || echo "")
    
    if [[ -n "$digest" ]]; then
        echo "$digest"
    else
        print_error "No digest found for ${repository}:${tag}"
        return 1
    fi
}

update_webmodeler_subcomponent() {
    local values_file="$1"
    local digest_file="$2"
    local subcomponent="$3"
    
    print_info "Processing webModeler subcomponent: ${subcomponent}"
    
    # Get repository from digest file
    local repository
    repository=$(yq eval ".webModeler.${subcomponent}.image.repository" "$digest_file" 2>/dev/null || echo "")
    
    # Get tag from values file (webModeler uses a shared tag)
    local tag
    tag=$(yq eval ".webModeler.image.tag" "$values_file" 2>/dev/null || echo "")
    if [[ -z "$tag" || "$tag" == "null" ]]; then
        tag=$(yq eval ".global.image.tag" "$values_file" 2>/dev/null || echo "")
    fi
    
    if [[ -n "$repository" && -n "$tag" && "$repository" != "null" && "$tag" != "null" ]]; then
        print_info "Found webModeler.${subcomponent}: ${repository}:${tag}"
        if digest=$(get_image_digest "$repository" "$tag"); then
            yq eval ".webModeler.${subcomponent}.image.digest = \"${digest}\"" -i "$digest_file"
            print_success "Updated webModeler.${subcomponent} with digest: ${digest}"
            return 0
        else
            print_error "Failed to get digest for webModeler.${subcomponent}"
            return 1
        fi
    else
        print_error "Missing repository or tag for webModeler.${subcomponent} (repo='${repository}', tag='${tag}')"
        return 1
    fi
}

update_component_digest() {
    local values_file="$1"
    local digest_file="$2" 
    local component="$3"
    
    print_info "Processing component: ${component}"
    
    # Special handling for webModeler nested structure
    if [[ "$component" == "webModeler" ]]; then
        local webmodeler_success=0
        local webmodeler_total=0
        
        # Get webModeler subcomponents
        local subcomponents=()
        while IFS= read -r subcomponent; do
            if [[ -n "$subcomponent" ]]; then
                subcomponents+=("$subcomponent")
            fi
        done < <(yq eval '.webModeler | keys | .[]' "$digest_file" 2>/dev/null || echo "")
        
        for subcomponent in "${subcomponents[@]}"; do
            ((webmodeler_total++))
            if update_webmodeler_subcomponent "$values_file" "$digest_file" "$subcomponent"; then
                ((webmodeler_success++))
            fi
        done
        
        if [[ $webmodeler_success -eq $webmodeler_total && $webmodeler_total -gt 0 ]]; then
            print_success "Updated all webModeler subcomponents (${webmodeler_success}/${webmodeler_total})"
            return 0
        else
            print_error "Failed to update some webModeler subcomponents (${webmodeler_success}/${webmodeler_total})"
            return 1
        fi
    fi
    
    # Standard handling for flat components
    # Get repository - try digest file first, then values file
    local repository
    repository=$(yq eval ".${component}.image.repository" "$digest_file" 2>/dev/null || echo "")
    if [[ -z "$repository" || "$repository" == "null" ]]; then
        repository=$(yq eval ".${component}.image.repository" "$values_file" 2>/dev/null || echo "")
    fi
    
    # Get tag from values file
    local tag
    tag=$(yq eval ".${component}.image.tag" "$values_file" 2>/dev/null || echo "")
    if [[ -z "$tag" || "$tag" == "null" ]]; then
        tag=$(yq eval ".global.image.tag" "$values_file" 2>/dev/null || echo "")
    fi
    
    if [[ -n "$repository" && -n "$tag" && "$repository" != "null" && "$tag" != "null" ]]; then
        print_info "Found ${component}: ${repository}:${tag}"
        if digest=$(get_image_digest "$repository" "$tag"); then
            yq eval ".${component}.image.digest = \"${digest}\"" -i "$digest_file"
            print_success "Updated ${component} with digest: ${digest}"
            return 0
        else
            print_error "Failed to get digest for ${component}"
            return 1
        fi
    else
        print_error "Missing repository or tag for ${component} (repo='${repository}', tag='${tag}')"
        return 1
    fi
}

main() {
    local chart_dir="$1"
    local values_file="${chart_dir}/values.yaml"
    local digest_file="${chart_dir}/values-digest.yaml"
    
    print_info "Processing chart directory: ${chart_dir}"
    
    if [[ ! -f "$values_file" || ! -f "$digest_file" ]]; then
        print_error "Missing values.yaml or values-digest.yaml in ${chart_dir}"
        exit 1
    fi
    
    # Create backup
    cp "$digest_file" "${digest_file}.backup"
    
    # Get all components from digest file
    local components=()
    while IFS= read -r component; do
        if [[ -n "$component" ]]; then
            components+=("$component")
        fi
    done < <(yq eval 'keys | .[]' "$digest_file")
    
    print_info "Found components: ${components[*]}"
    
    local success_count=0
    local total_count=${#components[@]}
    
    # Temporarily disable errexit for component processing
    set +e
    
    for component in ${components[@]}; do
        print_info "Processing component: ${component}"
        
        # Call update function and capture result
        if update_component_digest "$values_file" "$digest_file" "$component"; then
            ((success_count++))
        else
            print_error "Failed to process component: ${component}"
        fi
    done
    
    # Re-enable errexit
    set -e
    
    print_success "Completed: ${success_count}/${total_count} components updated"
    
    # Only exit with error if no components were updated successfully
    if [[ $success_count -eq 0 ]]; then
        exit 1
    fi
}

if [[ $# -eq 0 ]]; then
    echo "Usage: $0 <chart-directory-path>"
    exit 1
fi

# Check dependencies
for cmd in jq yq curl; do
    if ! command -v "$cmd" &> /dev/null; then
        print_error "$cmd is required but not installed"
        exit 1
    fi
done

main "$@"
