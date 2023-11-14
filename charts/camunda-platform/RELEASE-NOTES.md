The changelog is automatically generated using [git-chglog](https://github.com/git-chglog/git-chglog)
and it follows [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) format.


<a name="camunda-platform-8.3.2"></a>
## [camunda-platform-8.3.2](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.3.1...camunda-platform-8.3.2) (2023-11-14)

### Fix

* loads env vars for external database when external database is enabled ([#1029](https://github.com/camunda/camunda-platform-helm/issues/1029))
* optimize to use consistent versioning with other 8.3 components ([#1014](https://github.com/camunda/camunda-platform-helm/issues/1014))
* set the correct name for zeebe pvcStorageClassName
* fixes serviceMonitors for identity, connectors, and modeler

### Refactor

* add volume mounts for optimize migration init container ([#1037](https://github.com/camunda/camunda-platform-helm/issues/1037))

### Reverts

* Release Camunda Platform Helm Chart v8.3.2 ([#1067](https://github.com/camunda/camunda-platform-helm/issues/1067))

