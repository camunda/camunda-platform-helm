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

    subgraph CON["Concepts\n(explanation — understand a decision)"]
        direction TB
        aHUB["Hub / OC topology"]
        aTEN["Tenancy and isolation\n(Logical / Physical Tenant / Separate OC)"]
        aADR["Availability & DR\n(Tier 1 / 2 / 3)"]
        aANA["Process analytics"]
        aDB["Database strategy"]
        aAUT["Authentication"]
    end

    subgraph REF["Reference\n(look-up — find an exact value)"]
        direction TB
        aRA["Reference architectures"]
        aDBR["Databases\n(Operating ES/OS, RDBMS, Exporters)"]
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
    DTP -.->|"understand a decision"| CON
    DTP -.->|"look something up"| REF
    DTP --> RNM
    MIG -->|"forced changes\n(Helm v4, Bitnami, Hub values)"| DTP
    MIG -.->|"post-upgrade optional"| CON
```

## 8.9 vs 8.10 sidebar shape

| 8.9 | 8.10 |
|---|---|
| Categories to browse | Action path to baseline, then explanation / look-up split |
| Quickstart · Ref Arch · Deploy & manage · Concepts · Components · Upgrade | Quickstart · Deploy to production · Concepts · Reference · Run and maintain · Components · Migrate |
| Backup & restore buried under Concepts | Backup & restore top-level Run and maintain entry |
| Multi-region buried as one Concepts page | First-class Availability & DR with Tier 1 / 2 / 3 in Concepts |
| Deployment target choice implicit | Deployment target chosen explicitly in Plan your deployment |
| "Concepts" mixes definitions, operations, and look-up material | Concepts = explanation only; Reference = look-up only; Operations → Run and maintain |
