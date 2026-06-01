# Diagram: 8.10 Component Topology

Hub is the single management plane. Each Orchestration Cluster hosts one or more Physical Tenants. Optimize binds 1:1 to a PT.

```mermaid
graph TD
    HUB["Camunda Hub\n(one per deployment)\nnamespace: camunda-hub"]

    subgraph OC1["Orchestration Cluster 1  —  namespace: camunda-oc-1"]
        PT_DEFAULT["Physical Tenant: default\n(auto-created)"]
        PT_B["Physical Tenant: team-b"]
        PT_N["Physical Tenant: …"]
        CONN["Connectors runtime\n(shared across all PTs in this OC)"]
    end

    OC_N["Orchestration Cluster N\n(separate namespace / infra)"]

    OPT_DEFAULT["Optimize\n(1:1 per PT needing analytics)"]
    OPT_B["Optimize"]

    DB_RDBMS["PostgreSQL / RDBMS\n(Hub state + OC state)\nper-PT schema isolation"]
    DB_ES["Elasticsearch / OpenSearch\n(OC + Optimize indices)\nper-PT index-prefix isolation"]
    IDP["Identity provider\n(Keycloak / Entra ID / OIDC)"]

    HUB -->|"registers OC via values file"| OC1
    HUB -->|"registers OC via values file"| OC_N

    OC1 --- PT_DEFAULT
    OC1 --- PT_B
    OC1 --- PT_N
    OC1 --- CONN

    PT_DEFAULT -->|"if analytics needed"| OPT_DEFAULT
    PT_B -->|"if analytics needed"| OPT_B

    OC1 -->|"per-PT schema / index prefix"| DB_RDBMS
    OC1 -->|"per-PT index prefix\n(ES/OS deployments)"| DB_ES
    HUB -->|"Hub state"| DB_RDBMS

    OPT_DEFAULT -->|"ES/OS only\n(no RDBMS support)"| DB_ES
    OPT_B -->|"ES/OS only"| DB_ES

    HUB -->|"OIDC / Mgmt Identity"| IDP
    OC1 -->|"OC Admin / OIDC"| IDP
```

## Key constraints

| Constraint | Detail |
|---|---|
| Hub count | One Hub per deployment; N OCs register to it |
| Physical Tenant binding | Each OC auto-creates one default PT; add more via `orchestration-values.yaml` |
| Optimize binding | 1:1 per PT — one Optimize instance per PT needing analytics; each needs its own Mgmt Identity app registration |
| Optimize storage | ES/OS only — no RDBMS support, none planned |
| Connectors runtime | One per OC, shared across all PTs in that OC |
| Identity surfaces | Two distinct surfaces: Mgmt Identity (Hub + Optimize) and OC Admin (Zeebe, Operate, Tasklist) |
| Bitnami subcharts | Removed in 8.10 — all dependencies must be provisioned externally before install |
