# Diagram: High Availability Architectures — Tier Selection

Three named tiers replace ad-hoc multi-region patterns. Choose based on RTO/RPO requirements and platform constraints.

```mermaid
flowchart TD
    START(["What are your recovery\nrequirements?"])

    Q1{"Manual recovery\nin 1–4 hours OK?"}
    Q2{"Need scripted failover\nwith ~15 min RTO\nand zero data loss?"}

    T1["**Tier 1 — Cold recovery**\n\nBackup-based DR\nRTO: 1–4 h\nRPO: last backup\n\nStorage: RDBMS or ES/OS\nPlatforms: all\nCost: lowest\n\nBest for: internal tooling, non-customer-facing\nworkloads, batch processing, dev/staging"]

    T2["**Tier 2 — Dual-region hot standby**\n\nHA + DR\nRTO: ~15 min\nRPO: 0\n\nStorage: RDBMS only\nPlatforms: EKS · OpenShift · ROSA\n(ECS: coming 8.10.x)\n\nBest for: regulated SaaS, customer-facing\nautomation, payments, insurance"]

    T3["**Tier 3 — Three-region stretched**\n\nAutomatic quorum-based HA\nRTO: near-zero\nRPO: 0\n\nStorage: RDBMS only\nPlatforms: Kubernetes/Helm only\nRTO/RPO targets: TBD (camunda-docs#8492)\n\nBest for: banking, government, mission-critical\norchestration"]

    START --> Q1
    Q1 -->|"Yes"| T1
    Q1 -->|"No"| Q2
    Q2 -->|"Yes"| T2
    Q2 -->|"No — need near-zero\nautomatic failover"| T3
```

## Comparison table

| | Tier 1 | Tier 2 | Tier 3 |
|---|---|---|---|
| Name | Cold recovery | Dual-region hot standby | Three-region stretched |
| RTO | 1–4 h | ~15 min | near-zero |
| RPO | last backup | 0 | 0 |
| Regions | 1 (+ restore target) | 2 | 3 |
| Failover | Manual | Scripted | Automatic (quorum) |
| Standby cost | None | Active standby | Two standbys |
| Storage | RDBMS or ES/OS | RDBMS only | RDBMS only |
| Platforms (8.10 GA) | All | EKS, OpenShift, ROSA | Kubernetes/Helm |
| Operational complexity | Low | Medium | High |

> **Note:** If multi-region is on your roadmap, prefer RDBMS for the OC from day one — switching storage backend later requires a migration.
