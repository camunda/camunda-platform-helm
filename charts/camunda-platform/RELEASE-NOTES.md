The changelog is automatically generated using [git-chglog](https://github.com/git-chglog/git-chglog)
and it follows [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) format.


<a name="camunda-platform-8.2.0"></a>
## [camunda-platform-8.2.0](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.1.7...camunda-platform-8.2.0) (2023-04-11)

### Ci

* remove old integration tests ([#596](https://github.com/camunda/camunda-platform-helm/issues/596))

### Docs

* add official docs link for deployment ([#577](https://github.com/camunda/camunda-platform-helm/issues/577))

### Feat

* introduce inbound connectors ([#583](https://github.com/camunda/camunda-platform-helm/issues/583))

### Fix

* fix redirection issue in operate ([#606](https://github.com/camunda/camunda-platform-helm/issues/606))
* fix redirection issue in tasklist ([#598](https://github.com/camunda/camunda-platform-helm/issues/598))

### Refactor

* enable connectors by default ([#603](https://github.com/camunda/camunda-platform-helm/issues/603))
* switch keycloak from v16 to v19 ([#602](https://github.com/camunda/camunda-platform-helm/issues/602))
* enable readinessProbe by default for all components ([#601](https://github.com/camunda/camunda-platform-helm/issues/601))

### Test

* increase the retry for connecotrs check

### BREAKING CHANGE

Switch keycloak from v16 to v19
even though it's been tested for some time, this change could be a breaking change due to switching the base chart.

old chart with Keycloak v16:
https://artifacthub.io/packages/helm/camunda/keycloak/7.1.6

new chart with Keycloak v19:
https://artifacthub.io/packages/helm/bitnami/keycloak/12.2.0

