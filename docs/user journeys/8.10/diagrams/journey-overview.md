# Diagram: 8.10 Administrator Journey Overview

The four-phase journey replaces the 8.9 category-browse structure with an action-sequence structure.

```mermaid
flowchart LR
    QS(["Quickstart\n(dev / admin)\nnot prod-shaped"])

    subgraph PLAN["① Plan your deployment"]
        direction TB
        pDB["Decide: database technology"]
        pID["Decide: identity strategy"]
        pDT["Decide: deployment target"]
        pCL["Pre-install checklist"]
    end

    subgraph BASE["② Build your Baseline"]
        direction TB
        bPROV["Provision dependencies\n(PostgreSQL / ES-OS / Keycloak)"]
        bINST["Install\n(Provision K8s → Install Helm → Configure)"]
        bCONF["Common configuration\n(auth / secrets / license / connectors)"]
        bREG["Register OC in Hub"]
        bPROD["Production readiness checklist"]
        bSMOKE["Smoke test & exit criteria"]
        bPROV --> bINST --> bCONF --> bREG --> bPROD --> bSMOKE
    end

    subgraph EXTEND["③ Extend your deployment\n(independent, any order, any time)"]
        direction TB
        eADR["Availability & DR\nTier 1 / 2 / 3"]
        eTEN["Tenancy & isolation\nLogical / Physical Tenant / Separate OC"]
        eANA["Process analytics\nOptimize per Physical Tenant"]
        eDBT["Database topology\nSplitting / operating DB backends"]
    end

    subgraph DAY2["④ Day-2 operations"]
        direction TB
        dUPG["Upgrades"]
        dBAK["Backups & restore"]
        dMON["Monitoring"]
        dSCA["Scaling"]
        dDRD["DR drills (per tier)"]
        dONB["Onboard new teams\n(new PT vs new OC)"]
        dCRED["Credentials rotation"]
    end

    MIG(["Migrate 8.9 → 8.10\n(once-per-version path)"])
    TRB(["Troubleshooting & FAQ"])

    QS -.->|"shortcut\nnot prod-shaped"| BASE
    PLAN --> BASE
    BASE -->|"Baseline healthy"| EXTEND
    BASE --> DAY2
    EXTEND --> DAY2
    MIG -->|"forced changes\n(Helm v4, Bitnami, Hub values)"| BASE
    MIG -.->|"post-upgrade\noptional catch-up"| BASE
```

## 8.9 vs 8.10 sidebar shape

| 8.9 | 8.10 |
|---|---|
| Categories to browse | Phases to act through |
| Quickstart · Ref Arch · Deploy & manage · Concepts · Components · Upgrade | Quickstart · ① Plan · ② Baseline · ③ Extend · ④ Day-2 · Migrate · Troubleshooting |
| Backup & restore buried under Concepts | Backup & restore top-level Day-2 entry |
| Multi-region buried as one Concepts page | First-class Availability & DR with Tier 1/2/3 |
| Deployment target choice implicit | Deployment target chosen explicitly in Plan |
| Bitnami subcharts removed but no migration path | Migrate from Bitnami in Baseline → Provision |
