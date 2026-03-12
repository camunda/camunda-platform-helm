#!/bin/bash

response=$(curl -s -o /dev/null -w "%{http_code}" ${ES_AUTH} -X DELETE "${ES_URL}/$1")

if [ "$response" -eq 200 ]; then
  echo "  -> Deleted successfully"
else
  echo "  -> FAILED (HTTP ${response})"
fi
