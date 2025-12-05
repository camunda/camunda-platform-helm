# deploy-camunda

A CLI tool for deploying Camunda Platform to Kubernetes with automated Helm values preparation.

## Features

- **Configuration-driven deployments** - Define deployment profiles in YAML for consistent, reproducible deployments
- **Scenario-based values** - Automatically prepare Helm values files based on test scenarios
- **Parallel deployments** - Deploy multiple scenarios concurrently with isolated namespaces
- **External secrets support** - Integrate with Vault and External Secrets Operator
- **Keycloak integration** - Automated realm creation and index prefix management
- **Dry-run mode** - Preview deployment configuration without executing
- **Shell completions** - Tab completion for Bash, Zsh, Fish, and PowerShell

## Installation

```bash
# Build from source
cd scripts/deploy-camunda
go build -o deploy-camunda .

# Move to PATH (optional)
mv deploy-camunda /usr/local/bin/
```

## Quick Start

1. **Create a configuration file**:

   ```bash
   # Copy an example to your project root
   cp examples/basic.camunda-deploy.yaml .camunda-deploy.yaml
   
   # Edit with your settings
   vim .camunda-deploy.yaml
   ```

2. **Deploy**:

   ```bash
   # Deploy using active configuration
   deploy-camunda
   
   # Or specify options via flags
   deploy-camunda --namespace my-ns --scenario keycloak --chart-path ./charts/camunda-platform-8.8
   ```

## Configuration

### Configuration File Locations

The tool searches for configuration in this order:

1. `--config` / `-F` flag (explicit path)
2. `.camunda-deploy.yaml` in current directory
3. `~/.config/camunda/deploy.yaml` in user home

### Configuration Structure

```yaml
# Currently active deployment profile
current: dev

# Global defaults (apply to all deployments)
repoRoot: /path/to/camunda-platform-helm
platform: gke
logLevel: info
externalSecrets: true

# Keycloak settings
keycloak:
  host: keycloak.example.com
  protocol: https

# Named deployment profiles
deployments:
  dev:
    chart: camunda-platform-8.8
    namespace: camunda-dev
    release: camunda
    scenario: keycloak

  production:
    chart: camunda-platform-8.8
    namespace: camunda-prod
    release: camunda
    scenario: keycloak
    valuesPreset: enterprise
```

### Environment Variable Overrides

Settings can be overridden via environment variables:

| Variable | Config Field |
|----------|-------------|
| `CAMUNDA_CURRENT` | `current` |
| `CAMUNDA_REPO_ROOT` | `repoRoot` |
| `CAMUNDA_SCENARIO_ROOT` | `scenarioRoot` |
| `CAMUNDA_VALUES_PRESET` | `valuesPreset` |
| `CAMUNDA_PLATFORM` | `platform` |
| `CAMUNDA_LOG_LEVEL` | `logLevel` |
| `CAMUNDA_KEYCLOAK_HOST` | `keycloak.host` |
| `CAMUNDA_KEYCLOAK_PROTOCOL` | `keycloak.protocol` |
| `CAMUNDA_INGRESS_SUBDOMAIN` | `ingressSubdomain` |
| `CAMUNDA_INGRESS_HOSTNAME` | `ingressHostname` |

## Commands

### Deploy (default)

Deploy Camunda Platform using the active configuration:

```bash
deploy-camunda

# Deploy with specific options
deploy-camunda --namespace test --release integration --scenario keycloak

# Deploy multiple scenarios in parallel
deploy-camunda --scenario keycloak,keycloak-mt,saas --namespace integration
```

### Configuration Management

```bash
# List configured deployments
deploy-camunda config list

# Show deployment details (merged with defaults)
deploy-camunda config show dev

# Switch active deployment
deploy-camunda config use production
```

### Validation

Check configuration without deploying:

```bash
# Validate current configuration
deploy-camunda validate

# Validate with scenario and chart path checks
deploy-camunda validate --check-scenarios --check-chart

# Verbose output
deploy-camunda validate --verbose
```

### Dry Run

Preview what would happen without making changes:

```bash
deploy-camunda --dry-run
```

### Shell Completion

```bash
# Bash
deploy-camunda completion bash > /etc/bash_completion.d/deploy-camunda

# Zsh
deploy-camunda completion zsh > "${fpath[1]}/_deploy-camunda"

# Fish
deploy-camunda completion fish > ~/.config/fish/completions/deploy-camunda.fish
```

## Flags Reference

### Chart Source (choose one approach)

| Flag | Description |
|------|-------------|
| `--chart-path` | Local path to chart directory |
| `--chart`, `-c` | Chart name for remote charts |
| `--version`, `-v` | Chart version (use with `--chart`) |

### Core Deployment

| Flag | Description | Default |
|------|-------------|---------|
| `--namespace`, `-n` | Kubernetes namespace | (required) |
| `--release`, `-r` | Helm release name | (required) |
| `--scenario`, `-s` | Scenario name(s), comma-separated | (required) |
| `--auth` | Authentication scenario | `keycloak` |
| `--platform` | Target platform (gke, rosa, eks) | `gke` |

### Keycloak & Index Prefixes

