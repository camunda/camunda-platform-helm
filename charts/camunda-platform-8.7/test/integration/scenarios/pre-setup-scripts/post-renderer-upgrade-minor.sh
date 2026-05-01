#!/bin/bash
#
# Helm post-renderer for the upgrade-minor flow.
# Injects maxUnavailable=100% into the Zeebe broker StatefulSet so all pods
# are replaced simultaneously during CI upgrades instead of one at a time.
# The StatefulSet and PVCs are untouched — data migration is still fully tested.
#

set -euo pipefail

PYFILE=$(mktemp /tmp/post-renderer-XXXXXX.py)
trap 'rm -f "$PYFILE"' EXIT

cat > "$PYFILE" << 'PYEOF'
import sys
import yaml

docs = list(yaml.safe_load_all(sys.stdin))
for doc in docs:
    if (doc is not None
            and doc.get("kind") == "StatefulSet"
            and (doc.get("metadata") or {}).get("labels", {}).get("app.kubernetes.io/component") == "zeebe-broker"):
        (doc
            .setdefault("spec", {})
            .setdefault("updateStrategy", {})
            .setdefault("rollingUpdate", {}))["maxUnavailable"] = "100%"
sys.stdout.write(yaml.dump_all(docs, default_flow_style=False, allow_unicode=True))
PYEOF

python3 "$PYFILE"
