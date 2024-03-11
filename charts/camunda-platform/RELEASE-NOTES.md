The changelog is automatically generated using [git-chglog](https://github.com/git-chglog/git-chglog)
and it follows [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) format.


<a name="camunda-platform-9.3.0"></a>
## camunda-platform-9.3.0 (2024-03-11)

### Feat

* support OIDC in identity ([#1377](https://github.com/camunda/camunda-platform-helm/issues/1377))

### Fix

* add support for automountServiceAccountToken ([#1391](https://github.com/camunda/camunda-platform-helm/issues/1391))


---------

If you are using Camunda 8.2.x Helm chart, please follow the Camunda 8.3 upgrade guide.

https://docs.camunda.io/docs/self-managed/platform-deployment/helm-kubernetes/upgrade/#version-update-instructions

configuration key "web-modeler" renamed to "webModeler"; postgresql chart dependency disabled by default

2 vars have been changed as following:
- The var ".global.identity.keycloak.fullname" is deprecated
  in favour of ".global.identity.keycloak.url".
- The var ".global.identity.keycloak.url" is now a dict/map instead of
  string value.

