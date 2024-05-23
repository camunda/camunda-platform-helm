#!/bin/bash

# Description:
# This script performs a Terraform destroy operation for clusters defined in an S3 bucket.
# It copies the Terraform module directory to a temporary location, initializes Terraform with
# the appropriate backend configuration, and runs `terraform destroy`. If the destroy operation
# is successful, it removes the corresponding S3 objects.
#
# Usage:
# ./destroy_clusters.sh <BUCKET> <MODULES_DIR> <TEMP_DIR_PREFIX> <MIN_AGE_IN_HOURS>
#
# Arguments:
#   BUCKET: The name of the S3 bucket containing the cluster state files.
#   MODULES_DIR: The directory containing the Terraform modules.
#   TEMP_DIR_PREFIX: The prefix for the temporary directories created for each cluster.
#   MIN_AGE_IN_HOURS: The minimum age (in hours) of clusters to be destroyed.
#
# Example:
# ./destroy_clusters.sh tf-state-rosa-ci-eu-west-3 ./modules/rosa-hcp/ /tmp/rosa/ 24
#
# Requirements:
# - AWS CLI installed and configured with the necessary permissions to access and modify the S3 bucket.
# - Terraform installed and accessible in the PATH.

# Check for required arguments
if [ "$#" -ne 4 ]; then
  echo "Usage: $0 <BUCKET> <MODULES_DIR> <TEMP_DIR_PREFIX> <MIN_AGE_IN_HOURS>"
  exit 1
fi
# Check if required environment variables are set
if [ -z "$RH_TOKEN" ]; then
  echo "Error: The environment variable RH_TOKEN is not set."
  exit 1
fi

if [ -z "$AWS_REGION" ]; then
  echo "Error: The environment variable AWS_REGION is not set."
  exit 1
fi

# Variables
BUCKET=$1
MODULES_DIR=$2
TEMP_DIR_PREFIX=$3
MIN_AGE_IN_HOURS=$4
HTPASSWD_PASSWORD="Fakepassword!!!3893948" # don't change it, it's a fake value for the destruction
FAILED=0
CURRENT_DIR=$(pwd)

# Function to perform terraform destroy
destroy_cluster() {
  local cluster_id=$1
  local cluster_folder=$2
  local temp_dir="${TEMP_DIR_PREFIX}${cluster_id}"

  echo "Copying $MODULES_DIR in $temp_dir"

  mkdir -p "$temp_dir" || return 1
  cp -a "$MODULES_DIR." "$temp_dir" || return 1

  tree "$MODULES_DIR" "$temp_dir" || return 1

  cd "$temp_dir" || return 1

  tree "." || return 1

  if ! terraform init -backend-config="bucket=$BUCKET" -backend-config="key=${cluster_folder}/${cluster_id}.tfstate" -backend-config="region=$AWS_REGION"; then return 1; fi


  if ! terraform destroy -auto-approve -var "cluster_name=${cluster_id}" -var "htpasswd_password=$HTPASSWD_PASSWORD" -var "offline_access_token=$RH_TOKEN"; then return 1; fi

  # Cleanup S3
  echo "Deleting s3://$BUCKET/$cluster_folder"
  if ! aws s3 rm "s3://$BUCKET/$cluster_folder" --recursive; then return 1; fi
  if ! aws s3api delete-object --bucket "$BUCKET" --key "$cluster_folder/"; then return 1; fi

  cd - || return 1
  rm -rf "$temp_dir" || return 1
}

# List objects in the S3 bucket and parse the cluster IDs
clusters=$(aws s3 ls "s3://$BUCKET/" | awk '{print $2}' | sed -n 's#^tfstate-\(.*\)/$#\1#p')
current_timestamp=$(date +%s)

for cluster_id in $clusters; do
  cd "$CURRENT_DIR" || return 1


  cluster_folder="tfstate-$cluster_id"
  echo "Checking cluster $cluster_id in $cluster_folder"

  last_modified=$(aws s3api head-object --bucket "$BUCKET" --key "$cluster_folder/${cluster_id}.tfstate" --output json | grep LastModified | awk -F '"' '{print $4}')
  if [ -z "$last_modified" ]; then
    echo "Error: Failed to retrieve last modified timestamp for cluster $cluster_id"
    exit 1
  fi

  last_modified_timestamp=$(date -d "$last_modified" +%s)
  if [ -z "$last_modified_timestamp" ]; then
    echo "Error: Failed to convert last modified timestamp to seconds since epoch for cluster $cluster_id"
    exit 1
  fi
  echo "Cluster $cluster_id last modification: $last_modified ($last_modified_timestamp)"

  file_age_hours=$(( ($current_timestamp - $last_modified_timestamp) / 3600 ))
  if [ -z "$file_age_hours" ]; then
    echo "Error: Failed to calculate file age in hours for cluster $cluster_id"
    exit 1
  fi
  echo "Cluster $cluster_id is $file_age_hours hours old"

  if [ $file_age_hours -ge "$MIN_AGE_IN_HOURS" ]; then
    echo "Destroying cluster $cluster_id in $cluster_folder"

    if ! destroy_cluster "$cluster_id" "$cluster_folder"; then
      echo "Error destroying cluster $cluster_id"
      FAILED=1
    fi
  else
    echo "Skipping cluster $cluster_id as it does not meet the minimum age requirement of $MIN_AGE_IN_HOURS hours"
  fi
done

# Exit with the appropriate status
if [ $FAILED -ne 0 ]; then
  echo "One or more operations failed."
  exit 1
else
  echo "All operations completed successfully."
  exit 0
fi
