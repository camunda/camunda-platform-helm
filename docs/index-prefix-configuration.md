# Elasticsearch/OpenSearch Index Prefix Configuration

This document describes how to configure index prefixes for Elasticsearch and OpenSearch in Camunda Platform Helm charts.

## Overview

Index prefixes allow you to organize and separate Camunda indices in shared Elasticsearch/OpenSearch clusters. This is particularly useful when:
- Deploying multiple Camunda instances to the same Elasticsearch/OpenSearch cluster
- Separating environments (dev, staging, production) in a shared cluster
- Implementing multi-tenancy strategies

## Helm Values

### Global Prefix Configuration

The Helm charts provide global prefix values that apply to all components:

| Helm Value | Description | Default | Chart Versions |
|------------|-------------|---------|----------------|
| `global.elasticsearch.prefix` | Prefix for Zeebe Elasticsearch exporter indexes | `zeebe-record` | All versions |
| `global.opensearch.prefix` | Prefix for Zeebe OpenSearch exporter indexes | `zeebe-record` | 8.5+ |
| `orchestration.index.prefix` | Prefix for Camunda Exporter (new exporter in 8.8+) | `""` | 8.8+ |

### Example Configuration

```yaml
global:
  elasticsearch:
    enabled: true
    prefix: "demoenv01-zeebe"
  opensearch:
    enabled: false
    prefix: "demoenv01-zeebe"

orchestration:
  index:
    prefix: "demoenv01-orchestration"
```

## Component-Specific Prefix Configuration

### Zeebe Exporter Prefixes

The Zeebe broker exporter uses the global prefix values:

- **Elasticsearch Exporter**: Uses `global.elasticsearch.prefix`
- **OpenSearch Exporter**: Uses `global.opensearch.prefix`
- **Camunda Exporter**: Uses `orchestration.index.prefix` (if set) or falls back to global prefix

### Operate Prefixes

Operate requires two prefix configurations:

1. **Operate's own index prefix**: Used for Operate-specific indices
   - Environment variable: `CAMUNDA_OPERATE_OPENSEARCH_INDEXPREFIX` or `CAMUNDA_OPERATE_ELASTICSEARCH_INDEXPREFIX`
   - Configured via `operate.env` or `orchestration.extraEnvVars` (8.8+)

2. **Zeebe index prefix**: Used to read Zeebe exporter data
   - Environment variable: `CAMUNDA_OPERATE_ZEEBEOPENSEARCH_PREFIX` or `CAMUNDA_OPERATE_ZEEBEELASTICSEARCH_PREFIX`
   - In 8.8+ unified orchestration: Automatically set from `global.opensearch.prefix` or `global.elasticsearch.prefix`
   - In application.yaml: `camunda.operate.zeebeOpensearch.prefix` or `camunda.operate.zeebeElasticsearch.prefix`

### Tasklist Prefixes

Tasklist requires two prefix configurations:

1. **Tasklist's own index prefix**: Used for Tasklist-specific indices
   - Environment variable: `CAMUNDA_TASKLIST_OPENSEARCH_INDEXPREFIX` or `CAMUNDA_TASKLIST_ELASTICSEARCH_INDEXPREFIX`
   - Configured via `tasklist.env` or `orchestration.extraEnvVars` (8.8+)

2. **Zeebe index prefix**: Used to read Zeebe exporter data
   - Environment variable: `CAMUNDA_TASKLIST_ZEEBEOPENSEARCH_PREFIX` or `CAMUNDA_TASKLIST_ZEEBEELASTICSEARCH_PREFIX`
   - In 8.8+ unified orchestration: Automatically set from `global.opensearch.prefix` or `global.elasticsearch.prefix`
   - In application.yaml: `camunda.tasklist.zeebeOpensearch.prefix` or `camunda.tasklist.zeebeElasticsearch.prefix`

### Optimize Prefixes

Optimize uses the global prefix for Zeebe data:

- **Zeebe name**: Uses `global.elasticsearch.prefix` or `global.opensearch.prefix`
  - Environment variable: `CAMUNDA_OPTIMIZE_ZEEBE_NAME`
  - Configured in `optimize.configuration` or automatically from global prefix

- **Optimize index prefix**: Used for Optimize-specific indices
  - Environment variable: `CAMUNDA_OPTIMIZE_OPENSEARCH_SETTINGS_INDEX_PREFIX` or `CAMUNDA_OPTIMIZE_ES_SETTINGS_INDEX_PREFIX`
  - Configured via `optimize.env` or `optimize.configuration`

## Environment Variables Reference

### Zeebe Exporter

| Component | Elasticsearch | OpenSearch |
|-----------|---------------|------------|
| Zeebe Elasticsearch Exporter | `ZEEBE_BROKER_EXPORTERS_ELASTICSEARCH_ARGS_INDEX_PREFIX` | N/A |
| Zeebe OpenSearch Exporter | N/A | `ZEEBE_BROKER_EXPORTERS_OPENSEARCH_ARGS_INDEX_PREFIX` |
| Camunda Exporter | `ZEEBE_BROKER_EXPORTERS_CAMUNDAEXPORTER_ARGS_CONNECT_INDEXPREFIX` | Same |

### Operate

| Component | Elasticsearch | OpenSearch |
|-----------|---------------|------------|
| Operate Index | `CAMUNDA_OPERATE_ELASTICSEARCH_INDEXPREFIX` | `CAMUNDA_OPERATE_OPENSEARCH_INDEXPREFIX` |
| Operate Zeebe | `CAMUNDA_OPERATE_ZEEBEELASTICSEARCH_PREFIX` | `CAMUNDA_OPERATE_ZEEBEOPENSEARCH_PREFIX` |

