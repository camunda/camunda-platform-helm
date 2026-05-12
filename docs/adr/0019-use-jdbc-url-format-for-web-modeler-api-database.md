# Use JDBC URL format for Web Modeler API database configuration

- Status: accepted
- Date: 2023-09-29
- Decision-makers: Wolfgang Amann

## Context and Problem Statement

The Web Modeler REST API's external database configuration used a decomposed format of individual `host`, `port`, and `database` fields. This approach limited flexibility for users who needed to pass additional JDBC connection parameters (such as SSL mode, connection timeouts, or schema specifications) and forced the Helm template layer to assemble connection strings from discrete parts.

## Decision Drivers

- **Flexibility**: Users need to specify arbitrary JDBC connection parameters that cannot be anticipated by individual Helm values fields
- **Convention alignment**: JDBC URLs are the standard way applications accept database configuration; decomposing them into parts is an unnecessary abstraction
- **Reduced template complexity**: Assembling URLs from parts in Helm templates introduces logic that is fragile and difficult to test across edge cases
- **Compatibility with managed database services**: Cloud-managed PostgreSQL offerings often provide a full JDBC URL directly, making copy-paste configuration simpler

## Considered Options

- **Keep host/port/database fields and add an optional parameters field** — Rejected because it still requires URL assembly in templates and doesn't cover all JDBC URL variations (e.g., multiple hosts for failover)
- **Support both formats simultaneously** — Rejected to avoid long-term maintenance burden of dual code paths and ambiguous precedence rules
- **Adopt a single JDBC URL field (chosen)** — Breaking change accepted in favor of a cleaner, more capable interface

## Decision Outcome

The `webModeler.restapi.externalDatabase` configuration was changed from discrete `host`, `port`, and `database` fields to a single `url` field accepting a full JDBC connection string. This shifts connection string responsibility to the operator and removes assembly logic from Helm templates.

### Positive Consequences

- Users can now pass any valid JDBC URL including failover hosts, SSL parameters, and connection pool tuning without chart changes
- Helm template logic is simplified — no string concatenation or conditional port handling
- Aligns with how the underlying Spring Boot application natively consumes datasource configuration

### Negative Consequences

- **Breaking change**: Existing deployments must migrate their values files from the decomposed format to a JDBC URL, requiring action during upgrades
- Slightly higher cognitive load for simple cases where users previously only needed to specify a hostname and database name