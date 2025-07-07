# Changelog

## [12.3.0](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.7-12.2.0...camunda-platform-8.7-12.3.0) (2025-07-07)


### Features

* 8.8 integration test playwright ([#3675](https://github.com/camunda/camunda-platform-helm/issues/3675)) ([ef7d0a4](https://github.com/camunda/camunda-platform-helm/commit/ef7d0a43705da24e8b1e476d61d4c2b12d6905d4))
* add database name, host, port values to webmodeler.restapi.externalDatabase ([#3614](https://github.com/camunda/camunda-platform-helm/issues/3614)) ([ffaa50f](https://github.com/camunda/camunda-platform-helm/commit/ffaa50f3d5d3de25ebcbf7addc8e9dacd7848362))
* add extraVolumeClaimTemplates support ([#3710](https://github.com/camunda/camunda-platform-helm/issues/3710)) ([3fad97e](https://github.com/camunda/camunda-platform-helm/commit/3fad97ebc60d5e99a7fb860d70326f7531ff0cba))
* **images:** add values for enterprise images ([#3768](https://github.com/camunda/camunda-platform-helm/issues/3768)) ([5519fb9](https://github.com/camunda/camunda-platform-helm/commit/5519fb9c6c17b4362e264b88508c48a9b945b3d3))
* implement mutable global.commonLabels value ([#3717](https://github.com/camunda/camunda-platform-helm/issues/3717)) ([d614485](https://github.com/camunda/camunda-platform-helm/commit/d614485d67afb0f1ea65b98039d2349bf91fe09b))
* move disk settings to configmap ([#3668](https://github.com/camunda/camunda-platform-helm/issues/3668)) ([79740cf](https://github.com/camunda/camunda-platform-helm/commit/79740cfda4caee8afc6ae8032ec7c3349a5322c5))


### Bug Fixes

* add scheme into templates for use by connectors ([#3555](https://github.com/camunda/camunda-platform-helm/issues/3555)) ([dd3f213](https://github.com/camunda/camunda-platform-helm/commit/dd3f21308064ba7836188cc1b6333de58c79410b))
* connectors inbound disabled mode ([#3712](https://github.com/camunda/camunda-platform-helm/issues/3712)) ([035c090](https://github.com/camunda/camunda-platform-helm/commit/035c090c67747a1ff2393a1a3d92e6cacb1e636c))
* replace GKE with teleport EKS cluster ([#2832](https://github.com/camunda/camunda-platform-helm/issues/2832)) ([e9bb416](https://github.com/camunda/camunda-platform-helm/commit/e9bb4162739b6bb39f40918c4b99c566422f8155))
* replaced old property with new one ([#3734](https://github.com/camunda/camunda-platform-helm/issues/3734)) ([ba04791](https://github.com/camunda/camunda-platform-helm/commit/ba047910fe82f18df25bc6797aa08ad4059f7701))
* update params for existingSecret  ([#3583](https://github.com/camunda/camunda-platform-helm/issues/3583)) ([838c2c8](https://github.com/camunda/camunda-platform-helm/commit/838c2c80c544651a09c10dfaa9127900b2d8a946))


### Dependencies

* update bitnami/keycloak docker tag to v26.3.0 ([#3759](https://github.com/camunda/camunda-platform-helm/issues/3759)) ([93dccbe](https://github.com/camunda/camunda-platform-helm/commit/93dccbede96ce9e4ceb26b9b7785eeae9430ac26))
* update camunda-platform-8.7 (patch) ([#3738](https://github.com/camunda/camunda-platform-helm/issues/3738)) ([5302c90](https://github.com/camunda/camunda-platform-helm/commit/5302c90b8df93a61ddf40b5d938bcf1fa35c0362))
* update camunda-platform-8.7 (patch) ([#3743](https://github.com/camunda/camunda-platform-helm/issues/3743)) ([a47b06c](https://github.com/camunda/camunda-platform-helm/commit/a47b06c0214a4437882b682103deeb0c924c79ed))
* update camunda-platform-8.7 (patch) ([#3761](https://github.com/camunda/camunda-platform-helm/issues/3761)) ([6f2d0e3](https://github.com/camunda/camunda-platform-helm/commit/6f2d0e391003ecd7f3b89d4a2d882b260a220dfd))
* update camunda/console docker tag to v8.7.33 ([#3694](https://github.com/camunda/camunda-platform-helm/issues/3694)) ([2e9287c](https://github.com/camunda/camunda-platform-helm/commit/2e9287c77e0513d3efc4e6a72d8da7fb109fa6bd))
* update camunda/console docker tag to v8.7.34 ([#3700](https://github.com/camunda/camunda-platform-helm/issues/3700)) ([72948f8](https://github.com/camunda/camunda-platform-helm/commit/72948f8142243153568b3fa1a762a6e09de4cf47))
* update camunda/console docker tag to v8.7.36 ([#3701](https://github.com/camunda/camunda-platform-helm/issues/3701)) ([4841386](https://github.com/camunda/camunda-platform-helm/commit/4841386c27f031ec450c65f29aa1e3411a211454))
* update camunda/console docker tag to v8.7.37 ([#3709](https://github.com/camunda/camunda-platform-helm/issues/3709)) ([1e8e373](https://github.com/camunda/camunda-platform-helm/commit/1e8e373b63e4ae6e7b0f98ba029e8c74c6345b9f))
* update camunda/console docker tag to v8.7.38 ([#3721](https://github.com/camunda/camunda-platform-helm/issues/3721)) ([45fb982](https://github.com/camunda/camunda-platform-helm/commit/45fb982681167667bd38f5e2393e4e2afc090bf4))
* update camunda/console docker tag to v8.7.39 ([#3726](https://github.com/camunda/camunda-platform-helm/issues/3726)) ([0542b9c](https://github.com/camunda/camunda-platform-helm/commit/0542b9c16e8d5051ed3c6f7f741f1435ee5906df))
* update camunda/console docker tag to v8.7.40 ([#3728](https://github.com/camunda/camunda-platform-helm/issues/3728)) ([307b24e](https://github.com/camunda/camunda-platform-helm/commit/307b24e64d04ec96bbddf4a8b18a920e589aeb89))


### Refactors

* add local values file per chart version ([#3679](https://github.com/camunda/camunda-platform-helm/issues/3679)) ([cd3064d](https://github.com/camunda/camunda-platform-helm/commit/cd3064d2cf7daa17f6a9c2430687e2a013179be5))
