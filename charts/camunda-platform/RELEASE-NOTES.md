The changelog is automatically generated using [git-chglog](https://github.com/git-chglog/git-chglog)
and it follows [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) format.


<a name="camunda-platform-10.0.3"></a>
## [camunda-platform-10.0.3](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-10.0.2...camunda-platform-10.0.3) (2024-05-01)

### Ci

* release chart snapshot from main branch ([#1686](https://github.com/camunda/camunda-platform-helm/issues/1686))

### Fix

*  fullUrl for identity ([#1606](https://github.com/camunda/camunda-platform-helm/issues/1606))
* align conditionals in identityPostgres ([#1707](https://github.com/camunda/camunda-platform-helm/issues/1707))
* uses issuerBackendUrl if present before trying to calculate a keycloak url ([#1703](https://github.com/camunda/camunda-platform-helm/issues/1703))
* keycloak realm in console component is hardcoded ([#1701](https://github.com/camunda/camunda-platform-helm/issues/1701))
* ensured 8.5 compatibility layer works with sub-charts ([#1700](https://github.com/camunda/camunda-platform-helm/issues/1700))
* identity configmap sets audience variable ([#1654](https://github.com/camunda/camunda-platform-helm/issues/1654))
* identity configmap to use correct audience ([#1615](https://github.com/camunda/camunda-platform-helm/issues/1615))
* mismatch errors during installation z_compatibility_helpers.tpl ([#1634](https://github.com/camunda/camunda-platform-helm/issues/1634))
* added zeebe gateway rest api port to port forwards in `helm status` ([#1645](https://github.com/camunda/camunda-platform-helm/issues/1645))

