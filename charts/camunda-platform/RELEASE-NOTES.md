The changelog is automatically generated using [git-chglog](https://github.com/git-chglog/git-chglog)
and it follows [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) format.


<a name="camunda-platform-10.0.0"></a>
## [camunda-platform-10.0.0](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-9.3.3...camunda-platform-10.0.0) (2024-04-08)

### Feat

* AWS IRSA Addition ([#1545](https://github.com/camunda/camunda-platform-helm/issues/1545))
* override application config ([#1297](https://github.com/camunda/camunda-platform-helm/issues/1297))
* external elasticsearch ([#1356](https://github.com/camunda/camunda-platform-helm/issues/1356))
* support customizing ingress pathType ([#1509](https://github.com/camunda/camunda-platform-helm/issues/1509))
* add zeebe gateway REST API ([#1278](https://github.com/camunda/camunda-platform-helm/issues/1278))

### Fix

* keycloak path defaults to having trailing / ([#1557](https://github.com/camunda/camunda-platform-helm/issues/1557))
* broken helm upgrades due to matchLabels including image version tags ([#1533](https://github.com/camunda/camunda-platform-helm/issues/1533))
* add camundaPlatform.imageTagByParams to component helpers.tpl ([#1522](https://github.com/camunda/camunda-platform-helm/issues/1522))
* references to grpc:// protocol removed ([#1521](https://github.com/camunda/camunda-platform-helm/issues/1521))
* defining resources and limits for optimize migration container ([#1487](https://github.com/camunda/camunda-platform-helm/issues/1487))
* remove extra char in IDENTITY_URL ([#1480](https://github.com/camunda/camunda-platform-helm/issues/1480))

### Refactor

* adding zeebe rest config for tasklist ([#1494](https://github.com/camunda/camunda-platform-helm/issues/1494))
* upgrade keycloak from v22 to v23 ([#1229](https://github.com/camunda/camunda-platform-helm/issues/1229))
* upgrade elasticsearch from 8.9 to 8.12 ([#1474](https://github.com/camunda/camunda-platform-helm/issues/1474))
* rename zeebe-gateway key from to zeebeGateway ([#1459](https://github.com/camunda/camunda-platform-helm/issues/1459))
* rename web-modeler postgres key to webModelerPostgresql ([#1451](https://github.com/camunda/camunda-platform-helm/issues/1451))
* move identity and its dependencies to top-level ([#1448](https://github.com/camunda/camunda-platform-helm/issues/1448))
* use the unified uid/gid 1001 for all components ([#1452](https://github.com/camunda/camunda-platform-helm/issues/1452))

### BREAKING CHANGE


zeebeGateway.ingress has been changed to zeebeGateway.ingress.rest and zeebeGateway.ingress.grpc

Check the migration steps for more details:
https://docs.camunda.io/docs/self-managed/platform-deployment/helm-kubernetes/upgrade/#version-update-instructions

Following the unified naming convention, the following values file keys have been changed 1-to-1:
- "zeebe-gateway" became "zeebeGateway"

Check the migration steps for more details:
https://docs.camunda.io/docs/next/self-managed/platform-deployment/helm-kubernetes/upgrade/#version-update-instructions

Following the unified naming convention, the following values file keys have been changed 1-to-1:
- "postgresql" became "webModelerPostgresql"

Check the migration steps for more details:
https://docs.camunda.io/docs/next/self-managed/platform-deployment/helm-kubernetes/upgrade/#version-update-instructions

Following the unified naming convention, the following values file keys have been changed 1-to-1:
- "identity.keycloak" became "identityKeycloak"
- "identity.postgresql" became "identityPostgresql"

Check the migration steps for more details:
https://docs.camunda.io/docs/next/self-managed/platform-deployment/helm-kubernetes/upgrade/#version-update-instructions

