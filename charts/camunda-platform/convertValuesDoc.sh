# !/bin/bash

set -euo pipefail

# reads the documentation of the values file and prints them in a table format, so we can easier copy them to the README
lines=$(grep "# " values.yaml)

while IFS= read -r line; do

  if [[ "$line" =~ ^\#[[:blank:]][[:alpha:]]+' configuration' ]]
  then
    # create new table
    echo "\n"
    echo "| Section | Parameter | Description | Default |"
    echo "|-|-|-|-|"
    echo "$line" | awk '{ORS=""; print "| `"; $2=tolower(substr($2,0,1))substr($2,2); print $2 "` | |"; $1=$2=""; $3=toupper(substr($3, 0, 1))substr($3,2); ORS="\n"; print $0 " | |"}'
  else
    echo "$line" | awk '{ORS=""; print "| | `"; $2=tolower(substr($2,0,1))substr($2,2); print $2 "` |"; $1=$2=""; $3=toupper(substr($3, 0, 1))substr($3,2); ORS="\n"; print $0 " | |"}'
  fi
done <<< "$lines"

