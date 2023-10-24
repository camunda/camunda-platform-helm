The changelog is automatically generated using [git-chglog](https://github.com/git-chglog/git-chglog)
and it follows [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) format.


<a name="camunda-platform-8.3.1"></a>
## [camunda-platform-8.3.1](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.3.0...camunda-platform-8.3.1) (2023-10-23)

### Fix

* use the proper keycloak var for connectors ([#988](https://github.com/camunda/camunda-platform-helm/issues/988))
* configure identity base url ([#962](https://github.com/camunda/camunda-platform-helm/issues/962))
* readonly add zeebe cache path ([#960](https://github.com/camunda/camunda-platform-helm/issues/960))

### Refactor

* update accessing keycloak url ([#981](https://github.com/camunda/camunda-platform-helm/issues/981))
* allow customize optimize migration init container env vars
* allow toggle optimize migration init container
* move optimize to the main chart ([#973](https://github.com/camunda/camunda-platform-helm/issues/973))
* move zeebe/zeebe-gateway to the main chart ([#970](https://github.com/camunda/camunda-platform-helm/issues/970))
* move tasklist to the main chart ([#968](https://github.com/camunda/camunda-platform-helm/issues/968))
* move operate to the main chart ([#964](https://github.com/camunda/camunda-platform-helm/issues/964))

