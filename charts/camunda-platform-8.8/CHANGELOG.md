# Changelog

## [13.0.0-alpha6](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.8-13.0.0-alpha5...camunda-platform-8.8-13.0.0-alpha6) (2025-07-07)


### Features

* 8.8 integration test playwright ([#3675](https://github.com/camunda/camunda-platform-helm/issues/3675)) ([ef7d0a4](https://github.com/camunda/camunda-platform-helm/commit/ef7d0a43705da24e8b1e476d61d4c2b12d6905d4))
* add alternative emptyDir PVC support for components ([#3692](https://github.com/camunda/camunda-platform-helm/issues/3692)) ([d3e4d36](https://github.com/camunda/camunda-platform-helm/commit/d3e4d369b42b6999753ebedc91d70a399d3b5dea))
* add database name, host, port values to webmodeler.restapi.externalDatabase ([#3614](https://github.com/camunda/camunda-platform-helm/issues/3614)) ([ffaa50f](https://github.com/camunda/camunda-platform-helm/commit/ffaa50f3d5d3de25ebcbf7addc8e9dacd7848362))
* add extraVolumeClaimTemplates support ([#3710](https://github.com/camunda/camunda-platform-helm/issues/3710)) ([3fad97e](https://github.com/camunda/camunda-platform-helm/commit/3fad97ebc60d5e99a7fb860d70326f7531ff0cba))
* **images:** add values for enterprise images ([#3768](https://github.com/camunda/camunda-platform-helm/issues/3768)) ([5519fb9](https://github.com/camunda/camunda-platform-helm/commit/5519fb9c6c17b4362e264b88508c48a9b945b3d3))
* implement mutable global.commonLabels value ([#3717](https://github.com/camunda/camunda-platform-helm/issues/3717)) ([d614485](https://github.com/camunda/camunda-platform-helm/commit/d614485d67afb0f1ea65b98039d2349bf91fe09b))
* move disk settings to configmap ([#3668](https://github.com/camunda/camunda-platform-helm/issues/3668)) ([79740cf](https://github.com/camunda/camunda-platform-helm/commit/79740cfda4caee8afc6ae8032ec7c3349a5322c5))
* support configure replicas for web modeler restapi and webapp ([#3638](https://github.com/camunda/camunda-platform-helm/issues/3638)) ([11bc1c5](https://github.com/camunda/camunda-platform-helm/commit/11bc1c5d426a73a36d5ac71099ab217e4799f6ca))


### Bug Fixes

* add existingSecret object to core ([#3720](https://github.com/camunda/camunda-platform-helm/issues/3720)) ([f7c53b7](https://github.com/camunda/camunda-platform-helm/commit/f7c53b73ca4802c7a196c67f59869ce9c0b31238))
* add scheme into templates for use by connectors ([#3555](https://github.com/camunda/camunda-platform-helm/issues/3555)) ([dd3f213](https://github.com/camunda/camunda-platform-helm/commit/dd3f21308064ba7836188cc1b6333de58c79410b))
* allow string literal overrides for existingSecret for all components ([86a5db3](https://github.com/camunda/camunda-platform-helm/commit/86a5db3a5b494a7b24d76805ccba208c15901aaa))
* correct rendering for global.identity.auth.identity.existingSecret.name ([4e4c1f5](https://github.com/camunda/camunda-platform-helm/commit/4e4c1f55055d0faa1b33dd3d74f91abc708153b2))
* remove the metadata in the namespace name ([#3731](https://github.com/camunda/camunda-platform-helm/issues/3731)) ([8285686](https://github.com/camunda/camunda-platform-helm/commit/8285686ba9eef26641b7dfa7e2295f0b47233418))
* replace GKE with teleport EKS cluster ([#2832](https://github.com/camunda/camunda-platform-helm/issues/2832)) ([e9bb416](https://github.com/camunda/camunda-platform-helm/commit/e9bb4162739b6bb39f40918c4b99c566422f8155))
* schema type for `global.identity.auth.identity.existingSecret` ([#3667](https://github.com/camunda/camunda-platform-helm/issues/3667)) ([50d0084](https://github.com/camunda/camunda-platform-helm/commit/50d0084771fc4a23bb8ec02e62bfdc4105050f2e))
* update params for existingSecret  ([#3583](https://github.com/camunda/camunda-platform-helm/issues/3583)) ([838c2c8](https://github.com/camunda/camunda-platform-helm/commit/838c2c80c544651a09c10dfaa9127900b2d8a946))


### Dependencies

* update camunda-platform-8.8 (patch) ([#3727](https://github.com/camunda/camunda-platform-helm/issues/3727)) ([aee7a57](https://github.com/camunda/camunda-platform-helm/commit/aee7a577a10a2f623f69b968eb1dbf087e609301))
* update camunda/optimize docker tag to v8.8.0-alpha6 ([#3771](https://github.com/camunda/camunda-platform-helm/issues/3771)) ([249d3aa](https://github.com/camunda/camunda-platform-helm/commit/249d3aa2352a8a53b6b67065ebb682aadd853550))
* update module github.com/gruntwork-io/terratest to v0.50.0 ([#3659](https://github.com/camunda/camunda-platform-helm/issues/3659)) ([a50e09f](https://github.com/camunda/camunda-platform-helm/commit/a50e09f072109822e741503b291ad86f81d3522a))
* update tool-versions (patch) ([#3630](https://github.com/camunda/camunda-platform-helm/issues/3630)) ([f91804d](https://github.com/camunda/camunda-platform-helm/commit/f91804d518bfd35af75f8336a861d37867c0ff0d))


### Refactors

* add local values file per chart version ([#3679](https://github.com/camunda/camunda-platform-helm/issues/3679)) ([cd3064d](https://github.com/camunda/camunda-platform-helm/commit/cd3064d2cf7daa17f6a9c2430687e2a013179be5))
* update rest api endpoint in web modeler config ([#3664](https://github.com/camunda/camunda-platform-helm/issues/3664)) ([928f632](https://github.com/camunda/camunda-platform-helm/commit/928f632d012d175a0c9f52d892f116753e7a7a45))
