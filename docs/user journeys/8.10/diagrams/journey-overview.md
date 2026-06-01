# Diagram: 8.10 Administrator Journey Overview

The sidebar is action-ordered to a production baseline, then branches into reference and operations.

```mermaid
flowchart LR
    QS(["Quickstart\n(dev / admin)\nnot prod-shaped"])

    subgraph DTP["Deploy to production"]
        direction TB
        subgraph PLAN["Plan your deployment"]
            direction TB
            pDT["Choose deployment target"]
            pRT["Choose resilience tier"]
            pDB["Choose database strategy"]
            pID["Choose identity strategy"]
            pDT --> pRT --> pDB --> pID
        end
        subgraph BASE["Deploy your baseline"]
            direction TB
            b1["Kubernetes — Cloud providers\n(EKS / GKE / AKS / OpenShift)"]
            b2["Containers (Docker / ECS)"]
            b3["Manual / VM"]
        end
        bPROD["Production readiness checklist"]
        PLAN --> BASE --> bPROD
    end

    subgraph ANC["Architecture and concepts\n(reference — consult in any order)"]
        direction TB
        aRA["Reference architectures"]
        aHUB["Hub / OC topology"]
        aTEN["Tenancy and isolation\n(Logical / Physical Tenant / Separate OC)"]
        aADR["Availability & DR\n(Tier 1 / 2 / 3)"]
        aANA["Process analytics"]
        aDB["Databases"]
        aAUT["Authentication"]
    end

    subgraph RNM["Run and maintain"]
        direction TB
        dUPG["Upgrades"]
        dBAK["Backups & restore"]
        dMON["Monitoring"]
        dSCA["Scaling"]
        dCRED["Credentials rotation"]
        dTRB["Diagnostics & troubleshooting"]
    end

    MIG(["Migrate 8.9 → 8.10\n(once-per-version)"])
    COMP(["Components\n(cross-reference)"])

    QS -.->|"not prod-shaped"| BASE
    DTP -.->|"consult as needed"| ANC
    DTP --> RNM
    MIG -->|"forced changes\n(Helm v4, Bitnami, Hub values)"| DTP
    MIG -.->|"post-upgrade optional"| ANC
```

## 8.9 vs 8.10 sidebar shape

| 8.9 | 8.10 |
|---|---|
| Categories to browse | Action path to baseline, then reference |
| Quickstart · Ref Arch · Deploy & manage · Concepts · Components · Upgrade | Quickstart · Deploy to production · Architecture and concepts · Run and maintain · Components · Migrate |
| Backup & restore buried under Concepts | Backup & restore top-level Run and maintain entry |
| Multi-region buried as one Concepts page | First-class Availability & DR with Tier 1 / 2 / 3 |
| Deployment target choice implicit | Deployment target chosen explicitly in Plan your deployment |
| "Concepts" mixes definitions with operations | Concepts → Architecture and concepts; Operations → Run and maintain |
