The changelog is automatically generated using [git-chglog](https://github.com/git-chglog/git-chglog)
and it follows [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) format.


<a name="camunda-platform-8.3.0"></a>
## camunda-platform-8.3.0 (2023-10-10)

> **Warning**
> Updating Operate, Tasklist, and Optimize from 8.2.x to 8.3.0 will potentially take longer than expected, depending on the data to be migrated. Additionally, we identified some bugs that could also prevent the migration from succeeding. These are being addressed and will be available in an upcoming 8.3.1 patch. We suggest not updating until the patch is released.

### Ci

* use the latest released chart in the upgrade flow ([#918](https://github.com/camunda/camunda-platform-helm/issues/918))
* test chart upgrade by default in the ci pipeline ([#914](https://github.com/camunda/camunda-platform-helm/issues/914))
* run unit tests as groups ([#904](https://github.com/camunda/camunda-platform-helm/issues/904))

### Feat

* use read-only root file system for all components ([#864](https://github.com/camunda/camunda-platform-helm/issues/864))
* enable optimize upgrade process as initContainer ([#896](https://github.com/camunda/camunda-platform-helm/issues/896))
* support Multi-tenancy for all components ([#782](https://github.com/camunda/camunda-platform-helm/issues/782))
* extra initContainers for all components ([#885](https://github.com/camunda/camunda-platform-helm/issues/885))

### Fix

* use new disk usage configs for zeebe ([#927](https://github.com/camunda/camunda-platform-helm/issues/927))
* correct command value in all components ([#869](https://github.com/camunda/camunda-platform-helm/issues/869))

### Refactor

* upgrade Elasticsearch from v7 to v8 ([#884](https://github.com/camunda/camunda-platform-helm/issues/884))
* upgrade Keycloak from v19 to v22 ([#889](https://github.com/camunda/camunda-platform-helm/issues/889))
* use jdbc url in web-modeler api db config ([#748](https://github.com/camunda/camunda-platform-helm/issues/748))
* increased ingress proxy-buffer-size ([#902](https://github.com/camunda/camunda-platform-helm/issues/902))
* support non-root user by default in zeebe ([#778](https://github.com/camunda/camunda-platform-helm/issues/778))

### BREAKING CHANGE

If you are using Camunda 8.2.x Helm chart, please follow the Camunda 8.3 upgrade guide.

https://docs.camunda.io/docs/self-managed/platform-deployment/helm-kubernetes/upgrade/#version-update-instructions

- Elasticsearch upgraded from v7 to v8.
- Keycloak upgraded from v19 to v22.
- Zeebe runs as a non-root user by default.
- Web-Modeler external database syntax has been changed.
