The changelog is automatically generated using [git-chglog](https://github.com/git-chglog/git-chglog)
and it follows [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) format.


<a name="camunda-platform-8.1.0"></a>
## [camunda-platform-8.1.0](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.0.14...camunda-platform-8.1.0) (2022-10-26)

### Feat

* update Camunda Platform image tag to Global:8.1.2/Optimize:3.9.1 ([#452](https://github.com/camunda/camunda-platform-helm/issues/452)
* allow using external/existing keycloak ([#451](https://github.com/camunda/camunda-platform-helm/issues/451), [#457](https://github.com/camunda/camunda-platform-helm/issues/457))
* allow to exclude components in combined the ingress ([#454](https://github.com/camunda/camunda-platform-helm/issues/454))
* add management port to reach backup API ([#441](https://github.com/camunda/camunda-platform-helm/issues/441)

### Fix

* move keycloak inside identity section in NOTES.txt

### BREAKING CHANGE

There 2 vars have been changed as following:
- The var `.global.identity.keycloak.fullname` is deprecated
  in favour of `.global.identity.keycloak.url`.
- The var `.global.identity.keycloak.url` is now a dict/map instead of
  string value.
