# Adopt Docusaurus as the dedicated documentation platform for Helm chart project knowledge

- Status: accepted
- Date: 2026-04-08
- Decision-makers: Hamza Masood

## Context and Problem Statement

The Helm chart repository's documentation lived as scattered Markdown files in a `docs/` directory with no unified presentation layer, navigation, or versioning capability. As the project grew across multiple chart versions (8.8, 8.9, 8.10), contributors and operators needed a structured, searchable documentation site rather than raw Markdown files browsed on GitHub. The team needed a documentation platform that could be maintained alongside the code with CI-driven previews and deployments.

## Decision Drivers

- **Developer experience**: Contributors need discoverable, navigable documentation rather than hunting through flat Markdown files in the repository
- **CI/CD alignment**: Documentation should follow the same review and deployment patterns as code — preview on PR, deploy on merge
- **Low maintenance overhead**: The documentation tooling should be familiar to the JavaScript/TypeScript ecosystem already present in the project and require minimal custom infrastructure
- **Separation of concerns**: Documentation site tooling should not pollute the root project structure or interfere with Helm chart development workflows

## Considered Options

- **GitHub Pages with Jekyll** — Rejected; Jekyll is implicit in GitHub Pages but offers less flexibility for custom navigation, and the team already signals rejection via `.nojekyll` static file
- **MkDocs (Python-based)** — Would introduce a Python toolchain dependency not otherwise present in the project
- **Docusaurus (React/Node-based)** — Aligns with existing Node.js tooling (already in `.tool-versions`), provides built-in versioning, search, and sidebar navigation out of the box

## Decision Outcome

A dedicated Docusaurus site was bootstrapped under `helm-docs-site/` as an isolated subdirectory with its own `package.json` and dependency tree. Existing documentation files in `docs/` were restructured to serve as content sources, and two GitHub Actions workflows were added — one for deploy-on-merge and one for PR preview builds. This establishes documentation as a first-class artifact with its own build pipeline.

### Positive Consequences

- Documentation gets professional navigation, search, and structure without custom tooling effort
- PR previews enable documentation review as part of the normal code review process, catching errors before merge
- The isolated `helm-docs-site/` directory keeps documentation tooling decoupled from Helm chart development, allowing independent dependency updates

### Negative Consequences

- Adds a Node.js application to maintain (dependency updates, Docusaurus major version upgrades) alongside the primary Go/Helm toolchain
- Two CI workflows increase pipeline complexity and consume additional GitHub Actions minutes on every documentation-touching PR