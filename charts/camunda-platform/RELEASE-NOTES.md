The changelog is automatically generated using [git-chglog](https://github.com/git-chglog/git-chglog)
and it follows [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) format.


<a name="camunda-platform-8.1.1"></a>
## [camunda-platform-8.1.1](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.1.0...camunda-platform-8.1.1) (2022-11-07)

### Docs

* add camunda platform architecture diagram
* refine in-repo docs after v8.1 release

### Feat

* support using custom key for keycloak existing secret
* support custom keycloak context path

### Fix

* use service for keycloak instead of host
* put keycloak section under identity in NOTES.txt
* set keycloak proxy to global ingress tls
