# Upgrade Guide

## Breaking Changes in 8.10

### Authentication Method Changes

**What changed:** The `authentication.method: none` option has been removed.

**Why:** The Camunda application dropped support for `authentication.method: none` in version 8.8. This Helm chart now enforces this at installation time via JSON schema validation.

**Who's affected:**
- Users explicitly setting `authentication.method: none` in their values files
- Users relying on the implicit `none` default (if no authentication method was specified)

**Migration path:**

If you need unauthenticated access, use:

```yaml
global:
  security:
    authentication:
      method: basic
      unprotectedApi: true
```

Or for specific components:

```yaml
orchestration:
  security:
    authentication:
      method: basic
      unprotectedApi: true

connectors:
  security:
    authentication:
      method: basic
      unprotectedApi: true

webModeler:
  security:
    authentication:
      method: basic
```

**Note:** For version 8.8, the chart will fail at render time with a clear error message if `none` is used. For versions 8.9 and later, Helm will reject the installation at validation time.
