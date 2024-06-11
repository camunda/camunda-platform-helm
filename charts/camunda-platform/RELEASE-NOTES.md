The changelog is automatically generated using [git-chglog](https://github.com/git-chglog/git-chglog)
and it follows [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) format.


<a name="camunda-platform-10.1.0"></a>
## [camunda-platform-10.1.0](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-10.0.4...camunda-platform-10.1.0) (2024-06-10)

### Ci

* enrich release notes and version matrix ([#1716](https://github.com/camunda/camunda-platform-helm/issues/1716))

### Docs

* enhance version matrix structure ([#1742](https://github.com/camunda/camunda-platform-helm/issues/1742))

### Feat

* warning and error for not setting existingSecrets for all components ([#1876](https://github.com/camunda/camunda-platform-helm/issues/1876))
* support envFrom reference in all components ([#1949](https://github.com/camunda/camunda-platform-helm/issues/1949))
* adding a constraint for identity existingSecret ([#1969](https://github.com/camunda/camunda-platform-helm/issues/1969))

### Fix

* release info did not respect the context path ([#1908](https://github.com/camunda/camunda-platform-helm/issues/1908))
* update livenessProbe endpoint of zeebe ([#1858](https://github.com/camunda/camunda-platform-helm/issues/1858))
* re-added the postgresql secret ([#1845](https://github.com/camunda/camunda-platform-helm/issues/1845))
* Delete multi-region review comment in values.yaml ([#1819](https://github.com/camunda/camunda-platform-helm/issues/1819))
* use component service account ([#1753](https://github.com/camunda/camunda-platform-helm/issues/1753))
* handle type of es url correctly ([#1752](https://github.com/camunda/camunda-platform-helm/issues/1752))
* set zeebe exporters empty map when disabled ([#1743](https://github.com/camunda/camunda-platform-helm/issues/1743))
* add constraints for when identity is disabled and keycloak is enabled ([#1717](https://github.com/camunda/camunda-platform-helm/issues/1717))

### Refactor

* add support for seccomp profiles in all components ([#1973](https://github.com/camunda/camunda-platform-helm/issues/1973))
* update identity application yaml to match upstream ([#1737](https://github.com/camunda/camunda-platform-helm/issues/1737))

