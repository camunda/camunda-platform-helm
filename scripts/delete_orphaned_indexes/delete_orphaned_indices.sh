#!/bin/bash

indices="$( ./scripts/delete_orphaned_indexes/find-orphaned-es-indexes.sh )"
parallel --will-cite -j30 ./scripts/delete_orphaned_indexes/sendrequest.sh ::: $indices
