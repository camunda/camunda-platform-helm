# Transform `deploy-camunda matrix list --format json` output into the YAML
# block format that generate-chart-matrix.sh appends to matrix_versions.txt.
#
# Inputs (via --arg / --argjson):
#   $version           — current chart version (e.g. "8.10")
#   $version_prev      — previous chart version (e.g. "8.9")
#   $manual_scenario   — exact scenario name to keep, or "none"/"all"/""
#   $manual_flow       — comma-separated flow override, or "none"/""
#   $permitted_flows   — parsed .github/config/permitted-flows.yaml (object)
#
# CLI emits one entry per (scenario, flow, platform). This filter groups
# back to one YAML block per (scenario, flow), folding `.platform` into a
# CSV `platforms:` field and `.infraType` into per-platform legacy fields.
# Permitted-flows filter is re-applied here because $manual_flow can inject
# flows that CLI's pre-filter never saw.

def cmp_version($a; $b):
  ($a | split(".") | map(tonumber)) as $ap
  | ($b | split(".") | map(tonumber)) as $bp
  | if $ap[0] != $bp[0] then ($ap[0] - $bp[0])
    elif (($ap | length) > 1 and ($bp | length) > 1) then ($ap[1] - $bp[1])
    else 0
    end;

def match_version_expr($expr; $version):
  ($expr | capture("(?<op>(<=|>=|==|<|>))?\\s*(?<target>\\d+\\.\\d+)")) as $m
  | (if ($m.op // "") == "" then "==" else $m.op end) as $op
  | cmp_version($version; $m.target) as $c
  | if   $op == "==" then $c == 0
    elif $op == "<=" then $c <= 0
    elif $op == ">=" then $c >= 0
    elif $op == "<"  then $c <  0
    elif $op == ">"  then $c >  0
    else false end;

def is_flow_denied($pf; $version; $flow):
  any(($pf.rules // [])[]; match_version_expr(.match; $version) and ((.deny // []) | index($flow) != null));

def apply_manual_scenario($ms):
  if $ms == "none" or $ms == "all" or $ms == "" then .
  elif .scenario == $ms then .
  else empty
  end;

def apply_manual_flow($mf):
  if $mf == "none" or $mf == "" then .
  elif ($mf | test("^(install|upgrade-patch|upgrade-minor)(,(install|upgrade-patch|upgrade-minor))*$")) then
    . as $e | ($mf | split(",")) | map($e + {flow: .}) | .[]
  else
    error("Invalid flow '\($mf)'. Valid flows: install, upgrade-patch, upgrade-minor. modular-upgrade-minor only via integration-test-template.yaml.")
  end;

# Special-case scenario+flow exclusions (preserved from legacy bash logic):
# - keycloak-original / keycloak-mt + upgrade-patch: released chart templates
#   don't support custom realm bootstrapping.
# - oidc + upgrade-minor: requires Entra client setup not yet configured.
def apply_skip_filter:
  if ((.flow == "upgrade-patch") and (.scenario == "keycloak-original" or .scenario == "keycloak-mt"))
     or ((.flow == "upgrade-minor") and (.scenario == "oidc"))
  then empty
  else .
  end;

def yaml_block($version; $version_prev):
  .[0] as $e
  | (map(.platform // empty) | unique | map(select(. != "")) | join(",")) as $platforms
  | (if $platforms == "" then "gke" else $platforms end) as $platforms
  | ($e.exclude // [] | join("|")) as $exclude_str
  | ($e.features // [] | join(",")) as $features_str
  | (map(select(.platform == "gke") | .infraType) | first // "preemptible") as $infra_gke
  | (map(select(.platform == "eks") | .infraType) | first // "preemptible") as $infra_eks
  | "  - version: \"\($version)\"\n" +
    "    camundaVersionPrevious: \"\($version_prev)\"\n" +
    "    case: pr\n" +
    "    platforms: \($platforms)\n" +
    "    scenario: \($e.scenario)\n" +
    "    shortname: \($e.shortname)\n" +
    "    auth: \($e.auth)\n" +
    "    flow: \($e.flow)\n" +
    "    exclude: \($exclude_str)\n" +
    "    infraTypeGke: \($infra_gke)\n" +
    "    infraTypeEks: \($infra_eks)\n" +
    "    identity: \($e.identity // "")\n" +
    "    persistence: \($e.persistence // "")\n" +
    "    features: \($features_str)\n" +
    "    qa: \($e.qa // false)\n" +
    "    upgrade: \($e.upgrade // false)\n" +
    "    skipE2E: \($e.skipE2E // false)\n" +
    "    helmVersion: \"\($e.helmVersion // "")\"";

# Tag each input entry with its original CLI position so group_by can be
# re-ordered to preserve first-occurrence sequence (CLI emits per scenario
# in registry-manifest order; bash baseline preserved that order).
[ to_entries[] | (.value + {_order: .key}) ]
| [ .[]
    | apply_manual_scenario($manual_scenario)
    | apply_manual_flow($manual_flow)
    | apply_skip_filter
    | (if is_flow_denied($permitted_flows; $version; .flow) then empty else . end)
  ]
| group_by([.scenario, .shortname, .flow])
| sort_by(map(._order) | min)
| map(yaml_block($version; $version_prev))
| .[]
