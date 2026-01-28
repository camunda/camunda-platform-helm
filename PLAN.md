# PLAN.md - Camunda Platform Helm Deployment Skill

## Vision

Build an AI agent skill that transforms natural language infrastructure requests into production-ready Camunda Platform deployments. The skill acts as an **expert DevOps consultant** that understands Camunda best practices, security requirements, and Kubernetes patterns.

---

## 1. Skill Overview

### What It Does
- Translates user intent ("I need a Camunda cluster with SSO") into actionable deployment artifacts
- Applies organizational best practices automatically
- Generates complete deployment packages (values, secrets, commands)
- Validates configurations against known constraints

### Key Differentiators
- **Intent-driven**: Users describe what they need, not how to configure it
- **Best practices by default**: Security, HA, monitoring enabled appropriately
- **Context-aware**: Understands chart versions, cloud providers, existing infrastructure

---

## 2. Knowledge Base Requirements

### 2.1 Static Knowledge (from this repo)
| Source | Purpose |
|--------|---------|
| `charts/camunda-platform-*/values.schema.json` | Valid configuration options per version |
| `charts/camunda-platform-*/values.yaml` | Default values and structure |
| `charts/camunda-platform-*/README.md` | Component documentation |
| `version-matrix/` | Compatible component versions |
| `charts/chart-versions.yaml` | Available chart versions |

### 2.2 Dynamic Knowledge (to build/maintain)
| Knowledge Type | Content |
|----------------|---------|
| **Deployment Patterns** | Common architectures (dev, staging, prod, HA) |
| **Security Profiles** | Auth methods, TLS configs, network policies |
| **Cloud Provider Specifics** | AWS/GCP/Azure ingress controllers, storage classes |
| **Sizing Guidelines** | Resource requests based on workload |
| **Troubleshooting Guides** | Common issues and resolutions |

---

## 3. Skill Capabilities (Intents)

### 3.1 Primary Intents

```
┌─────────────────────────────────────────────────────────────────┐
│                        USER INTENTS                              │
├─────────────────────────────────────────────────────────────────┤
│  DEPLOY_NEW        → Fresh Camunda installation                 │
│  UPGRADE_EXISTING  → Upgrade chart/components version           │
│  CONFIGURE_INGRESS → Expose services externally                 │
│  SECURE_CLUSTER    → Add auth, TLS, network policies            │
│  SCALE_CLUSTER     → Adjust resources, replicas                 │
│  INTEGRATE_SYSTEM  → Connect external DBs, identity providers   │
│  TROUBLESHOOT      → Diagnose and fix issues                    │
│  EXPLAIN_CONFIG    → Understand existing values                 │
└─────────────────────────────────────────────────────────────────┘
```

### 3.2 Intent Detection Examples

| User Says | Detected Intent | Sub-context |
|-----------|-----------------|-------------|
| "Set up Camunda for my team" | DEPLOY_NEW | dev environment |
| "Production cluster with OAuth" | DEPLOY_NEW | prod + SECURE_CLUSTER |
| "Add basic auth to Operate" | SECURE_CLUSTER | component-specific |
| "Migrate from 8.5 to 8.8" | UPGRADE_EXISTING | version migration |
| "Why is Zeebe not starting?" | TROUBLESHOOT | component: zeebe |

---

## 4. Conversation Flow Design

### 4.1 Information Gathering Strategy

```
                    ┌──────────────────┐
                    │   User Request   │
                    └────────┬─────────┘
                             │
                    ┌────────▼─────────┐
                    │  Detect Intent   │
                    └────────┬─────────┘
                             │
              ┌──────────────┴──────────────┐
              │                             │
     ┌────────▼────────┐          ┌────────▼────────┐
     │ Sufficient Info │          │ Need More Info  │
     │    (Proceed)    │          │   (Clarify)     │
     └────────┬────────┘          └────────┬────────┘
              │                             │
              │                   ┌────────▼────────┐
              │                   │ Smart Questions │
              │                   │ (max 3 rounds)  │
              │                   └────────┬────────┘
              │                             │
              └──────────────┬──────────────┘
                             │
                    ┌────────▼─────────┐
                    │ Apply Defaults   │
                    │ + Best Practices │
                    └────────┬─────────┘
                             │
                    ┌────────▼─────────┐
                    │ Generate Output  │
                    └──────────────────┘
```

