# Architecture Decision Records

| # | Decision |
|---|----------|
| 1 | [Copy zeebe-cluster-helm chart into monorepo to establish unified Helm chart CI/CD pipeline](0001-copy-zeebe-cluster-helm-chart-into-monorepo-to-establish.md) |
| 2 | [Introduce a dedicated Helm chart for Zeebe Operate as an independently deployable component](0002-introduce-a-dedicated-helm-chart-for-zeebe-operate-as-an.md) |
| 3 | [Introduce a dedicated Helm chart for Zeebe Tasklist as an independently deployable component](0003-introduce-a-dedicated-helm-chart-for-zeebe-tasklist-as-an.md) |
| 4 | [Introduce a unified "full" Helm chart to orchestrate all Zeebe sub-charts as a single deployable unit](0004-introduce-a-unified-full-helm-chart-to-orchestrate-all.md) |
| 5 | [Add a dedicated Helm chart for ZeeQS as an independent deployable component](0005-add-a-dedicated-helm-chart-for-zeeqs-as-an-independent.md) |
| 6 | [Support single domain setup for Camunda Platform Helm chart ingress routing](0006-support-single-domain-setup-for-camunda-platform-helm-chart.md) |
| 7 | [Allow using an external/existing Keycloak instance instead of the bundled one](0007-allow-using-an-external-existing-keycloak-instance-instead.md) |
| 8 | [Replace Keycloak fullname string with structured URL configuration map](0008-replace-keycloak-fullname-string-with-structured-url.md) |
| 9 | [Introduce Web Modeler as an in-tree subchart with multi-component deployment architecture](0009-introduce-web-modeler-as-an-in-tree-subchart-with-multi.md) |
| 10 | [Support Keycloak v19 via Bitnami Helm Chart v12.2.0 as Identity Provider Dependency](0010-support-keycloak-v19-via-bitnami-helm-chart-v12-2-0-as.md) |
| 11 | [Migrate integration test infrastructure from custom testsuite runner to Venom declarative framework](0011-migrate-integration-test-infrastructure-from-custom.md) |
| 12 | [Add Connectors as a first-class Helm chart component without Keycloak dependency](0012-add-connectors-as-a-first-class-helm-chart-component.md) |
| 13 | [Remove backward compatibility support for legacy Keycloak v16 in Helm chart templates](0013-remove-backward-compatibility-support-for-legacy-keycloak.md) |
| 14 | [Enforce authenticated communication between platform components and Zeebe Gateway](0014-enforce-authenticated-communication-between-platform.md) |
| 15 | [Add sidecar container support to all Camunda platform Helm chart components](0015-add-sidecar-container-support-to-all-camunda-platform-helm.md) |
| 16 | [Add Console as a first-class component in the Camunda Platform Helm chart](0016-add-console-as-a-first-class-component-in-the-camunda.md) |
| 17 | [Add first-class initContainers extension points to all Helm chart components](0017-add-first-class-initcontainers-extension-points-to-all-helm.md) |
| 18 | [Add global key and Zeebe configuration support for multi-tenancy](0018-add-global-key-and-zeebe-configuration-support-for-multi.md) |
| 19 | [Use JDBC URL format for Web Modeler API database configuration](0019-use-jdbc-url-format-for-web-modeler-api-database.md) |
| 20 | [Migrate Elasticsearch dependency from 7.x to 8.x for Camunda Platform Helm chart](0020-migrate-elasticsearch-dependency-from-7-x-to-8-x-for.md) |
| 21 | [Add multi-tenancy configuration support to Identity service](0021-add-multi-tenancy-configuration-support-to-identity-service.md) |
| 22 | [Enforce read-only root filesystems across all Camunda platform components](0022-enforce-read-only-root-filesystems-across-all-camunda.md) |
| 23 | [Integrate Console Self-Managed with Identity for authentication and authorization](0023-integrate-console-self-managed-with-identity-for.md) |
| 24 | [Consolidate Operate from subchart into parent Helm chart templates](0024-consolidate-operate-from-subchart-into-parent-helm-chart.md) |
| 25 | [Enable Zeebe Multi-Region Deployment Support in the Camunda Platform Helm Chart](0025-enable-zeebe-multi-region-deployment-support-in-the-camunda.md) |
| 26 | [Hoist Identity authentication secrets to the parent chart to enable multi-namespace deployment](0026-hoist-identity-authentication-secrets-to-the-parent-chart.md) |
| 27 | [Centralize auth configuration through a shared Identity ConfigMap rather than per-component variables](0027-centralize-auth-configuration-through-a-shared-identity.md) |
| 28 | [Switch Connectors component to Identity-based authentication configuration](0028-switch-connectors-component-to-identity-based.md) |
| 29 | [Integrate Console as a first-class component in the Camunda Platform Helm chart](0029-integrate-console-as-a-first-class-component-in-the-camunda.md) |
| 30 | [Support OIDC as an alternative identity provider to Keycloak in the Camunda platform Helm chart](0030-support-oidc-as-an-alternative-identity-provider-to.md) |
| 31 | [Promote Identity from subchart to top-level template directory for structural consistency](0031-promote-identity-from-subchart-to-top-level-template.md) |
| 32 | [Split Zeebe Gateway ingress into separate REST and gRPC resources to support multi-protocol exposure](0032-split-zeebe-gateway-ingress-into-separate-rest-and-grpc.md) |
| 33 | [Migrate application configuration from environment variables to ConfigMap-mounted files](0033-migrate-application-configuration-from-environment.md) |
| 34 | [Remove image version tags from Kubernetes matchLabels selectors to enable non-destructive Helm upgrades](0034-remove-image-version-tags-from-kubernetes-matchlabels.md) |
| 35 | [Support AWS IAM Roles for Service Accounts (IRSA) for OpenSearch authentication](0035-support-aws-iam-roles-for-service-accounts-irsa-for.md) |
| 36 | [Vendor Web Modeler PostgreSQL as an embedded sub-chart to resolve Helm dependency name collisions](0036-vendor-web-modeler-postgresql-as-an-embedded-sub-chart-to.md) |
| 37 | [Adopt per-component Kubernetes ServiceAccounts for Camunda platform Helm chart](0037-adopt-per-component-kubernetes-serviceaccounts-for-camunda.md) |
| 38 | [Adopt directory-based multi-version chart structure for independent version lifecycle management](0038-adopt-directory-based-multi-version-chart-structure-for.md) |
| 39 | [Adopt reusable workflow templates with declarative per-version configuration for CI/CD pipelines](0039-adopt-reusable-workflow-templates-with-declarative-per.md) |
| 40 | [Introduce a standalone alpha release channel for the Camunda Platform Helm chart](0040-introduce-a-standalone-alpha-release-channel-for-the.md) |
| 41 | [Use Public Docker Images for Web Modeler Components](0041-use-public-docker-images-for-web-modeler-components.md) |
| 42 | [Remove global image tag in favor of component-level image versioning](0042-remove-global-image-tag-in-favor-of-component-level-image.md) |
| 43 | [Hardcode deployment strategy for all components and remove user-configurable strategy option](0043-hardcode-deployment-strategy-for-all-components-and-remove.md) |
| 44 | [Replace imperative post-render script with declarative Helm values overlay for OpenShift compatibility](0044-replace-imperative-post-render-script-with-declarative-helm.md) |
| 45 | [Adopt Helm-hook-based secrets auto-generation with install-time immutability](0045-adopt-helm-hook-based-secrets-auto-generation-with-install.md) |
| 46 | [Migrate Bitnami Helm chart dependencies from HTTP repository to OCI registry](0046-migrate-bitnami-helm-chart-dependencies-from-http.md) |
| 47 | [Introduce a Camunda Exporter abstraction layer for Zeebe data export](0047-introduce-a-camunda-exporter-abstraction-layer-for-zeebe.md) |
| 48 | [Consolidate Zeebe, Operate, and Tasklist into a unified Orchestration Core statefulset](0048-consolidate-zeebe-operate-and-tasklist-into-a-unified.md) |
| 49 | [Consolidate ingress routing into a single global combined ingress](0049-consolidate-ingress-routing-into-a-single-global-combined.md) |
| 50 | [Remove deprecated Helm values keys during alpha window to eliminate compatibility debt](0050-remove-deprecated-helm-values-keys-during-alpha-window-to.md) |
| 51 | [Revert Camunda 8.7 Helm chart to 8.6 structure and isolate architectural refactoring in alpha-8.8](0051-revert-camunda-8-7-helm-chart-to-8-6-structure-and-isolate.md) |
| 52 | [Centralize document-store configuration via shared ConfigMap with multi-backend support](0052-centralize-document-store-configuration-via-shared.md) |
| 53 | [Disable management identity by default in Camunda 8.8 alpha chart](0053-disable-management-identity-by-default-in-camunda-8-8-alpha.md) |
| 54 | [Publish alpha Helm chart releases to the public repository](0054-publish-alpha-helm-chart-releases-to-the-public-repository.md) |
| 55 | [Introduce dedicated OpenSearch index prefix configuration independent of Elasticsearch settings](0055-introduce-dedicated-opensearch-index-prefix-configuration.md) |
| 56 | [Support image digest-based pinning as an alternative to tag-based references across all Helm chart components](0056-support-image-digest-based-pinning-as-an-alternative-to-tag.md) |
| 57 | [Support Core Identity OIDC as the authentication mechanism for Camunda 8.8 components](0057-support-core-identity-oidc-as-the-authentication-mechanism.md) |
| 58 | [Co-locate Playwright E2E tests with versioned Helm charts for PR-level integration validation](0058-co-locate-playwright-e2e-tests-with-versioned-helm-charts.md) |
| 59 | [Introduce Identity Migration as a Dedicated Helm Job Separate from the Identity Deployment](0059-introduce-identity-migration-as-a-dedicated-helm-job.md) |
| 60 | [Introduce a dedicated Identity Migration Job as a separate Helm component (Phase 2)](0060-introduce-a-dedicated-identity-migration-job-as-a-separate.md) |
| 61 | [Introduce a unified configuration mechanism for Camunda Platform core components](0061-introduce-a-unified-configuration-mechanism-for-camunda.md) |
| 62 | [Introduce Process Migration as a Dedicated Kubernetes Job with Separate ConfigMap](0062-introduce-process-migration-as-a-dedicated-kubernetes-job.md) |
| 63 | [Internalize integration test values within the Helm chart repository to eliminate cross-repo push dependency](0063-internalize-integration-test-values-within-the-helm-chart.md) |
| 64 | [Support deployment without secondary storage (Elasticsearch/OpenSearch) in Camunda 8.8 Helm chart](0064-support-deployment-without-secondary-storage-elasticsearch.md) |
| 65 | [Introduce an orchestration cluster compatibility layer between 8.7 and 8.8 charts](0065-introduce-an-orchestration-cluster-compatibility-layer.md) |
| 66 | [Rename the Helm chart "core" values key to "orchestration" to align with upstream product terminology](0066-rename-the-helm-chart-core-values-key-to-orchestration-to.md) |
| 67 | [Relocate security configuration from global scope to orchestration component scope](0067-relocate-security-configuration-from-global-scope-to.md) |
| 68 | [Standardize existingSecret input interface across all Helm chart components via a common normalization layer](0068-standardize-existingsecret-input-interface-across-all-helm.md) |
| 69 | [Separate Tasklist and Operate importer into its own Kubernetes deployment](0069-separate-tasklist-and-operate-importer-into-its-own.md) |
| 70 | [Differentiate orchestration services into distinct Kubernetes resources for independent routing and observability](0070-differentiate-orchestration-services-into-distinct.md) |
| 71 | [Implement matrix-driven CI framework for testing supported minor version upgrade paths](0071-implement-matrix-driven-ci-framework-for-testing-supported.md) |
| 72 | [Centralize OIDC and Microsoft Entra authentication configuration into shared helpers and a unified ConfigMap](0072-centralize-oidc-and-microsoft-entra-authentication.md) |
| 73 | [Remove standalone migrator component from Camunda 8.9 chart in favor of integrated migration lifecycle](0073-remove-standalone-migrator-component-from-camunda-8-9-chart.md) |
| 74 | [Externalize Elasticsearch and Keycloak as shared CI infrastructure services](0074-externalize-elasticsearch-and-keycloak-as-shared-ci.md) |
| 75 | [Rewrite CI shell scripts as structured Go CLIs with shared core library](0075-rewrite-ci-shell-scripts-as-structured-go-clis-with-shared.md) |
| 76 | [Support hybrid authentication mode in Helm charts to enable mixed Identity/direct-credential deployments](0076-support-hybrid-authentication-mode-in-helm-charts-to-enable.md) |
| 77 | [Add RDBMS as a supported persistence backend for Camunda 8.9 orchestration components](0077-add-rdbms-as-a-supported-persistence-backend-for-camunda-8.md) |
| 78 | [Remove default secondaryStorage configuration from Helm chart to require explicit opt-in](0078-remove-default-secondarystorage-configuration-from-helm.md) |
| 79 | [Extend deployment CLI and integration test infrastructure to support EKS as a first-class platform](0079-extend-deployment-cli-and-integration-test-infrastructure.md) |
| 80 | [Adopt Kubernetes Gateway API as a first-class routing alternative alongside Ingress in the Helm chart](0080-adopt-kubernetes-gateway-api-as-a-first-class-routing.md) |
| 81 | [Expose Helm v4 compatibility options as explicit values.yaml configuration across all supported chart versions](0081-expose-helm-v4-compatibility-options-as-explicit-values.md) |
| 82 | [Enforce explicit secret management by removing deprecated secret keys and autogeneration in Camunda 8.9](0082-enforce-explicit-secret-management-by-removing-deprecated.md) |
| 83 | [Deprecate Bitnami subcharts in Camunda Platform Helm chart 8.9](0083-deprecate-bitnami-subcharts-in-camunda-platform-helm-chart.md) |
| 84 | [Deprecate global Elasticsearch/OpenSearch configuration in favor of component-scoped settings for Optimize](0084-deprecate-global-elasticsearch-opensearch-configuration-in.md) |
| 85 | [Consolidate Web Modeler into a single-container architecture by removing the standalone webapp component](0085-consolidate-web-modeler-into-a-single-container.md) |
| 86 | [Introduce a Model Context Protocol (MCP) server to expose Helm chart values as AI-toolable knowledge](0086-introduce-a-model-context-protocol-mcp-server-to-expose.md) |
| 87 | [Remove deprecated 8.8-cycle compatibility shims from the 8.10 Helm chart](0087-remove-deprecated-8-8-cycle-compatibility-shims-from-the-8.md) |
| 88 | [Remove shared Argo-managed Elasticsearch and Keycloak infrastructure from CI](0088-remove-shared-argo-managed-elasticsearch-and-keycloak.md) |
| 89 | [Integrate Azure Blob Storage as a document handling backend in the Helm chart](0089-integrate-azure-blob-storage-as-a-document-handling-backend.md) |
| 90 | [Adopt Docusaurus as the dedicated documentation platform for Helm chart project knowledge](0090-adopt-docusaurus-as-the-dedicated-documentation-platform.md) |
| 91 | [Standardize `<component>.extraConfiguration` as the Application Configuration Mechanism](0091-adopt-component-extraconfiguration-as-the-standard-application-configuration-mechanism.md) |
| 92 | [Allow opt-in deployment strategy for components with chart-managed RWO persistence](0092-allow-opt-in-deployment-strategy-for-components-with-chart-managed-rwo-persistence.md) |
| 93 | [Adopt a composable CI scenario registry for the per-version integration test matrix](0093-adopt-composable-ci-scenario-registry-for-per-version-test-matrix.md) |
| 94 | [Remove bundled Bitnami subcharts from the 8.10 chart and migrate CI to companion charts](0094-remove-bundled-bitnami-subcharts-from-the-8-10-chart.md) |
| 95 | [Ship opt-in least-privilege NetworkPolicies as a first-class chart feature](0095-ship-opt-in-least-privilege-networkpolicies.md) |
