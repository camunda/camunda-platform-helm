The changelog is automatically generated using [git-chglog](https://github.com/git-chglog/git-chglog)
and it follows [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) format.


<a name="camunda-platform-8.1.7"></a>
## [camunda-platform-8.1.7](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.1.6...camunda-platform-8.1.7) (2023-03-29)

### Ci

* add all integration scenarios to venom ([#551](https://github.com/camunda/camunda-platform-helm/issues/551))

### Feat

* remove hiding of logout button in Optimize
* add Connectors component without authentication ([#566](https://github.com/camunda/camunda-platform-helm/issues/566))
* support web-modeler startup/readiness/liveness probes

### Refactor

* remove hiding of logout button in Optimize
* migrate Web Modeler subchart to parent chart
* update web-modeler to version 0.8.0-beta

### Pull Requests

* Merge pull request [#569](https://github.com/camunda/camunda-platform-helm/issues/569) from camunda/web-modeler-3179-context-path
* Merge pull request [#585](https://github.com/camunda/camunda-platform-helm/issues/585) from camunda/web-modeler-0.8.0-beta
* Merge pull request [#565](https://github.com/camunda/camunda-platform-helm/issues/565) from camunda/web-modeler-3180-probes

### BREAKING CHANGE

Beta component: configuration key "web-modeler" renamed to "webModeler"; postgresql chart dependency disabled by default
