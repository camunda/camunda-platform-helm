# Deploy-Camunda Configuration Examples

This directory contains example configuration files for the `deploy-camunda` tool.

## Quick Start

1. Copy one of these examples to your project root as `.camunda-deploy.yaml`:

   ```bash
   cp basic.camunda-deploy.yaml /path/to/project/.camunda-deploy.yaml
   ```

   Or to your user config directory:

   ```bash
   mkdir -p ~/.config/camunda
   cp basic.camunda-deploy.yaml ~/.config/camunda/deploy.yaml
   ```

2. Edit the configuration to match your environment

3. Run the tool:

   ```bash
   deploy-camunda
   ```

## Examples

| File | Description |
|------|-------------|
| `basic.camunda-deploy.yaml` | Simple configuration for getting started |
| `advanced.camunda-deploy.yaml` | Full reference of all available options |
| `ci-pipeline.camunda-deploy.yaml` | Optimized for CI/CD automation |

## Configuration Resolution

The tool looks for configuration in this order:

1. Explicit path via `--config` / `-F` flag
2. `.camunda-deploy.yaml` in current directory
3. `~/.config/camunda/deploy.yaml` in user home

## Environment Variable Overrides

Many settings can be overridden via environment variables with the `CAMUNDA_` prefix:

| Environment Variable | Config Field |
|---------------------|--------------|
| `CAMUNDA_CURRENT` | `current` |
| `CAMUNDA_REPO_ROOT` | `repoRoot` |
| `CAMUNDA_SCENARIO_ROOT` | `scenarioRoot` |
| `CAMUNDA_VALUES_PRESET` | `valuesPreset` |
| `CAMUNDA_PLATFORM` | `platform` |
| `CAMUNDA_LOG_LEVEL` | `logLevel` |
| `CAMUNDA_KEYCLOAK_HOST` | `keycloak.host` |
| `CAMUNDA_KEYCLOAK_PROTOCOL` | `keycloak.protocol` |
| `CAMUNDA_HOSTNAME` | `ingressHost` |

## Commands

```bash
# Deploy using current configuration
deploy-camunda

# List configured deployments
deploy-camunda config list

# Show details of a deployment
deploy-camunda config show dev

# Switch active deployment
deploy-camunda config use staging

# Override settings via flags
deploy-camunda --namespace my-ns --scenario keycloak

# Validate configuration without deploying
deploy-camunda validate

# Preview deployment (dry-run)
deploy-camunda --dry-run
```

