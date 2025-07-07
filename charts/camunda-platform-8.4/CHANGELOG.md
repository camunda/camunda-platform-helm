# Changelog

## [9.7.0](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.4-9.6.0...camunda-platform-8.4-9.7.0) (2025-07-07)


### Features

* 8.8 integration test playwright ([#3675](https://github.com/camunda/camunda-platform-helm/issues/3675)) ([ef7d0a4](https://github.com/camunda/camunda-platform-helm/commit/ef7d0a43705da24e8b1e476d61d4c2b12d6905d4))
* add extraVolumeClaimTemplates support ([#3710](https://github.com/camunda/camunda-platform-helm/issues/3710)) ([3fad97e](https://github.com/camunda/camunda-platform-helm/commit/3fad97ebc60d5e99a7fb860d70326f7531ff0cba))
* implement mutable global.commonLabels value ([#3717](https://github.com/camunda/camunda-platform-helm/issues/3717)) ([d614485](https://github.com/camunda/camunda-platform-helm/commit/d614485d67afb0f1ea65b98039d2349bf91fe09b))


### Bug Fixes

* add scheme into templates for use by connectors ([#3555](https://github.com/camunda/camunda-platform-helm/issues/3555)) ([dd3f213](https://github.com/camunda/camunda-platform-helm/commit/dd3f21308064ba7836188cc1b6333de58c79410b))
* replace GKE with teleport EKS cluster ([#2832](https://github.com/camunda/camunda-platform-helm/issues/2832)) ([e9bb416](https://github.com/camunda/camunda-platform-helm/commit/e9bb4162739b6bb39f40918c4b99c566422f8155))
* update params for existingSecret  ([#3583](https://github.com/camunda/camunda-platform-helm/issues/3583)) ([838c2c8](https://github.com/camunda/camunda-platform-helm/commit/838c2c80c544651a09c10dfaa9127900b2d8a946))


### Dependencies

* update camunda-platform-8.4 (patch) ([#3746](https://github.com/camunda/camunda-platform-helm/issues/3746)) ([af8bde5](https://github.com/camunda/camunda-platform-helm/commit/af8bde5c5d2877b986031d79b36a461c8dab3fc7))
* update camunda/connectors-bundle docker tag to v8.4.21 ([#3715](https://github.com/camunda/camunda-platform-helm/issues/3715)) ([1270104](https://github.com/camunda/camunda-platform-helm/commit/12701042b3e6da44e6ef5656bb4f7248ff7297b0))
* update camunda/identity docker tag to v8.4.23 ([#3740](https://github.com/camunda/camunda-platform-helm/issues/3740)) ([a375bc7](https://github.com/camunda/camunda-platform-helm/commit/a375bc75b3c226b903ef76d12a475b63752b0f0a))
* update camunda/operate docker tag to v8.4.22 ([#3762](https://github.com/camunda/camunda-platform-helm/issues/3762)) ([91af259](https://github.com/camunda/camunda-platform-helm/commit/91af259f40c885f59e7d141cf2822348ccd74639))
* update camunda/web-modeler docker tag to v8.4.19 ([#3736](https://github.com/camunda/camunda-platform-helm/issues/3736)) ([247bfa3](https://github.com/camunda/camunda-platform-helm/commit/247bfa33fa8430bbbe958b0de419cc561193c65c))
* update module github.com/gruntwork-io/terratest to v0.50.0 ([#3658](https://github.com/camunda/camunda-platform-helm/issues/3658)) ([46ebb36](https://github.com/camunda/camunda-platform-helm/commit/46ebb36a3fba071031c9d6fc2c0c61123c07ea48))


### Refactors

* add local values file per chart version ([#3679](https://github.com/camunda/camunda-platform-helm/issues/3679)) ([cd3064d](https://github.com/camunda/camunda-platform-helm/commit/cd3064d2cf7daa17f6a9c2430687e2a013179be5))
