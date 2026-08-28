# Narrow document-store configuration to its verified consumers and separate it from ambient cloud credentials

- Status: proposed
- Date: 2026-08-21
- Decision-makers: Distribution team
- Amends: [ADR 0052](0052-centralize-document-store-configuration-via-shared.md), [ADR 0089](0089-integrate-azure-blob-storage-as-a-document-handling-backend.md)

## Context and Problem Statement

Two accepted ADRs describe a document-store architecture the chart never implemented as written.
Source-level verification against the images each chart version pins — done for issue #3741 and
landed in PR #6724 — contradicts a load-bearing claim in each.

[ADR 0052](0052-centralize-document-store-configuration-via-shared.md) states that the shared
`-documentstore-env-vars` ConfigMap is "consumed uniformly by Connectors, Console, Core/Zeebe,
Identity, Optimize, and Web Modeler", and lists "all components receive identical document-store
configuration" as a positive consequence.
[ADR 0089](0089-integrate-azure-blob-storage-as-a-document-handling-backend.md) requires Azure
document-store configuration to be "rendered into application config files for both the orchestration
statefulset and the connectors deployment".

Neither claim held. Both ADRs were created in the 91-file bulk commit `c53782e82` ("docs: add
architecture decision records derived from git history", #6075, 2026-05-12) — retroactive
reconstructions of repository state, not decision-time records. ADR 0052 documents the one-shot
copy-paste of PR #2836, where the wiring was added to every component because it was not yet known
which ones would support the feature (per the #3741 discussion); ADR 0089 documents PR #5370. Their
stated decisions therefore describe implementation accidents, which is why an amendment rather than an
in-place clarification is warranted.

Verified consumption, from application source at the pinned image tags plus the component teams' own
answers in the #3741 thread:

| Component | Reads `DOCUMENT_STORE_*` | Reads the injected cloud credentials | Basis |
|---|---|---|---|
| orchestration (8.8+); `zeebe`, `zeebe-gateway`, `operate`, `tasklist` (8.7) | Yes | — | Serves the REST gateway document API |
| `execution-identity` (8.7) | Potential consumer | — | The REST gateway condition defaults on in `StandaloneCamunda`, so the Document API is genuinely served; wiring kept |
| `console` (8.7–8.9), `web-modeler-webapp` (8.7/8.8) | No | No | Owners confirmed non-use; the pinned images ship no AWS/GCP client, only unreferenced type declarations. Wiring removed in #6724 |
| management `identity` (8.7–8.10) | No | No | No reader. The images ship the AWS SDK v2 credential stack plus `aws-advanced-jdbc-wrapper`/`rds`/`sts` for documented Aurora/RDS IAM database auth, but no chart-renderable configuration activates that path — dormant capability. Wiring removed in #6724 |
| `connectors` (8.7–8.10) | No — documents are proxied through Orchestration's REST API | **Yes** | `CredentialsProviderSupportV2` falls back to `DefaultCredentialsProvider` for AWS connector tasks. ConfigMap reference removed in #6724; credentials retained |
| `optimize` (8.8+) | No | **Yes** | `OpenSearchClientBuilder.useAwsCredentials()` / `getAwsTransport()` use the credentials and `AWS_REGION` to sign AWS OpenSearch requests. ConfigMap reference and credentials retained |
| `web-modeler` restapi (8.7–8.10) | No | **Yes** | The bare credential injection is the sole credential path for its own `camunda.modeler.document-storage.*` feature, and the ConfigMap is its only source of `AWS_REGION`. Retained |

The table exposes a distinction ADR 0052 does not make. The ConfigMap and the env block that
accompanies it carry two unrelated payload classes: the `DOCUMENT_*` / `DOCUMENT_STORE_*` keys, and
ambient cloud credentials (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION`,
`GOOGLE_APPLICATION_CREDENTIALS` and its GCP volume). An earlier revision of this analysis concluded
that Optimize and Connectors had "no reader of the injected config"; that is true only of the
`DOCUMENT_STORE_*` family. Both read the credentials the same block injects, for purposes unrelated
to documents. Collapsing the two classes into one unit produced both the original over-wiring and the
wrong "no reader" conclusion. The residual coupling is tracked in #6778.

ADR 0089's rendering requirement fails on both of its named components:

- Connectors never talks to Azure, or any store, directly. Its Azure application-config block was
  non-functional from the start and was removed in #6724.
- On 8.9 the rendered property `camunda.document.store.azure.connection-string` binds to nothing:
  that application line ships no Azure provider at all. The provider landed on `camunda/camunda`
  `main` as `d8e79996664` (2026-03-05) and was never backported to `stable/8.9` (verification recorded
  in #6735, gap tracked in #6733).
- On 8.10 the property path itself was wrong — the application binds the map-keyed
  `camunda.document.azure.<storeId>.*`, with no legacy alias for the `store` segment. Fixed in PR
  #6737 (issue #6732), merged 2026-08-05.

### Applicability by version

Verified consumer sets on `main` after #6724 and #6737:

- **8.7** — no unified `orchestration` component. ConfigMap consumers: `zeebe` statefulset,
  `zeebe-gateway`, `operate`, `tasklist`, `execution-identity`, `web-modeler` restapi. `connectors`
  carries credentials only; `optimize` carries neither. `console` and `web-modeler-webapp` wiring
  removed in #6724. `global.documentStore.type.azure` does not exist on this line, so ADR 0089 does
  not apply.
- **8.8** — ConfigMap consumers: `orchestration` statefulset, `orchestration` importer deployment,
  `optimize`, `web-modeler` restapi. `connectors` carries credentials only. No Azure backend, so
  ADR 0089 does not apply.
- **8.9** — ConfigMap consumers: `orchestration` statefulset, `optimize`, `web-modeler` restapi.
  `connectors` carries credentials only. Azure configuration renders but is inert (#6733).
- **8.10** — same consumer set as 8.9. Azure renders at the store-id-keyed path after #6737.
  Additionally, the `-documentstore-env-vars` `envFrom` is suppressed per component when that
  component owns `camunda.document.*` through its `extraConfiguration`
  ([ADR 0091](0091-adopt-component-extraconfiguration-as-the-standard-application-configuration-mechanism.md)).

## Decision Drivers

- **Record fidelity.** An accepted ADR that overstates which components implement a shared contract
  misleads reviewers, and — because `docs/adr/` is consumed by `crev` — degrades automated review of
  every subsequent document-store change.
- **Honest blast radius.** ADR 0052 sold uniform propagation across seven components as its main
  benefit. The real coupling is narrower for document configuration and wider for credentials, which
  is the opposite shape and a different risk profile.
- **Prevent re-expansion by symmetry.** The original wiring spread because components were added by
  analogy rather than verification. Without a normative rule, the next backend repeats it.
- **Separate concerns that merely share a transport.** Document configuration and ambient cloud
  credentials travel in one ConfigMap and one env block, but are consumed by different code for
  different reasons and must be reasoned about independently.

## Considered Options

- **Edit ADR 0052 and ADR 0089 in place.** Rejected — `docs/maintainer-guide.md` permits in-place
  edits to an accepted ADR only for non-semantic corrections. Narrowing a consumer set and a rendering
  requirement changes normative content in both.
- **Two amending ADRs, one per amended ADR.** Rejected — both defects share one root cause
  (retroactive documentation of unverified copy-paste wiring) and one corrective rule. Splitting them
  would duplicate the verification table across two records that are then free to drift.
- **Supersede both with a new document-store ADR.** Rejected — the load-bearing decision in each
  still stands: a shared ConfigMap as the single rendering point for document-store configuration
  (0052), and Azure as a first-class backend (0089). Only their consumption and rendering claims are
  wrong.
- **One amending ADR that narrows both claims and adds a verification rule (chosen).**

## Decision Outcome

The consumption claim in ADR 0052 and the rendering requirement in ADR 0089 are narrowed to the
verified sets recorded above. The following constraints are normative for all supported chart
versions:

1. **Consumer set.** The `-documentstore-env-vars` ConfigMap MUST be referenced only by components
   verified to read the `DOCUMENT_*` / `DOCUMENT_STORE_*` keys, or verified to read a key that block
   uniquely supplies (`web-modeler` restapi's `AWS_REGION`, Optimize's AWS OpenSearch signing inputs).
   ADR 0052's "consumed uniformly by Connectors, Console, Core/Zeebe, Identity, Optimize, and Web
   Modeler" is replaced by the per-version sets under "Applicability by version".

2. **Credentials are a separate contract.** `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`,
   `AWS_REGION`, `GOOGLE_APPLICATION_CREDENTIALS` and the GCP credentials volume MUST be treated as
   ambient cloud-SDK configuration, not as document-store configuration. They MAY be injected into a
   component that reads no `DOCUMENT_STORE_*` key, but only when a chart value activates a verified
   consumer path — the credentials are gated on `global.documentStore.type.aws.enabled`, while the
   consuming feature is gated separately (for Optimize, AWS OpenSearch signing). Any analysis of this
   block MUST evaluate the two payload classes independently: "no `DOCUMENT_STORE_*` reader" MUST NOT
   be read as "no reader".

3. **Verification before wiring.** Adding a component to the document-store wiring, or adding a store
   backend to an already-wired component, MUST be justified by source-level verification against the
   image that chart version pins. Symmetry with an already-wired component is not sufficient
   justification.

4. **Azure rendering.** Azure document-store configuration MUST be rendered only for components
   verified to bind it, at the property path the pinned application line actually binds — on 8.10,
   `camunda.document.azure.<storeId>.connection-string`. It MUST NOT be rendered for Connectors.
   ADR 0089's "both the orchestration statefulset and the connectors deployment" is replaced by
   orchestration only.

5. **Inert configuration is a defect, not a decision.** Where a chart renders document-store
   configuration the pinned application cannot bind, the gap MUST be tracked as a bug against the
   chart or the application line, and MUST NOT be recorded as intended architecture.

Applicability: charts 8.7, 8.8, 8.9 and 8.10, with the Azure constraint limited to 8.9 and 8.10.
First implemented in PR #6724 (the removals, merged 2026-08-05) and PR #6737 (the 8.10 Azure path,
merged 2026-08-05). Open follow-ups: #6733 (8.9 Azure inert), #6778 (credential coupling), #6779
(S3-compatible endpoint), #6780 (RWX-backed local document storage).

### Positive Consequences

- Both ADRs now describe wiring that exists, so reviewers and `crev` reason about the real consumer
  set instead of an aspirational one.
- Constraint 3 closes the mechanism that produced the original over-wiring: the next backend or
  component must be verified against a pinned image rather than copied from a neighbour.
- Splitting document configuration from ambient credentials turns #6778 into a scoped follow-up with a
  clear contract, instead of an ambiguous "who reads this ConfigMap" question.
- Fewer components carry a `secretKeyRef` they never read — Console, Web Modeler webapp and management
  Identity no longer receive cloud credentials, reducing needless secret exposure.

### Negative Consequences

- The document-store decision record now spans three ADRs (0052, 0089 and this one) that must be read
  together.
- Constraint 3 adds a verification step — read the pinned image — to every document-store change,
  which is slower than copying an existing component's block.
- Per-version consumer sets replace a single uniform claim, so the record is accurate but no longer
  quotable in one line; maintainers must check the version they are editing.
- Two inert paths remain in shipped charts until #6733 and #6779 land, so this ADR documents the
  correct model while the 8.9 chart still renders Azure configuration that binds to nothing.

## Links

- Builds on [ADR 0091](0091-adopt-component-extraconfiguration-as-the-standard-application-configuration-mechanism.md) — on 8.10 the ConfigMap `envFrom` is suppressed for a component that owns `camunda.document.*` via `extraConfiguration`.
- Issue #3741 — per-component consumer verification and the component-team answers this ADR narrows to.
- Issue #6735 — the amendment request and the full verification evidence.
