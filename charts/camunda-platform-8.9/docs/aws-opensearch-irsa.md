# Configuring AWS OpenSearch IRSA (IAM Roles for Service Accounts)

Starting with Camunda 8.9, the Helm chart **no longer manages** the `awsEnabled` / `aws-enabled` application properties. The legacy global flag `global.opensearch.aws.enabled` is **deprecated**, ignored by the chart, and will be removed in a future version.

Users must configure AWS IRSA directly via environment variables or extra configuration files.

## Background

The Java-side change ([camunda/camunda#47230](https://github.com/camunda/camunda/pull/47230)) introduced a unified configuration property `camunda.data.secondary-storage.opensearch.aws-enabled` that propagates the AWS flag to all internal components (Operate `OpensearchConnector`, Tasklist, Camunda Exporter). Previously, the legacy Operate connector ignored the `awsEnabled` flag, causing crashes on AWS OpenSearch IRSA deployments.

The documentation for this property is tracked in [camunda/camunda-docs#8351](https://github.com/camunda/camunda-docs/pull/8351).

## Legacy configuration (deprecated)

```yaml
global:
  opensearch:
    enabled: true
    aws:
      enabled: true  # DEPRECATED — ignored by the chart, triggers a warning at deploy time
```

Setting this value emits a deprecation warning during `helm install`/`helm upgrade` but has **no effect** on the deployed application. You must use one of the options below instead.

## Configuration options

### Option 1: Environment variable

Use `orchestration.env` to set the Spring Boot property as an environment variable:

```yaml
orchestration:
  env:
    - name: CAMUNDA_DATA_SECONDARYSTORAGE_OPENSEARCH_AWSENABLED
      value: "true"
```

This follows Spring Boot's [relaxed binding](https://docs.spring.io/spring-boot/docs/current/reference/html/features.html#features.external-config.typesafe-configuration-properties.relaxed-binding) rules: dots become underscores, hyphens are removed, everything is uppercased.

For Optimize (separate component), use `optimize.env`:

```yaml
optimize:
  env:
    - name: CAMUNDA_OPTIMIZE_OPENSEARCH_AWS_ENABLED
      value: "true"
```

### Option 2: Extra configuration file

Use `orchestration.extraConfiguration` to provide a custom `application.yaml` snippet that gets merged via Spring's config import mechanism:

```yaml
orchestration:
  extraConfiguration:
    - file: aws-opensearch.yaml
      content: |
        camunda:
          data:
            secondary-storage:
              opensearch:
                aws-enabled: true
```

The file is mounted into the pod at `/usr/local/camunda/config/aws-opensearch.yaml` and imported via `spring.config.import`.

## Legacy property mapping

The unified `camunda.data.secondary-storage.opensearch.aws-enabled` property replaces all of these legacy properties:

| Legacy property | Component |
|----------------|-----------|
| `camunda.database.awsEnabled` | Global database config |
| `camunda.operate.opensearch.awsEnabled` | Operate |
| `camunda.tasklist.opensearch.awsEnabled` | Tasklist |
| `zeebe.broker.exporters.camundaexporter.args.connect.awsEnabled` | Zeebe Camunda Exporter |

## Elasticsearch and RDBMS

Similar properties exist for Elasticsearch and RDBMS secondary storage. Configure them via environment variables:

```bash
CAMUNDA_DATA_SECONDARYSTORAGE_ELASTICSEARCH_AWSENABLED=true
CAMUNDA_DATA_SECONDARYSTORAGE_RDBMS_AWSENABLED=true
```

Or via extra configuration:

```yaml
orchestration:
  extraConfiguration:
    - file: aws-secondary-storage.yaml
      content: |
        camunda:
          data:
            secondary-storage:
              elasticsearch:
                aws-enabled: true
              rdbms:
                aws-enabled: true
```
