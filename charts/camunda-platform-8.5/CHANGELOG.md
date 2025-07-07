# Changelog

## [10.9.0](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.5-10.8.0...camunda-platform-8.5-10.9.0) (2025-07-07)


### Features

* 8.8 integration test playwright ([#3675](https://github.com/camunda/camunda-platform-helm/issues/3675)) ([ef7d0a4](https://github.com/camunda/camunda-platform-helm/commit/ef7d0a43705da24e8b1e476d61d4c2b12d6905d4))
* add extraVolumeClaimTemplates support ([#3710](https://github.com/camunda/camunda-platform-helm/issues/3710)) ([3fad97e](https://github.com/camunda/camunda-platform-helm/commit/3fad97ebc60d5e99a7fb860d70326f7531ff0cba))
* **images:** add values for enterprise images ([#3768](https://github.com/camunda/camunda-platform-helm/issues/3768)) ([5519fb9](https://github.com/camunda/camunda-platform-helm/commit/5519fb9c6c17b4362e264b88508c48a9b945b3d3))
* implement mutable global.commonLabels value ([#3717](https://github.com/camunda/camunda-platform-helm/issues/3717)) ([d614485](https://github.com/camunda/camunda-platform-helm/commit/d614485d67afb0f1ea65b98039d2349bf91fe09b))
* support configure replicas for web modeler restapi and webapp ([#3638](https://github.com/camunda/camunda-platform-helm/issues/3638)) ([11bc1c5](https://github.com/camunda/camunda-platform-helm/commit/11bc1c5d426a73a36d5ac71099ab217e4799f6ca))


### Bug Fixes

* add scheme into templates for use by connectors ([#3555](https://github.com/camunda/camunda-platform-helm/issues/3555)) ([dd3f213](https://github.com/camunda/camunda-platform-helm/commit/dd3f21308064ba7836188cc1b6333de58c79410b))
* replace GKE with teleport EKS cluster ([#2832](https://github.com/camunda/camunda-platform-helm/issues/2832)) ([e9bb416](https://github.com/camunda/camunda-platform-helm/commit/e9bb4162739b6bb39f40918c4b99c566422f8155))
* update params for existingSecret  ([#3583](https://github.com/camunda/camunda-platform-helm/issues/3583)) ([838c2c8](https://github.com/camunda/camunda-platform-helm/commit/838c2c80c544651a09c10dfaa9127900b2d8a946))


### Dependencies

* update camunda-platform-8.5 (patch) ([#3749](https://github.com/camunda/camunda-platform-helm/issues/3749)) ([03d5de4](https://github.com/camunda/camunda-platform-helm/commit/03d5de4521193621583106c90b34b1a611516bfe))
* update camunda-platform-8.5 to v8.5.18 (patch) ([#3739](https://github.com/camunda/camunda-platform-helm/issues/3739)) ([ebfe74c](https://github.com/camunda/camunda-platform-helm/commit/ebfe74cc84b542e2e52e1bcd2d2e3ba689bca37b))
* update camunda/connectors-bundle docker tag to v8.5.18 ([#3716](https://github.com/camunda/camunda-platform-helm/issues/3716)) ([a11bb54](https://github.com/camunda/camunda-platform-helm/commit/a11bb54fac78c28f5cbcb09905647ed6c6615eaa))
* update camunda/web-modeler docker tag to v8.5.20 ([#3737](https://github.com/camunda/camunda-platform-helm/issues/3737)) ([4f6e10e](https://github.com/camunda/camunda-platform-helm/commit/4f6e10e64d51df9c3b34c855563cb9fe5e5d375f))
* update module github.com/gruntwork-io/terratest to v0.50.0 ([#3658](https://github.com/camunda/camunda-platform-helm/issues/3658)) ([46ebb36](https://github.com/camunda/camunda-platform-helm/commit/46ebb36a3fba071031c9d6fc2c0c61123c07ea48))


### Refactors

* add local values file per chart version ([#3679](https://github.com/camunda/camunda-platform-helm/issues/3679)) ([cd3064d](https://github.com/camunda/camunda-platform-helm/commit/cd3064d2cf7daa17f6a9c2430687e2a013179be5))