### Tasklist

| Component | Elasticsearch | OpenSearch |
|-----------|---------------|------------|
| Tasklist Index | `CAMUNDA_TASKLIST_ELASTICSEARCH_INDEXPREFIX` | `CAMUNDA_TASKLIST_OPENSEARCH_INDEXPREFIX` |
| Tasklist Zeebe | `CAMUNDA_TASKLIST_ZEEBEELASTICSEARCH_PREFIX` | `CAMUNDA_TASKLIST_ZEEBEOPENSEARCH_PREFIX` |

### Optimize

| Component | Elasticsearch | OpenSearch |
|-----------|---------------|------------|
| Optimize Zeebe Name | `CAMUNDA_OPTIMIZE_ZEEBE_NAME` | Same |
| Optimize Index Prefix | `CAMUNDA_OPTIMIZE_ES_SETTINGS_INDEX_PREFIX` | `CAMUNDA_OPTIMIZE_OPENSEARCH_SETTINGS_INDEX_PREFIX` |

### Database API

| Component | Elasticsearch | OpenSearch |
|-----------|---------------|------------|
| Database Index Prefix | `CAMUNDA_DATABASE_INDEXPREFIX` | Same |

## Important Notes

### Prefix Consistency

When configuring prefixes, ensure consistency within each group:

**Group 1 - Zeebe Exporter Prefixes** (must match):
- `ZEEBE_BROKER_EXPORTERS_ELASTICSEARCH_ARGS_INDEX_PREFIX` / `ZEEBE_BROKER_EXPORTERS_OPENSEARCH_ARGS_INDEX_PREFIX`
- `CAMUNDA_OPERATE_ZEEBEELASTICSEARCH_PREFIX` / `CAMUNDA_OPERATE_ZEEBEOPENSEARCH_PREFIX`
- `CAMUNDA_TASKLIST_ZEEBEELASTICSEARCH_PREFIX` / `CAMUNDA_TASKLIST_ZEEBEOPENSEARCH_PREFIX`
- `CAMUNDA_OPTIMIZE_ZEEBE_NAME`
- `ZEEBE_BROKER_EXPORTERS_CAMUNDAEXPORTER_ARGS_CONNECT_INDEXPREFIX` (if using Camunda Exporter)

**Group 2 - Component-Specific Index Prefixes** (can differ):
- `CAMUNDA_OPERATE_OPENSEARCH_INDEXPREFIX` / `CAMUNDA_OPERATE_ELASTICSEARCH_INDEXPREFIX`
- `CAMUNDA_TASKLIST_OPENSEARCH_INDEXPREFIX` / `CAMUNDA_TASKLIST_ELASTICSEARCH_INDEXPREFIX`
- `CAMUNDA_OPTIMIZE_OPENSEARCH_SETTINGS_INDEX_PREFIX` / `CAMUNDA_OPTIMIZE_ES_SETTINGS_INDEX_PREFIX`
- `CAMUNDA_DATABASE_INDEXPREFIX`

### Chart Version Differences

- **Charts 8.5-8.7**: Components are separate, prefixes must be configured via environment variables
- **Charts 8.8+**: Unified orchestration cluster automatically propagates `global.opensearch.prefix` and `global.elasticsearch.prefix` to Operate and Tasklist zeebeOpensearch/zeebeElasticsearch configurations

### OpenSearch Support

- Charts 8.5-8.7: OpenSearch prefix support requires manual environment variable configuration
- Charts 8.8+: Full OpenSearch prefix support via `global.opensearch.prefix`

## Examples

### Example 1: Simple Prefix Configuration (8.8+)

```yaml
global:
  opensearch:
    enabled: true
    prefix: "prod-zeebe"

orchestration:
  index:
    prefix: "prod-orchestration"
```

This configuration will:
- Set Zeebe OpenSearch exporter prefix to `prod-zeebe`
- Set Operate zeebeOpensearch prefix to `prod-zeebe`
- Set Tasklist zeebeOpensearch prefix to `prod-zeebe`
- Set Camunda Exporter prefix to `prod-orchestration`

### Example 2: Custom Component Prefixes (8.8+)

```yaml
global:
  opensearch:
    enabled: true
    prefix: "shared-zeebe"

orchestration:
  extraEnvVars:
    - name: CAMUNDA_OPERATE_OPENSEARCH_INDEXPREFIX
      value: "prod-operate"
    - name: CAMUNDA_TASKLIST_OPENSEARCH_INDEXPREFIX
      value: "prod-tasklist"
```

### Example 3: Legacy Charts (8.5-8.7)

```yaml
global:
  opensearch:
    enabled: true
    prefix: "prod-zeebe"

operate:
  env:
    - name: CAMUNDA_OPERATE_ZEEBEOPENSEARCH_PREFIX
      value: "prod-zeebe"
    - name: CAMUNDA_OPERATE_OPENSEARCH_INDEXPREFIX
      value: "prod-operate"

tasklist:
  env:
    - name: CAMUNDA_TASKLIST_ZEEBEOPENSEARCH_PREFIX
      value: "prod-zeebe"
    - name: CAMUNDA_TASKLIST_OPENSEARCH_INDEXPREFIX
      value: "prod-tasklist"
```

## References

- [Camunda 8 Self-Managed Documentation](https://docs.camunda.io/docs/self-managed/about-self-managed/)
- [Helm Chart README](../charts/camunda-platform-8.9/README.md)

