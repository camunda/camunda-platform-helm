# Diagram: Tenancy & Isolation Model Selection

Three isolation levels. Models can be mixed — e.g., most tenants as Physical Tenants within a shared OC, one regulated tenant as a Separate OC.

```mermaid
flowchart TD
    START(["How isolated do\nyour tenants need to be?"])

    Q1{"Need hard infra boundary?\n(regulatory, independent upgrade\ncadence, or regional isolation)"}
    Q2{"Need per-tenant DB\nschema / index prefix?\n(data separation or perf isolation)"}

    LOG["**Logical tenants**\n\nSoft data isolation\nShared DB, shared OC, shared infra\n\nCost: lowest\nOps: simplest (one Helm value)\nUpgrade: shared cadence\nPer-tenant backup: no\n\nBest for: internal teams, non-regulated\nworkloads, soft data separation"]

    PT["**Physical tenants** *(new in 8.10)*\n\nMedium isolation\nPer-PT DB schema / index prefix\nShared OC, shared Connectors runtime\n\nCost: medium\nOps: moderate (per-PT credentials)\nUpgrade: shared cadence\nPer-tenant backup: yes (REST API)\n\nBest for: per-team perf isolation,\nper-tenant data separation within\na shared platform"]

    OC["**Separate OC**\n\nHard isolation\nFull infra + upgrade independence\nOwn namespace, own DB\n\nCost: highest\nOps: highest (N OC registrations in Hub)\nUpgrade: independent per OC\nPer-tenant backup: yes\n\nBest for: regulatory boundary, different\nrelease cadences, regional separation,\nbusiness-unit hard separation"]

    MIX["Mixing allowed:\ne.g. most tenants as PTs\nin a shared OC; one\nregulated tenant as\na Separate OC"]

    START --> Q1
    Q1 -->|"Yes"| OC
    Q1 -->|"No"| Q2
    Q2 -->|"No"| LOG
    Q2 -->|"Yes"| PT

    PT -.->|"not enough isolation?"| OC
    LOG -.->|"need data separation?"| PT
    PT & OC -.-> MIX
```

## Comparison table

| | Logical tenants | Physical tenants | Separate OC |
|---|---|---|---|
| New in 8.10 | No | **Yes** | No |
| What's isolated | Data (soft, app-level) | DB schema / index prefix | Full infra |
| Infra cost | Lowest | Medium | Highest |
| Per-tenant IdP | No | Yes | Yes |
| Per-tenant backup | No | Yes (REST API) | Yes |
| Per-tenant perf isolation | No | Partial (own partitions) | Full |
| Upgrade cadence | Shared | Shared | Independent |
| Operational complexity | Low | Medium | High |
| Enable via | `global.multitenancy.enabled` | `camunda.physical-tenants.*` | Separate Helm release |
