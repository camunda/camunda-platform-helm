The changelog is automatically generated using [git-chglog](https://github.com/git-chglog/git-chglog)
and it follows [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) format.


<a name="camunda-platform-8.3.0"></a>
## camunda-platform-8.3.0 (2023-10-10)

### Build

* release 8.0.12
* release 8.0.11
* release 8.0.10
* release 8.0.9
* release 8.0.8
* release 8.0.7
* release 8.0.6
* release 8.0.5
* release 8.0.4
* release 8.0.3
* release 8.0.2
* release 8.0.1
* update release script
* release 8.0.0
* release 0.0.30
* create release 0.0.29

### Ci

* use the latest released chart in the upgrade flow ([#918](https://github.com/camunda/camunda-platform-helm/issues/918))
* test chart upgrade by default in the ci pipeline ([#914](https://github.com/camunda/camunda-platform-helm/issues/914))
* run unit tests as groups ([#904](https://github.com/camunda/camunda-platform-helm/issues/904))
* fix renovate es docker matching ([#877](https://github.com/camunda/camunda-platform-helm/issues/877))
* support persistent with pr labels ([#837](https://github.com/camunda/camunda-platform-helm/issues/837))
* copy values files to gh-pages branch ([#792](https://github.com/camunda/camunda-platform-helm/issues/792))
* test chart in full setup with Ingress and TLS ([#728](https://github.com/camunda/camunda-platform-helm/issues/728))
* remove old mechanism to update image tags ([#713](https://github.com/camunda/camunda-platform-helm/issues/713))
* support web-modeler and connectors in the update image pipeline ([#640](https://github.com/camunda/camunda-platform-helm/issues/640))
* remove old integration tests ([#596](https://github.com/camunda/camunda-platform-helm/issues/596))
* add all integration scenarios to venom ([#551](https://github.com/camunda/camunda-platform-helm/issues/551))
* enhance k8s/gke integration tests ([#497](https://github.com/camunda/camunda-platform-helm/issues/497))

### Docs

* update the versioning details ([#791](https://github.com/camunda/camunda-platform-helm/issues/791))
* add compatibility matrix ([#752](https://github.com/camunda/camunda-platform-helm/issues/752))
* includes note about adding environment variables using env option ([#731](https://github.com/camunda/camunda-platform-helm/issues/731))
* connectors is enabled by default since 8.2
* add official docs link for deployment ([#577](https://github.com/camunda/camunda-platform-helm/issues/577))
* add details about c8 helm chart versioning ([#522](https://github.com/camunda/camunda-platform-helm/issues/522))
* correct info about accessing Identity over HTTP
* update default values for probes
* tidy up keycloak ingress config details
* add note about identity access over http
* added dependencies section for better visibility
* refine readme files after v8.1 release
* use correct parameter for extraPorts
* fix zeebe-gateway config link
* fix zeebe-gateway config link
* fix zeebe-gateway config link
* use absolute path
* use absolute path
* use absolute path
* document gateway ingress values
* update javaOpts
* javaOpts are begin with a small j
* update readme add link to keycloak/identity docs
* update prometheusServiceMonitor.scrapeInterval doc
* scrapeInterval should be less than 60s
* start operate with big letter
* use correct service name in values file
* add readme details
* update readme doc for new property
* update documentation in values file
* update helper doc
* set default cpu/io thread count to 3
* add Optimize to NOTES.txt
* adjust resources
* fix readme
* start product names with capital letter
* add optimize to readme
* adjust values doc
* update charts/camunda-platform/README.md
* improve identity.enabled documentation
* improve readme
* add link to chmod calculator
* update firstuser docs
* fix typo
* update readme
* update existingSecret doc
* add values to readme
* add identity to toc

### Feat

* use read-only root filesystems for all components ([#864](https://github.com/camunda/camunda-platform-helm/issues/864))
* enable optimize upgrade process as initContainer ([#896](https://github.com/camunda/camunda-platform-helm/issues/896))
* add global key and Zeebe config for Multi-tenancy ([#890](https://github.com/camunda/camunda-platform-helm/issues/890))
* extra initContainers for all components ([#885](https://github.com/camunda/camunda-platform-helm/issues/885))
* add console self-managed initial support ([#835](https://github.com/camunda/camunda-platform-helm/issues/835))
* added ingress urls to the chart notes ([#749](https://github.com/camunda/camunda-platform-helm/issues/749))
* add podLabels to Identity ([#729](https://github.com/camunda/camunda-platform-helm/issues/729))
* customize Identity firstUser creation ([#737](https://github.com/camunda/camunda-platform-helm/issues/737))
* adds sidecar option to all components ([#723](https://github.com/camunda/camunda-platform-helm/issues/723))
* support external url templating based on global ingress host ([#722](https://github.com/camunda/camunda-platform-helm/issues/722))
* add check scheme to all probes ([#720](https://github.com/camunda/camunda-platform-helm/issues/720))
* values file level backporting per minor version ([#661](https://github.com/camunda/camunda-platform-helm/issues/661))
* zeebe gateway authentication ([#634](https://github.com/camunda/camunda-platform-helm/issues/634))
* update elastic to 7.17.9 ([#633](https://github.com/camunda/camunda-platform-helm/issues/633))
* introduce inbound connectors ([#583](https://github.com/camunda/camunda-platform-helm/issues/583))
* remove hiding of logout button in Optimize
* add Connectors component without authentication ([#566](https://github.com/camunda/camunda-platform-helm/issues/566))
* support extra volume mounts for web-modeler ([#548](https://github.com/camunda/camunda-platform-helm/issues/548))
* expose identity metrics port ([#545](https://github.com/camunda/camunda-platform-helm/issues/545))
* add initial support for Keycloak v19 ([#534](https://github.com/camunda/camunda-platform-helm/issues/534))
* add initial support for Keycloak v19 ([#534](https://github.com/camunda/camunda-platform-helm/issues/534))
* add new subchart for Web Modeler ([#500](https://github.com/camunda/camunda-platform-helm/issues/500))
* support startupProbe, readinessProbe, and livenessProbe
* use identity theme in keycloak login page
* support customizing container image registry
* support using custom key for keycloak existing secret
* support custom keycloak context path
* allow using external/existing keycloak ([#451](https://github.com/camunda/camunda-platform-helm/issues/451))
* allow to exclude components in combined the ingress
* set jwt tokens source for operate, optimize, and tasklist
* config tasklist elasticsearch url
* configure securityContext for pod and container
* support single domain setup
* add tls configuration to tasklist ingress
* added labels to elasticsearch pvc
* add extraVolumes and extraVolumeMounts to tasklist chart
* configure default mode for OpenShift
* add affinity configuration to identity
* add podAnnotations to identity
* add podAnnotations to tasklist
* add podAnnotations to optimize
* add podAnnotations to operate
* Add the possibility to Override the keycloak service name
* configure imagePullSecrets for all subcharts
* extend tasklist configuration
* set default cpu/io thread counts to 3
* allow more configuration of LoadBalancer
* hide logout
* add optimize to notes
* integrate Optimize with Identity
* add optimize sub-chart to Chart.yaml
* add default values for operate
* add optimize sub-chart
* add json schema for validating
* allow different persistence types
* add nodeSelector/affinity/tolerations and podLabels for zeebe operate and tasklist
* add initContainer cfg to gateway
* update elasticsearch
* migrate to 8.0.0
* make first user configurable
* integrate identity with tasklist
* use publicIssuerUrl in deployment
* make identity auth in operate toggleable
* use generate secret in operate
* generate secret for operate-identity
* configure identity with operate

### Fix

* use new disk usage configs for zeebe ([#927](https://github.com/camunda/camunda-platform-helm/issues/927))
* correct command value in all components ([#869](https://github.com/camunda/camunda-platform-helm/issues/869))
* set missing tasklist service account ([#842](https://github.com/camunda/camunda-platform-helm/issues/842))
* correct backups endpoint in operate ([#814](https://github.com/camunda/camunda-platform-helm/issues/814))
* remove callback url suffix from optimize root redirect url ([#780](https://github.com/camunda/camunda-platform-helm/issues/780))
* use new curator image ([#755](https://github.com/camunda/camunda-platform-helm/issues/755))
* invalid redirect uri in the apps ([#715](https://github.com/camunda/camunda-platform-helm/issues/715))
* add optimize mapping for redirect url env var ([#672](https://github.com/camunda/camunda-platform-helm/issues/672))
* identity connectors secret key
* connectors secret in identity ([#630](https://github.com/camunda/camunda-platform-helm/issues/630))
* add contextPath to components url ([#629](https://github.com/camunda/camunda-platform-helm/issues/629))
* remove support for old built-in keycloak v16 ([#625](https://github.com/camunda/camunda-platform-helm/issues/625))
* add CAMUNDA_OPERATE_IDENTITY_REDIRECT_ROOT_URL var ([#606](https://github.com/camunda/camunda-platform-helm/issues/606))
* add `CAMUNDA_TASKLIST_IDENTITY_REDIRECT_ROOT_URL` var ([#598](https://github.com/camunda/camunda-platform-helm/issues/598))
* set elasticsearch exporter prefix index config ([#532](https://github.com/camunda/camunda-platform-helm/issues/532))
* use keycloak 7.1.6 from camunda repo index
* correct optimize spring uri env var name ([#505](https://github.com/camunda/camunda-platform-helm/issues/505))
* set keycloak proxy to global ingress tls
* use service for keycloak instead of host
* put keycloak inside identity section in NOTES.txt
* update camunda-platform chart appVersion
* remove identity/optimize vars when auth is disabled
* operate logging level package name ([#410](https://github.com/camunda/camunda-platform-helm/issues/410))
* unify sub-charts icon
* unify imagePullPolicy across components ([#397](https://github.com/camunda/camunda-platform-helm/issues/397))
* update changelog for camunda-platform 8.0.12
* optimize tls termination on ingress
* use correct indentation
* use existingSecret string as secret value
* use correct type
* add conditions for zeebe notes
* use correct image tag for operate in notes
* use correct image tag for tasklist in notes
* adjust resource defaults
* set partitionCount
* allow to disable optimize with own value
* add publicIssuerUrl to identity chart
* add condition to notes
* make configmap defaultMode for tasklist configurable
* make configmap defaultMode for operate configurable
* make configmap defaultMode for gateway configurable
* make configmap default mode configurable
* adjust ports in IT
* merge conditional
* improve wording
* set initRootUrl from values file
* use more recent version
* add missing env vars
* adjust golden file

### Refactor

* migrate to elasticsearch 8 ([#884](https://github.com/camunda/camunda-platform-helm/issues/884))
* upgrade Keycloak from v19 to v22 ([#889](https://github.com/camunda/camunda-platform-helm/issues/889))
* use jdbc url in web-modeler api db config ([#748](https://github.com/camunda/camunda-platform-helm/issues/748))
* increased ingress proxy-buffer-size ([#902](https://github.com/camunda/camunda-platform-helm/issues/902))
* support non-root user by default in zeebe ([#778](https://github.com/camunda/camunda-platform-helm/issues/778))
* disable operate client in connectors if inbound is disabled ([#756](https://github.com/camunda/camunda-platform-helm/issues/756))
* print zeebe env vars in debug mode only ([#712](https://github.com/camunda/camunda-platform-helm/issues/712))
* enable connectors by default ([#603](https://github.com/camunda/camunda-platform-helm/issues/603))
* switch keycloak from v16 to v19 ([#602](https://github.com/camunda/camunda-platform-helm/issues/602))
* enable readinessProbe by default for all components ([#601](https://github.com/camunda/camunda-platform-helm/issues/601))
* migrate Web Modeler subchart to parent chart
* customize identity/keycloak combined ingress endpoint ([#549](https://github.com/camunda/camunda-platform-helm/issues/549))
* upgrade k8s api from policy/v1beta1 to policy/v1 ([#525](https://github.com/camunda/camunda-platform-helm/issues/525))
* enhance elasticsearch config ([#531](https://github.com/camunda/camunda-platform-helm/issues/531))
* add operate default actuator endpoints
* add tasklist default actuator endpoints
* use external keycloak host directly
* rename ccms-service-monitor to use the release name
* allow more flexible keycloak config
* simplify code
* introduce constant
* rm functions from it file
* move all setup related functions
* move all connection related functions
* move all login related functions
* move struct into separate file
* improve script
* replace tasklist name
* rename ram to memory
* redirectUrl and release notes
* rework the global values
* update release notes
* apply review hints
* adjust release notes
* rewrite/simplify test methods
* generate golden files
* rename and add doc
* use camunda platform in notest
* replace ccsm
* replace CCSM
* replace ccsm-
* replace ccsm.
* replace ccsm-helm
* iterate over readme
* replace Camunda Platform self managed
* replace Camunda Cloud Self Managed
* replace camunda-cloud-self-managed -> camunda-platform
* copy chart to camunda-platform

### Style

* fix indent
* gofmt
* gofmt
* gofmt
* gofmt

### Test

* fix optimize init container
* retry within the integration tests ([#657](https://github.com/camunda/camunda-platform-helm/issues/657))
* relax the timeouts
* add web-modeler integration tests ([#616](https://github.com/camunda/camunda-platform-helm/issues/616))
* increase the retry for connecotrs check
* configure securityContext for pod and container
* support single domain setup
* add tasklist ingress unit tests
* extraVolumes and extraVolumeMounts in tasklist chart
* update golden files
* configure imagePullSecrets for all subcharts
* add upgrade test
* list pods via release name
* adjust template tests
* update golden files
* add license
* add optimize login
* rename release
* disable charts
* gen golden files
* update charts/camunda-platform/test/global_deployment_test.go
* remove test
* gen golden files
* Update charts/camunda-platform/test/optimize/goldenfiles_test.go
* add template tests for optimize
* verify that Optimize is disabled
* adjust package name
* adjust identity golden file
* gofmt
* add new it test
* gofmt
* gen golden statefulset file
* gen golden files
* gen goldenfiles
* adjust tasklist tests for identity integration
* adjust integration deploy tests
* gen tasklist-secret.golden.yaml
* adjust golden files + tests
* adjust IT for tasklist login
* ignore gen. secret in golden test
* add test for operate deployment
* add test for identity deployment
* add operate-secret to golden files
* login and query operate works now
* try with jwt token

### Pull Requests

* Merge pull request [#569](https://github.com/camunda/camunda-platform-helm/issues/569) from camunda/web-modeler-3179-context-path
* Merge pull request [#585](https://github.com/camunda/camunda-platform-helm/issues/585) from camunda/web-modeler-0.8.0-beta
* Merge pull request [#565](https://github.com/camunda/camunda-platform-helm/issues/565) from camunda/web-modeler-3180-probes
* Merge pull request [#340](https://github.com/camunda/camunda-platform-helm/issues/340) from camunda/falko-patch-1
* Merge pull request [#363](https://github.com/camunda/camunda-platform-helm/issues/363) from camunda/349-openshift-it
* Merge pull request [#350](https://github.com/camunda/camunda-platform-helm/issues/350) from camunda/falko-patch-2
* Merge pull request [#344](https://github.com/camunda/camunda-platform-helm/issues/344) from camunda/343-gateway-ingress
* Merge pull request [#325](https://github.com/camunda/camunda-platform-helm/issues/325) from falko/documentation-links
* Merge pull request [#331](https://github.com/camunda/camunda-platform-helm/issues/331) from RobertRad/patch-1
* Merge pull request [#327](https://github.com/camunda/camunda-platform-helm/issues/327) from camunda/zell-303-pod-annotations
* Merge pull request [#312](https://github.com/camunda/camunda-platform-helm/issues/312) from camunda/issue-310
* Merge pull request [#311](https://github.com/camunda/camunda-platform-helm/issues/311) from camunda/zell-309-upgrade-secrets
* Merge pull request [#306](https://github.com/camunda/camunda-platform-helm/issues/306) from lemrouch/feat-loadbalancer
* Merge pull request [#300](https://github.com/camunda/camunda-platform-helm/issues/300) from camunda/zell-opt-resources
* Merge pull request [#297](https://github.com/camunda/camunda-platform-helm/issues/297) from camunda/zell-opt-it
* Merge pull request [#296](https://github.com/camunda/camunda-platform-helm/issues/296) from camunda/zell-opt-readme
* Merge pull request [#292](https://github.com/camunda/camunda-platform-helm/issues/292) from camunda/zell-partition-count
* Merge pull request [#257](https://github.com/camunda/camunda-platform-helm/issues/257) from mihneastaub/mihneastaub-podlabels-affinity
* Merge pull request [#278](https://github.com/camunda/camunda-platform-helm/issues/278) from camunda/zell-177-init-containers
* Merge pull request [#275](https://github.com/camunda/camunda-platform-helm/issues/275) from camunda/zell-update-elastic
* Merge pull request [#274](https://github.com/camunda/camunda-platform-helm/issues/274) from camunda/zell-release8

### BREAKING CHANGE


If you are using Camunda 8.2.x Helm chart, please follow the Camunda 8.3 upgrade guide.

https://docs.camunda.io/docs/self-managed/platform-deployment/helm-kubernetes/upgrade/#version-update-instructions

configuration key "web-modeler" renamed to "webModeler"; postgresql chart dependency disabled by default

2 vars have been changed as following:
- The var ".global.identity.keycloak.fullname" is deprecated
  in favour of ".global.identity.keycloak.url".
- The var ".global.identity.keycloak.url" is now a dict/map instead of
  string value.

