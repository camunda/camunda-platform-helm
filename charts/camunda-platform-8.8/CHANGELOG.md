# Changelog

## [13.1.0](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.8-13.0.0...camunda-platform-8.8-13.1.0) (2025-10-24)


### Features

* data injection during testing ([#4423](https://github.com/camunda/camunda-platform-helm/issues/4423)) ([3446ddd](https://github.com/camunda/camunda-platform-helm/commit/3446ddd0e8834f19f475f66d650cb780a4b9e80d))


### Bug Fixes

* add back mapping rules in the orchestration cluster ([#4545](https://github.com/camunda/camunda-platform-helm/issues/4545)) ([70c353f](https://github.com/camunda/camunda-platform-helm/commit/70c353f4d0ad2b173f596bdd66640328dc283840))
* allows cpu to be set as either string or number ([#4491](https://github.com/camunda/camunda-platform-helm/issues/4491)) ([a30c924](https://github.com/camunda/camunda-platform-helm/commit/a30c9249329346012427df2d814d61476b51ccc6))
* apply dual-region exclusion logic to opensearch exporter as well ([#4352](https://github.com/camunda/camunda-platform-helm/issues/4352)) ([8b0aff7](https://github.com/camunda/camunda-platform-helm/commit/8b0aff7ad1de25d3187497321707b9b278882fca))
* changing pod anti-affinity rules for migration jobs and importer ([#4481](https://github.com/camunda/camunda-platform-helm/issues/4481)) ([dd89d8d](https://github.com/camunda/camunda-platform-helm/commit/dd89d8d088c88992bf8c4cf7c5b310bf17bdb104))
* consistent naming for migrations and importer ([#4477](https://github.com/camunda/camunda-platform-helm/issues/4477)) ([5009b69](https://github.com/camunda/camunda-platform-helm/commit/5009b69ec29d32311bdadd4e06040c1f3b02cab7))
* delete the migrator from 8.9 ([#4503](https://github.com/camunda/camunda-platform-helm/issues/4503)) ([3ead5cc](https://github.com/camunda/camunda-platform-helm/commit/3ead5cc1a15b3070c611e0ec0fdbb3aed18d86f5))
* the oidc scenario was always running the upgrade. Removing here … ([#4523](https://github.com/camunda/camunda-platform-helm/issues/4523)) ([7849361](https://github.com/camunda/camunda-platform-helm/commit/78493618d57881a32141411a26633989edc8c97d))
* use joinPath helper to fix double slash issues in console configmap ([#4562](https://github.com/camunda/camunda-platform-helm/issues/4562)) ([4e06bd5](https://github.com/camunda/camunda-platform-helm/commit/4e06bd57017a7116c607a97a1fce299dd93f4725))


### Dependencies

* update camunda-platform-8.8 (patch) ([#4485](https://github.com/camunda/camunda-platform-helm/issues/4485)) ([2d7e6c9](https://github.com/camunda/camunda-platform-helm/commit/2d7e6c9969bc1619c9552b32f11d6c3d98213622))
* update camunda-platform-8.8-digest ([#4455](https://github.com/camunda/camunda-platform-helm/issues/4455)) ([d6ddf4c](https://github.com/camunda/camunda-platform-helm/commit/d6ddf4c3d55b6e7508d82946dd13a6a066de34fa))
* update camunda-platform-8.8-digest ([#4464](https://github.com/camunda/camunda-platform-helm/issues/4464)) ([67d1858](https://github.com/camunda/camunda-platform-helm/commit/67d1858519130151a00cb7c139b2bd6cf45f02ab))
* update camunda-platform-8.8-digest ([#4470](https://github.com/camunda/camunda-platform-helm/issues/4470)) ([382ccf4](https://github.com/camunda/camunda-platform-helm/commit/382ccf4cf12888160a38d7bd4d07bb724c8aa191))
* update camunda-platform-8.8-digest ([#4474](https://github.com/camunda/camunda-platform-helm/issues/4474)) ([4cfd615](https://github.com/camunda/camunda-platform-helm/commit/4cfd615326673063ad2cad9b3c875d9a13001b8e))
* update camunda-platform-8.8-digest ([#4480](https://github.com/camunda/camunda-platform-helm/issues/4480)) ([32c9cc0](https://github.com/camunda/camunda-platform-helm/commit/32c9cc0c3d9cf8f37297b3ca6fb8043c8cddb53c))
* update camunda-platform-8.8-digest ([#4483](https://github.com/camunda/camunda-platform-helm/issues/4483)) ([b7c610e](https://github.com/camunda/camunda-platform-helm/commit/b7c610e4c9e6f141cace84154754a511af7d0dec))
* update camunda-platform-8.8-digest ([#4486](https://github.com/camunda/camunda-platform-helm/issues/4486)) ([f4a06d4](https://github.com/camunda/camunda-platform-helm/commit/f4a06d41a6295f8d9dd511b1bdb5d709a9883913))
* update camunda-platform-8.8-digest ([#4493](https://github.com/camunda/camunda-platform-helm/issues/4493)) ([8940f64](https://github.com/camunda/camunda-platform-helm/commit/8940f647939cf279b864f67b27630b1434a5c752))
* update camunda-platform-8.8-digest ([#4496](https://github.com/camunda/camunda-platform-helm/issues/4496)) ([aea36b5](https://github.com/camunda/camunda-platform-helm/commit/aea36b56e884580abe5b4096773bf8a0e19d12da))
* update camunda-platform-8.8-digest ([#4497](https://github.com/camunda/camunda-platform-helm/issues/4497)) ([99b4547](https://github.com/camunda/camunda-platform-helm/commit/99b454794ad1883cdabaa3d7cb43ff572ad5995b))
* update camunda-platform-8.8-digest ([#4498](https://github.com/camunda/camunda-platform-helm/issues/4498)) ([9d8daac](https://github.com/camunda/camunda-platform-helm/commit/9d8daac03b03683fc7a8485f476129215ea0b817))
* update camunda-platform-8.8-digest ([#4504](https://github.com/camunda/camunda-platform-helm/issues/4504)) ([e73dc47](https://github.com/camunda/camunda-platform-helm/commit/e73dc4749bae741422fc8cfed4a8345559b85b85))
* update camunda-platform-8.8-digest ([#4509](https://github.com/camunda/camunda-platform-helm/issues/4509)) ([5de3e92](https://github.com/camunda/camunda-platform-helm/commit/5de3e92f25a80585b7c725b2eaac52d4fbe74156))
* update camunda-platform-8.8-digest ([#4514](https://github.com/camunda/camunda-platform-helm/issues/4514)) ([702a820](https://github.com/camunda/camunda-platform-helm/commit/702a820cdb4a5f54e0ba788bc5c583e939eadf21))
* update camunda-platform-8.8-digest ([#4516](https://github.com/camunda/camunda-platform-helm/issues/4516)) ([836dd1c](https://github.com/camunda/camunda-platform-helm/commit/836dd1ced4fc06b4ab856b80a1cb941dbd3e9696))
* update camunda-platform-8.8-digest ([#4520](https://github.com/camunda/camunda-platform-helm/issues/4520)) ([5fbb026](https://github.com/camunda/camunda-platform-helm/commit/5fbb026ab18930d321a529d722c76270b7e00055))
* update camunda-platform-8.8-digest ([#4521](https://github.com/camunda/camunda-platform-helm/issues/4521)) ([ddfca1a](https://github.com/camunda/camunda-platform-helm/commit/ddfca1a92c8dd699a7ec49a61ce4399bb61f5e52))
* update camunda-platform-8.8-digest ([#4542](https://github.com/camunda/camunda-platform-helm/issues/4542)) ([a257106](https://github.com/camunda/camunda-platform-helm/commit/a257106c1ff76e9393dfe2e7b85a1412ced6172e))
* update camunda-platform-8.8-digest ([#4547](https://github.com/camunda/camunda-platform-helm/issues/4547)) ([4fe3298](https://github.com/camunda/camunda-platform-helm/commit/4fe3298a8830ced28e742ad6389ebf1ef62ef387))
* update camunda-platform-8.8-digest ([#4550](https://github.com/camunda/camunda-platform-helm/issues/4550)) ([4c617b6](https://github.com/camunda/camunda-platform-helm/commit/4c617b6ca18e31c0a68f174247845ce386712e71))
* update camunda-platform-digests ([#4566](https://github.com/camunda/camunda-platform-helm/issues/4566)) ([2a0c9fc](https://github.com/camunda/camunda-platform-helm/commit/2a0c9fc7d1fdf050aac08f9ddd6224c24c018678))
* update camunda/camunda docker tag to v8.8.1 ([#4560](https://github.com/camunda/camunda-platform-helm/issues/4560)) ([adc0783](https://github.com/camunda/camunda-platform-helm/commit/adc0783bb690adc2954b2da82c60824216a2f0c5))
* update camunda/camunda:8.8-snapshot docker digest to ff8d838 ([#4535](https://github.com/camunda/camunda-platform-helm/issues/4535)) ([029dc84](https://github.com/camunda/camunda-platform-helm/commit/029dc84d71e77314fb9bfbfa8f929446a4a02ffd))
* update camunda/console docker tag to v8.8.11 ([#4528](https://github.com/camunda/camunda-platform-helm/issues/4528)) ([d5ed7a6](https://github.com/camunda/camunda-platform-helm/commit/d5ed7a6c2daed86717be6a04e61a4577d12c3db4))
* update camunda/console docker tag to v8.8.12 ([#4531](https://github.com/camunda/camunda-platform-helm/issues/4531)) ([8d4a1a8](https://github.com/camunda/camunda-platform-helm/commit/8d4a1a84c2f75ce659ab58d83118c53217908c66))
* update camunda/console docker tag to v8.8.13 ([#4558](https://github.com/camunda/camunda-platform-helm/issues/4558)) ([e3b709b](https://github.com/camunda/camunda-platform-helm/commit/e3b709ba6337d4b7fd5291c7dc2fb7344ad7b7df))
* update camunda/console docker tag to v8.8.17 ([#4574](https://github.com/camunda/camunda-platform-helm/issues/4574)) ([9eb550a](https://github.com/camunda/camunda-platform-helm/commit/9eb550a74a029d2f439f927a0f1326604ae93098))
* update camunda/console docker tag to v8.8.5 ([#4458](https://github.com/camunda/camunda-platform-helm/issues/4458)) ([fa2cfc0](https://github.com/camunda/camunda-platform-helm/commit/fa2cfc066338b32db9340095dfe1fbf45c8622fe))
* update camunda/console docker tag to v8.8.7 ([#4475](https://github.com/camunda/camunda-platform-helm/issues/4475)) ([7f821ad](https://github.com/camunda/camunda-platform-helm/commit/7f821adc13a017e0a7b3c68199c4231ee9dcd4d4))
* update camunda/console docker tag to v8.8.8 ([#4479](https://github.com/camunda/camunda-platform-helm/issues/4479)) ([c0cc2be](https://github.com/camunda/camunda-platform-helm/commit/c0cc2be06347fb654a8aadf0d7446c43ba7d9c19))
* update camunda/console:latest docker digest to bf89ecd ([#4557](https://github.com/camunda/camunda-platform-helm/issues/4557)) ([a5a4c0d](https://github.com/camunda/camunda-platform-helm/commit/a5a4c0d96473afcb622a0f544fab7569c4ad945b))
* update camunda/web-modeler-restapi docker tag to v8.8.1 ([#4492](https://github.com/camunda/camunda-platform-helm/issues/4492)) ([72571a5](https://github.com/camunda/camunda-platform-helm/commit/72571a57adbed4012e30887f22209b8b790228ce))
* update dependency go to v1.25.3 ([#4456](https://github.com/camunda/camunda-platform-helm/issues/4456)) ([9192ee2](https://github.com/camunda/camunda-platform-helm/commit/9192ee225b22b93fe25342648741f08867637808))
* update registry.camunda.cloud/keycloak-ee/keycloak docker tag to v26.4.1 ([#4540](https://github.com/camunda/camunda-platform-helm/issues/4540)) ([7453ba2](https://github.com/camunda/camunda-platform-helm/commit/7453ba2392978aea2690b420dda1bfe0f1927c18))