| Flag | Description |
|------|-------------|
| `--keycloak-host` | External Keycloak hostname |
| `--keycloak-protocol` | Keycloak protocol (http/https) |
| `--keycloak-realm` | Realm name (auto-generated if empty) |
| `--optimize-index-prefix` | Optimize ES index prefix |
| `--orchestration-index-prefix` | Orchestration ES index prefix |
| `--tasklist-index-prefix` | Tasklist ES index prefix |
| `--operate-index-prefix` | Operate ES index prefix |

### Ingress Configuration

| Flag | Description |
|------|-------------|
| `--ingress-subdomain` | Subdomain prefix (combined with ci.distro.ultrawombat.com) |
| `--ingress-hostname` | Full hostname override (bypasses subdomain + base domain) |

**Ingress Hostname Construction:**
- If `--ingress-hostname` is set, it's used as-is (full override)
- If `--ingress-subdomain` is set, the hostname is constructed as `{subdomain}.ci.distro.ultrawombat.com`
- For multi-scenario deployments, the scenario name is prepended: `{scenario}-{subdomain}.ci.distro.ultrawombat.com`

### Secrets & Authentication

| Flag | Description | Default |
|------|-------------|---------|
| `--external-secrets` | Enable External Secrets integration | `true` |
| `--auto-generate-secrets` | Generate random test secrets | `false` |
| `--vault-secret-mapping` | Vault secret mapping specification | |
| `--docker-username` | Docker registry username | |
| `--docker-password` | Docker registry password | |
| `--ensure-docker-registry` | Create Docker registry secret | `false` |

### Deployment Behavior

| Flag | Description | Default |
|------|-------------|---------|
| `--flow` | Deployment flow (install/upgrade) | `install` |
| `--timeout` | Helm timeout in minutes | `5` |
| `--skip-dependency-update` | Skip helm dependency update | `true` |
| `--delete-namespace` | Delete namespace before deploy | `false` |
| `--interactive` | Enable interactive prompts | `true` |
| `--dry-run` | Preview without executing | `false` |

### Output

| Flag | Description | Default |
|------|-------------|---------|
| `--render-templates` | Render manifests instead of deploying | `false` |
| `--render-output-dir` | Output directory for rendered manifests | |
| `--log-level`, `-l` | Log level (debug/info/warn/error) | `info` |

## Examples

### Basic Local Development

```bash
deploy-camunda \
  --chart-path ./charts/camunda-platform-8.8 \
  --namespace camunda-dev \
  --release camunda \
  --scenario keycloak
```

### CI/CD Pipeline

```bash
export CAMUNDA_REPO_ROOT=/workspace
deploy-camunda \
  --chart camunda-platform-8.8 \
  --namespace "e2e-${CI_PIPELINE_ID}" \
  --release integration \
  --scenario keycloak \
  --auto-generate-secrets \
  --delete-namespace \
  --interactive=false
```

### Multi-Scenario Parallel Deployment

```bash
deploy-camunda \
  --chart-path ./charts/camunda-platform-8.8 \
  --namespace integration \
  --scenario keycloak,keycloak-mt,oidc \
  --auto-generate-secrets
```

### Template Rendering (GitOps)

```bash
deploy-camunda \
  --chart-path ./charts/camunda-platform-8.8 \
  --namespace production \
  --release camunda \
  --scenario keycloak \
  --render-templates \
  --render-output-dir ./manifests
```

## Project Structure

```
deploy-camunda/
├── cmd/                    # Command definitions
│   ├── root.go            # Main command and flags
│   ├── config.go          # config subcommand
│   ├── validate.go        # validate subcommand
│   └── completion.go      # Shell completion
├── config/                 # Configuration handling
│   ├── config.go          # Config file parsing
│   ├── merge.go           # Flag/config merging
│   └── fields.go          # Field merger utilities
├── deploy/                 # Deployment logic
│   ├── deploy.go          # Main orchestration
│   ├── scenario.go        # Scenario context
│   ├── executor.go        # Deployment execution
│   ├── secrets.go         # Secret generation
│   ├── env.go             # Environment handling
│   ├── output.go          # Result formatting
│   └── constants.go       # Magic strings
├── format/                 # Output formatting
│   └── output.go          # Flag printing
├── internal/util/          # Shared utilities
│   └── strings.go         # String helpers
├── examples/               # Example configurations
└── main.go
```

## Development

### Running Tests

```bash
cd scripts/deploy-camunda
go test ./...
```

### Building

```bash
go build -o deploy-camunda .
```

### Adding New Flags

1. Add field to `RuntimeFlags` in `config/merge.go`
2. Add flag definition in `cmd/root.go`
3. If config-file-settable, add to `DeploymentConfig` and merge logic
4. Update documentation

## Troubleshooting

### "scenario not found" Error

```
Scenario "my-scenario" not found

Available scenarios (5 found):
  - keycloak
  - keycloak-mt
  - oidc
  - saas
  - basic
```

**Solution**: Check `--chart-path` or `--scenario-path` points to a directory containing scenario files.

### "namespace not set" Error

**Solution**: Provide `--namespace` flag or set `namespace` in your config file.

### Configuration Not Loading

Run with `--log-level debug` to see config resolution:

```bash
deploy-camunda --log-level debug validate
```

## License

See [LICENSE](../../LICENSE) in the repository root.