### 4.2 Smart Defaults Philosophy

**Principle**: Minimize questions, maximize sensible defaults

| If Unknown | Default To | Rationale |
|------------|------------|-----------|
| Environment | `development` | Safe starting point |
| Chart version | Latest stable | Best features + support |
| Ingress controller | `nginx` | Most common |
| TLS | `enabled` | Security by default |
| Monitoring | `enabled` | Observability matters |
| Identity provider | `Keycloak (embedded)` | Works out of box |

### 4.3 Required vs Optional Information

**Always Required** (ask if missing):
- Target namespace
- Hostname/domain (if ingress needed)

**Inferred from Context**:
- Kubernetes provider (from kubeconfig)
- Existing resources (from cluster scan)
- Team size (affects sizing)

**Optional Enhancements** (suggest, don't require):
- Custom resource limits
- External database connections
- Advanced auth configurations

---

## 5. Output Artifacts

### 5.1 Output Types

```
┌─────────────────────────────────────────────────────────────────┐
│                      OUTPUT PACKAGE                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  📄 values.yaml          Helm values overlay                    │
│  📄 values-secrets.yaml  Sensitive values (gitignored)          │
│  📄 pre-install.sh       Prerequisite commands                  │
│  📄 install.sh           Helm install/upgrade command           │
│  📄 post-install.sh      Verification commands                  │
│  📄 README.md            Deployment documentation               │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### 5.2 Output Modes

| Mode | Use Case | Output |
|------|----------|--------|
| **Explain** | Understanding | Natural language explanation |
| **Review** | Validation | Show config + ask confirmation |
| **Apply** | Execution | Run commands directly |
| **Export** | GitOps | Generate files for repo |

---

## 6. Implementation Phases

### Phase 1: Foundation (Week 1-2)
- [ ] Define SKILL.md structure
- [ ] Extract knowledge from chart schemas
- [ ] Build intent detection logic
- [ ] Create values.yaml generation templates

### Phase 2: Core Intents (Week 3-4)
- [ ] Implement DEPLOY_NEW flow
- [ ] Implement CONFIGURE_INGRESS flow
- [ ] Implement SECURE_CLUSTER flow
- [ ] Build output generation pipeline

### Phase 3: Advanced Features (Week 5-6)
- [ ] Add UPGRADE_EXISTING with migration checks
- [ ] Add TROUBLESHOOT with log analysis
- [ ] Implement cluster scanning for context
- [ ] Add validation against running cluster

### Phase 4: Polish & Testing (Week 7-8)
- [ ] Test across chart versions (8.5-8.9)
- [ ] Test across cloud providers
- [ ] Add edge case handling
- [ ] Documentation and examples

---

## 7. SKILL.md Structure Preview

```markdown
# skill: camunda-helm-deploy

## description
Deploy and manage Camunda Platform on Kubernetes using Helm charts

## triggers
- "deploy camunda"
- "install camunda platform"
- "set up camunda cluster"
- "configure camunda ingress"
- "upgrade camunda"

## context_sources
- charts/camunda-platform-*/values.schema.json
- charts/camunda-platform-*/README.md
- version-matrix/**/*.yaml

## capabilities
- generate_values: Create Helm values from requirements
- generate_commands: Create kubectl/helm commands
- validate_config: Check values against schema
- explain_config: Describe what configuration does

## conversation_rules
- max_clarification_rounds: 3
- apply_best_practices: true
- require_confirmation_before_apply: true

## outputs
- type: yaml (values files)
- type: shell (installation scripts)
- type: markdown (documentation)
```

---

## 8. Success Metrics

| Metric | Target |
|--------|--------|
| Questions before generation | ≤ 3 |
| Valid configurations generated | > 95% |
| User satisfaction (would use again) | > 90% |
| Time to working deployment | < 10 minutes |

---

## 9. Open Questions

1. **Scope**: Should skill handle multi-cluster deployments?
2. **State**: Should skill remember previous deployments?
3. **Execution**: Direct apply vs generate-only?
4. **Versioning**: How to handle deprecated chart versions?

---

## Next Steps

1. Review this plan with team
2. Create SKILL.md skeleton
3. Start Phase 1 implementation
4. Set up testing framework
