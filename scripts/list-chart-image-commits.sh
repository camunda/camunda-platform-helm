#!/bin/bash

#
# List git commits for each Docker image used in the chart.
# Extracts revision information from Docker image labels.
#

set -euo pipefail

# Function to extract git commit from Docker image labels
get_image_commit() {
    local image=$1
    local image_name=$(echo "$image" | cut -d':' -f1 | awk -F'/' '{print $NF}')
    local commit=""
    
    # Try to get the org.opencontainers.image.revision label
    commit=$(docker inspect "$image" 2>/dev/null | jq -r '.[0].Config.Labels["org.opencontainers.image.revision"] // ""' || echo "")
    
    # If commit is empty or null, try alternative label
    if [[ -z "$commit" || "$commit" == "null" ]]; then
        commit="N/A"
    fi
    
    echo "$image_name|$commit"
}

# Function to format output as markdown table
format_as_table() {
    echo ""
    echo "## Docker Image Git Commits"
    echo ""
    echo "| Component | Git Commit |"
    echo "|-----------|------------|"
    
    # Read from stdin and format as table
    while IFS='|' read -r component commit; do
        # Truncate commit to 12 characters if it's a full SHA
        if [[ ${#commit} -gt 12 && "$commit" != "N/A" ]]; then
            commit="${commit:0:12}"
        fi
        echo "| $component | \`$commit\` |"
    done | sort -u
    
    echo ""
}

# Main execution
main() {
    # Check if required tools are available
    for tool in docker jq kubectl helm; do
        if ! command -v "$tool" &> /dev/null; then
            echo "Warning: $tool is not installed. Skipping image commit extraction." >&2
            exit 0
        fi
    done
    
    # Get list of images from the deployed resources in the namespace
    echo "Extracting images from namespace: ${TEST_NAMESPACE:-default}" >&2
    
    images=$(kubectl get pods -n "${TEST_NAMESPACE:-default}" -o jsonpath='{.items[*].spec.containers[*].image}' 2>/dev/null | tr ' ' '\n' | sort -u)
    
    if [[ -z "$images" ]]; then
        echo "Warning: No images found in namespace ${TEST_NAMESPACE:-default}" >&2
        exit 0
    fi
    
    echo "Found $(echo "$images" | wc -l) unique images" >&2
    echo "" >&2
    
    # Process each image
    declare -A processed_images
    for image in $images; do
        # Skip if already processed (same image with different tags)
        image_base=$(echo "$image" | cut -d':' -f1 | awk -F'/' '{print $NF}')
        if [[ -n "${processed_images[$image_base]:-}" ]]; then
            continue
        fi
        processed_images[$image_base]=1
        
        echo "Processing: $image" >&2
        
        # Pull the image if not already available
        if ! docker inspect "$image" &>/dev/null; then
            echo "  Pulling image..." >&2
            if ! docker pull "$image" &>/dev/null; then
                echo "  Warning: Failed to pull image $image" >&2
                echo "$image_base|N/A"
                continue
            fi
        fi
        
        # Extract commit information
        get_image_commit "$image"
    done | format_as_table
}

# Run main function
main
