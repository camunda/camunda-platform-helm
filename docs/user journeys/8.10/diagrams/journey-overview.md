# Diagram: 8.10 Administrator Journey Overview

The four-phase journey replaces the 8.9 category-browse structure with an action-sequence structure.

```mermaid
flowchart LR
    QS(["Quickstart\n(dev / admin)\nnot prod-shaped"])

    subgraph PLAN["① Plan your deployment"]
        direction TB
        pDT["Choose your deployment target\n(K8s / Containers / Manual / VM)"]
        pDB["Database options\n(depends on target)"]
        pID["Identity & authentication\n(depends on target + database)"]
        pCL["Pre-install checklist"]
        pDT --> pDB --> pID --> pCL
    end

    subgraph BASE["② Build your Baseline"]
        direction TB
        b1["1. Provision infrastructure\n[K8s: cloud provider + K8s config]\n[Containers: Docker / ECS]\n[Manual: VM]"]
        b2["2. Provision databases\n(PostgreSQL / ES-OS / Keycloak)"]
        b3["3. Set up authentication\n(OIDC / Keycloak / basic auth)"]
        b4["4. Install Camunda\nLicense key · Secret management\nInstall Hub · Install OC · Register OC in Hub"]
        bPROD["Production readiness checklist"]
        bSMOKE["Smoke test & exit criteria"]
        b1 --> b2 --> b3 --> b4 --> bPROD --> bSMOKE
    end

    subgraph EXTEND["③ Extend your deployment\n(independent, any order, any time)"]
        direction TB
        eADR["High availability architectures\nTier 1 / 2 / 3 + DR drills"]
        eTEN["Tenancy & isolation\nLogical / Physical Tenant / Separate OC"]
        eANA["Process analytics with Optimize\nOptimize per Physical Tenant"]
    end

    subgraph DAY2["④ Day-2 operations"]
        direction TB
        dUPG["Upgrades"]
        dBAK["Backups & restore"]
        dMON["Monitoring"]
        dSCA["Scaling"]
        dONB["Onboard new teams\n(new PT vs new OC)"]
        dCRED["Credentials rotation"]
    end

    MIG(["Migrate 8.9 → 8.10\n(once-per-version path)"])
    TRB(["Troubleshooting & FAQ"])

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
| Multi-region buried as one Concepts page | First-class High availability architectures with Tier 1/2/3 |
| Deployment target choice implicit | Deployment target chosen explicitly in Plan |
| Bitnami subcharts removed but no migration path | Migrate from Bitnami in Migrate from 8.9 → 8.10 |
