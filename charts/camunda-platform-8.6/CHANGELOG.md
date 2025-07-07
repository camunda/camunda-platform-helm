# Changelog

## [11.7.0](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.6-11.6.0...camunda-platform-8.6-11.7.0) (2025-07-07)


### Features

* 8.8 integration test playwright ([#3675](https://github.com/camunda/camunda-platform-helm/issues/3675)) ([ef7d0a4](https://github.com/camunda/camunda-platform-helm/commit/ef7d0a43705da24e8b1e476d61d4c2b12d6905d4))
* add extraVolumeClaimTemplates support ([#3710](https://github.com/camunda/camunda-platform-helm/issues/3710)) ([3fad97e](https://github.com/camunda/camunda-platform-helm/commit/3fad97ebc60d5e99a7fb860d70326f7531ff0cba))
* **images:** add values for enterprise images ([#3768](https://github.com/camunda/camunda-platform-helm/issues/3768)) ([5519fb9](https://github.com/camunda/camunda-platform-helm/commit/5519fb9c6c17b4362e264b88508c48a9b945b3d3))
* implement mutable global.commonLabels value ([#3717](https://github.com/camunda/camunda-platform-helm/issues/3717)) ([d614485](https://github.com/camunda/camunda-platform-helm/commit/d614485d67afb0f1ea65b98039d2349bf91fe09b))


### Bug Fixes

* add scheme into templates for use by connectors ([#3555](https://github.com/camunda/camunda-platform-helm/issues/3555)) ([dd3f213](https://github.com/camunda/camunda-platform-helm/commit/dd3f21308064ba7836188cc1b6333de58c79410b))
* replace GKE with teleport EKS cluster ([#2832](https://github.com/camunda/camunda-platform-helm/issues/2832)) ([e9bb416](https://github.com/camunda/camunda-platform-helm/commit/e9bb4162739b6bb39f40918c4b99c566422f8155))
* update params for existingSecret  ([#3583](https://github.com/camunda/camunda-platform-helm/issues/3583)) ([838c2c8](https://github.com/camunda/camunda-platform-helm/commit/838c2c80c544651a09c10dfaa9127900b2d8a946))


### Dependencies

* update camunda-platform-8.6 to v8.6.20 (patch) ([#3703](https://github.com/camunda/camunda-platform-helm/issues/3703)) ([665803d](https://github.com/camunda/camunda-platform-helm/commit/665803d0e7f9cc6019877dd1dfd0580ae01bf8c4))
* update camunda-platform-8.6 to v8.6.21 (patch) ([#3747](https://github.com/camunda/camunda-platform-helm/issues/3747)) ([8dba1e3](https://github.com/camunda/camunda-platform-helm/commit/8dba1e3d5b5e15c4228dd203ab0e9ad5d03af66e))
* update camunda/connectors-bundle docker tag to v8.6.15 ([#3711](https://github.com/camunda/camunda-platform-helm/issues/3711)) ([8f74134](https://github.com/camunda/camunda-platform-helm/commit/8f74134163a0f31c87c9364d0a09124d6e2a2e8d))
* update camunda/identity docker tag to v8.6.16 ([#3707](https://github.com/camunda/camunda-platform-helm/issues/3707)) ([21fd222](https://github.com/camunda/camunda-platform-helm/commit/21fd222c1f1a0788e712c70ed99c222ce2ed6e0c))
* update camunda/identity docker tag to v8.6.17 ([#3742](https://github.com/camunda/camunda-platform-helm/issues/3742)) ([9022d9c](https://github.com/camunda/camunda-platform-helm/commit/9022d9c4f188c414c0f0410f43c0f66b71f939dc))
* update camunda/operate docker tag to v8.6.20 ([#3704](https://github.com/camunda/camunda-platform-helm/issues/3704)) ([90029d2](https://github.com/camunda/camunda-platform-helm/commit/90029d2964e4dcad12f078e2fc873aee8553b842))
* update camunda/optimize docker tag to v8.6.12 ([#3760](https://github.com/camunda/camunda-platform-helm/issues/3760)) ([daad9f9](https://github.com/camunda/camunda-platform-helm/commit/daad9f955dd8156c269f54b52489693cada135a5))
* update camunda/web-modeler-restapi docker tag to v8.6.15 ([#3735](https://github.com/camunda/camunda-platform-helm/issues/3735)) ([5a0be52](https://github.com/camunda/camunda-platform-helm/commit/5a0be5202600e55fff137a459c55b58dcc7b0813))
* update tool-versions (patch) ([#3630](https://github.com/camunda/camunda-platform-helm/issues/3630)) ([f91804d](https://github.com/camunda/camunda-platform-helm/commit/f91804d518bfd35af75f8336a861d37867c0ff0d))


### Refactors

* add local values file per chart version ([#3679](https://github.com/camunda/camunda-platform-helm/issues/3679)) ([cd3064d](https://github.com/camunda/camunda-platform-helm/commit/cd3064d2cf7daa17f6a9c2430687e2a013179be5))
