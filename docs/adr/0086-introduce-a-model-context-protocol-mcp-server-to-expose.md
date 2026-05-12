# Introduce a Model Context Protocol (MCP) server to expose Helm chart values as AI-toolable knowledge

- Status: accepted
- Date: 2026-03-10
- Decision-makers: Clément Nero

## Context and Problem Statement

Camunda's Helm charts contain complex, versioned configuration surfaces spanning multiple components and deployment scenarios. Engineers and AI assistants working with these charts lack structured, queryable access to valid configuration options, component relationships, and scenario definitions. This knowledge is scattered across values files, documentation, and CI configuration, making it difficult to generate correct Helm values without deep repository familiarity.

## Decision Drivers

- **AI-assisted developer experience**: Enable LLM-based tools (Copilot, OpenCode, Claude) to programmatically discover and reason about chart configuration without hallucinating invalid paths or values
- **Maintainability of configuration knowledge**: Centralize parsing and serving of Helm values metadata in a single, testable service rather than relying on ad-hoc prompts or documentation that drifts from source
- **Deployment independence**: Keep the MCP server as a self-contained module with its own lifecycle, dependencies, and containerization, avoiding coupling to the chart release process
- **Discoverability across versions**: Support querying configuration across multiple chart versions and deployment scenarios from a single interface

## Considered Options

- **Static documentation generation** — Rejected because documentation drifts from source and cannot answer contextual queries or generate examples dynamically
- **Embedding values schemas directly into AI system prompts** — Rejected due to token limits and inability to handle multi-version, multi-scenario complexity
- **Extending deploy-camunda CLI with query subcommands** — Rejected as it would conflate deployment tooling with knowledge-serving concerns and wouldn't integrate with MCP-compatible AI clients

## Decision Outcome

A new standalone TypeScript service (`helm-values-mcp/`) was introduced implementing the Model Context Protocol, exposing tools for listing versions, components, scenarios, searching configurations, retrieving config details, and generating example values. The server fetches and parses Helm values data, stores it in an in-memory structure, and serves it over the MCP transport layer. It is containerized independently and integrated into the VS Code MCP client configuration.

### Positive Consequences

- AI assistants can generate valid, version-aware Helm values configurations without hallucination, reducing deployment errors
- Configuration knowledge is derived directly from source data via the fetcher/parser pipeline, ensuring it stays current
- The module boundary is clean — separate package, Dockerfile, and test suite — allowing independent iteration without affecting chart releases

### Negative Consequences

- Introduces a new service to maintain, including its own dependency tree (Node.js/TypeScript), test infrastructure, and container build pipeline
- The MCP protocol is relatively nascent; changes to the spec or client ecosystem may require rework of the server interface