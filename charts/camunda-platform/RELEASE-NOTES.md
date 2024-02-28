The changelog is automatically generated using [git-chglog](https://github.com/git-chglog/git-chglog)
and it follows [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) format.


<a name="camunda-platform-9.1.3"></a>
## [camunda-platform-9.1.3](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-9.1.2...camunda-platform-9.1.3) (2024-02-28)

### Feat

* support custom pvc annotations for Zeebe ([#1359](https://github.com/camunda/camunda-platform-helm/issues/1359))
* console sm alpha ([#1334](https://github.com/camunda/camunda-platform-helm/issues/1334))

### Fix

* moves ccsm spring profile outside of conditional ([#1337](https://github.com/camunda/camunda-platform-helm/issues/1337))
* update to use initialContactPoints for Zeebe Gateway ([#1353](https://github.com/camunda/camunda-platform-helm/issues/1353))
* update version matrix after latest bugfix releases ([#1358](https://github.com/camunda/camunda-platform-helm/issues/1358))
* separated ingress status in helm notes ([#1318](https://github.com/camunda/camunda-platform-helm/issues/1318))

### Test

* remove old console test config
* disable prometheus tests until prometheus Completed issue is resolved ([#1350](https://github.com/camunda/camunda-platform-helm/issues/1350))
* fix yamllint errors ([#1345](https://github.com/camunda/camunda-platform-helm/issues/1345))

