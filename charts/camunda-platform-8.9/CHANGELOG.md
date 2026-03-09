# Changelog

## [14.0.0-alpha5](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.9-14.0.0-alpha4...camunda-platform-8.9-14.0.0-alpha5) (2026-03-09)


### ⚠ BREAKING CHANGES

* remove deprecated secret keys and secret autogeneration for camunda 8.9 ([#5100](https://github.com/camunda/camunda-platform-helm/issues/5100))

### Features

* Add App Integrations exporter config ([#5130](https://github.com/camunda/camunda-platform-helm/issues/5130)) ([b7b02f5](https://github.com/camunda/camunda-platform-helm/commit/b7b02f54b21e2c4efa862eb097aa85062376cc53))
* add tpl support to podLabels, podAnnotations, and global.ingress.host ([#5064](https://github.com/camunda/camunda-platform-helm/issues/5064)) ([b1d64aa](https://github.com/camunda/camunda-platform-helm/commit/b1d64aaa2a9d633e2b266a2abae9b7b07b0db6b0))
* add values.yaml keys for gateway api resources ([#4841](https://github.com/camunda/camunda-platform-helm/issues/4841)) ([d5c614f](https://github.com/camunda/camunda-platform-helm/commit/d5c614fda15bbc8509c206e9bcaf942f13ea955d))
* continuous backups scheduler config ([#5093](https://github.com/camunda/camunda-platform-helm/issues/5093)) ([bb81e68](https://github.com/camunda/camunda-platform-helm/commit/bb81e68802e4bef7a2f0a46c1433beecbe167492))
* copy global.es/os options to optimize and deprecate global.es/os ([#5171](https://github.com/camunda/camunda-platform-helm/issues/5171)) ([6bc146c](https://github.com/camunda/camunda-platform-helm/commit/6bc146c4e50b409b7555184bd4ea1e56c4dcadef))
* do not disable persistent web sessions on RDBMS ([#5098](https://github.com/camunda/camunda-platform-helm/issues/5098)) ([1a239a8](https://github.com/camunda/camunda-platform-helm/commit/1a239a8b451bc927975b69a7790a7d0619c89f93))
* expose options in values.yaml for helm v4 support ([#4918](https://github.com/camunda/camunda-platform-helm/issues/4918)) ([ec0fb7f](https://github.com/camunda/camunda-platform-helm/commit/ec0fb7f62af76b07b5fb970099781ddb4901ef68))
* remove Web Modeler webapp component ([#5193](https://github.com/camunda/camunda-platform-helm/issues/5193)) ([53c6a90](https://github.com/camunda/camunda-platform-helm/commit/53c6a9071bd530701febc344234cfc93763daaa5))
* support admin profile ([#5187](https://github.com/camunda/camunda-platform-helm/issues/5187)) ([04b59be](https://github.com/camunda/camunda-platform-helm/commit/04b59be436fe4f475c8e845b3a2969bc175353e0))
* warn when web modeler pusher secret is auto-generated ([#5168](https://github.com/camunda/camunda-platform-helm/issues/5168)) ([5328915](https://github.com/camunda/camunda-platform-helm/commit/5328915d5388802e1decc006e0a20e2b30d6b40b))


### Bug Fixes

* add initContainer to connectors ([#5271](https://github.com/camunda/camunda-platform-helm/issues/5271)) ([739cb4e](https://github.com/camunda/camunda-platform-helm/commit/739cb4ebeaaf9aedc4d2d2ea5e9e4243a9577426))
* add operate setting back ([#5240](https://github.com/camunda/camunda-platform-helm/issues/5240)) ([cb18cbb](https://github.com/camunda/camunda-platform-helm/commit/cb18cbb50454c9e3476ff35d0160d5917bd991f4))
* add type to headless orchestration cluster service ([#5107](https://github.com/camunda/camunda-platform-helm/issues/5107)) ([5c57502](https://github.com/camunda/camunda-platform-helm/commit/5c5750232ccf29d39bb68b1509d9131603536429))
* application override fix for console ([#5144](https://github.com/camunda/camunda-platform-helm/issues/5144)) ([273535f](https://github.com/camunda/camunda-platform-helm/commit/273535f89c8cd301ad5ac6c709249f26597108d6))
* disable operate and taskist profiles when noSecondaryStorage is enabled ([#5234](https://github.com/camunda/camunda-platform-helm/issues/5234)) ([aa649ca](https://github.com/camunda/camunda-platform-helm/commit/aa649ca7f34aa17584934aa938361c36f7d9497a))
* **documentStore:** allow IRSA AWS usage ([#5026](https://github.com/camunda/camunda-platform-helm/issues/5026)) ([b625076](https://github.com/camunda/camunda-platform-helm/commit/b6250760e1a41f4b477bb1cca408064153482b54))
* don't hardcode binary paths for restore ([#5269](https://github.com/camunda/camunda-platform-helm/issues/5269)) ([6d1743c](https://github.com/camunda/camunda-platform-helm/commit/6d1743c29f6f4a3c043c3bbd6933468e8a0365df))
* drop secondary storage constraint if orchestration.enabled is false ([#5192](https://github.com/camunda/camunda-platform-helm/issues/5192)) ([564a7b9](https://github.com/camunda/camunda-platform-helm/commit/564a7b9ff7cc16220870fe6dc79c293d2f82e474))
* expose public API in headless service ([#5061](https://github.com/camunda/camunda-platform-helm/issues/5061)) ([6bca7e6](https://github.com/camunda/camunda-platform-helm/commit/6bca7e63d9aabd8884fcccf480fbc01d6196400b))
* gate optimize config in identity templates on optimize.enabled ([#5263](https://github.com/camunda/camunda-platform-helm/issues/5263)) ([e0ef936](https://github.com/camunda/camunda-platform-helm/commit/e0ef93612113074ada290c07324e3bd710691b95))
* make OIDC groups-claim optional in orchestration config ([#5207](https://github.com/camunda/camunda-platform-helm/issues/5207)) ([1efc487](https://github.com/camunda/camunda-platform-helm/commit/1efc487d044be9a2beed075ad7a70a21ccc7bd44))
* migrate keycloak auth configuration to standard secret pattern for 8.9 ([#5170](https://github.com/camunda/camunda-platform-helm/issues/5170)) ([e71277d](https://github.com/camunda/camunda-platform-helm/commit/e71277ded83a060253e7087550f1e9a7e556f31d))
* modify extraConfiguration property for all relevant components ([#5134](https://github.com/camunda/camunda-platform-helm/issues/5134)) ([d238ff9](https://github.com/camunda/camunda-platform-helm/commit/d238ff9c3e555a7730343ee99996baad031f1e4b))
* move pusher app key to secret and allow explicit configuration ([#5289](https://github.com/camunda/camunda-platform-helm/issues/5289)) ([64a0ed9](https://github.com/camunda/camunda-platform-helm/commit/64a0ed9c2fda2f3e71d83305b5787e138e86bc02))
* normalize authIssuerBackendUrl to prevent double-slash when contextPath is root ([#5114](https://github.com/camunda/camunda-platform-helm/issues/5114)) ([8525c94](https://github.com/camunda/camunda-platform-helm/commit/8525c942bbaf881200bb58aaca20042f305dea07))
* **openshift:** when es is disabled, fix templating error of label ([#5020](https://github.com/camunda/camunda-platform-helm/issues/5020)) ([50552d7](https://github.com/camunda/camunda-platform-helm/commit/50552d7ed4f97b9706989a9c89e2956aa5d8fac5))
* prevent invalid YAML when zeebe exporter enabled without camunda… ([#4962](https://github.com/camunda/camunda-platform-helm/issues/4962)) ([65db90a](https://github.com/camunda/camunda-platform-helm/commit/65db90a213f1534841314e9ff9a6e40fed9ee1d2))
* remove deprecated secret keys and secret autogeneration for camunda 8.9 ([#5100](https://github.com/camunda/camunda-platform-helm/issues/5100)) ([6f16b3c](https://github.com/camunda/camunda-platform-helm/commit/6f16b3c10d8405e0846a50bc9c94d856d11ac490))
* remove double braces ([#5259](https://github.com/camunda/camunda-platform-helm/issues/5259)) ([2198ce5](https://github.com/camunda/camunda-platform-helm/commit/2198ce55508d8b265e0975ab01ab21238c613742))
* remove helm workaround for optimize extraConfig ([#5245](https://github.com/camunda/camunda-platform-helm/issues/5245)) ([ba5cfbd](https://github.com/camunda/camunda-platform-helm/commit/ba5cfbd08fe3bf42fe92ae8fd9951356cb9507bf))
* support all restore modes ([#5248](https://github.com/camunda/camunda-platform-helm/issues/5248)) ([e91abf3](https://github.com/camunda/camunda-platform-helm/commit/e91abf38760470ae53440b1780e925eeca135140))
* zeebe.broker.exporters nil rendered ([#5243](https://github.com/camunda/camunda-platform-helm/issues/5243)) ([ac9e4a7](https://github.com/camunda/camunda-platform-helm/commit/ac9e4a7388664be45d4046e5b91183fc0c92ecdd))


### Documentation

* document authentication modes in values.yaml ([#5113](https://github.com/camunda/camunda-platform-helm/issues/5113)) ([096bfda](https://github.com/camunda/camunda-platform-helm/commit/096bfdab27e05aee952ac93f691e101ce7959bac))


### Dependencies

* update camunda-platform-digests ([#5094](https://github.com/camunda/camunda-platform-helm/issues/5094)) ([4f6dc63](https://github.com/camunda/camunda-platform-helm/commit/4f6dc634c9465737d6f286c9f3182174fc4868cf))
* update camunda-platform-digests ([#5106](https://github.com/camunda/camunda-platform-helm/issues/5106)) ([d1cadd3](https://github.com/camunda/camunda-platform-helm/commit/d1cadd3cbb695287c2ffec7c1f43811681cef29d))
* update camunda-platform-digests ([#5110](https://github.com/camunda/camunda-platform-helm/issues/5110)) ([edae209](https://github.com/camunda/camunda-platform-helm/commit/edae2098f5a9e4f3db231db1ca2cb3e15ee3f0e6))
* update camunda-platform-digests ([#5137](https://github.com/camunda/camunda-platform-helm/issues/5137)) ([4b4678f](https://github.com/camunda/camunda-platform-helm/commit/4b4678fc23eeeffc07f94953234da4709a3e9bd1))
* update camunda-platform-digests ([#5145](https://github.com/camunda/camunda-platform-helm/issues/5145)) ([3ea171a](https://github.com/camunda/camunda-platform-helm/commit/3ea171ac2d586a4527ce03649e70cf303172caed))
* update camunda-platform-digests ([#5151](https://github.com/camunda/camunda-platform-helm/issues/5151)) ([e7d2e7e](https://github.com/camunda/camunda-platform-helm/commit/e7d2e7e50347d26eeda69d67d288a275fbf0837f))
* update camunda-platform-digests ([#5181](https://github.com/camunda/camunda-platform-helm/issues/5181)) ([1c9625f](https://github.com/camunda/camunda-platform-helm/commit/1c9625f68e57071aa16b9c7d6beab184bb9686be))
* update camunda-platform-digests ([#5209](https://github.com/camunda/camunda-platform-helm/issues/5209)) ([8ea4102](https://github.com/camunda/camunda-platform-helm/commit/8ea4102959794dbbd8b6f369555d9908b9d3e4c9))
* update camunda-platform-digests ([#5211](https://github.com/camunda/camunda-platform-helm/issues/5211)) ([80f14eb](https://github.com/camunda/camunda-platform-helm/commit/80f14eb1c14e17303fac63d44f5b3b03380ddb60))
* update camunda-platform-digests ([#5212](https://github.com/camunda/camunda-platform-helm/issues/5212)) ([5504144](https://github.com/camunda/camunda-platform-helm/commit/5504144ca3c4904c8e9bfae47915957a8a72fae5))
* update camunda-platform-digests ([#5215](https://github.com/camunda/camunda-platform-helm/issues/5215)) ([aef1596](https://github.com/camunda/camunda-platform-helm/commit/aef1596508cdd4adc9f3ac0c294b4b34b55e28f1))
* update camunda-platform-digests ([#5216](https://github.com/camunda/camunda-platform-helm/issues/5216)) ([226200d](https://github.com/camunda/camunda-platform-helm/commit/226200d948c57018c2aaf633a1e2384bad7af3ae))
* update camunda-platform-digests ([#5218](https://github.com/camunda/camunda-platform-helm/issues/5218)) ([4909c0f](https://github.com/camunda/camunda-platform-helm/commit/4909c0f9d283a7a84635d034cfd56ca367a286e7))
* update camunda-platform-digests ([#5219](https://github.com/camunda/camunda-platform-helm/issues/5219)) ([77636e5](https://github.com/camunda/camunda-platform-helm/commit/77636e59941bf2a6912178646ec25e6769102289))
* update camunda-platform-digests ([#5220](https://github.com/camunda/camunda-platform-helm/issues/5220)) ([529823f](https://github.com/camunda/camunda-platform-helm/commit/529823f2a6bff55d4bde82effd494d5d3f5cb68d))
* update camunda-platform-digests ([#5221](https://github.com/camunda/camunda-platform-helm/issues/5221)) ([b0800db](https://github.com/camunda/camunda-platform-helm/commit/b0800dbf9038bf83a9d9ecad4f507bd878370f33))
* update camunda-platform-digests ([#5222](https://github.com/camunda/camunda-platform-helm/issues/5222)) ([c74cb20](https://github.com/camunda/camunda-platform-helm/commit/c74cb20b8966c559b288a255d19da8156e731be2))
* update camunda-platform-digests ([#5223](https://github.com/camunda/camunda-platform-helm/issues/5223)) ([51e71e8](https://github.com/camunda/camunda-platform-helm/commit/51e71e8880937ba414d0b2dac019a6ad3e377581))
* update camunda-platform-digests ([#5224](https://github.com/camunda/camunda-platform-helm/issues/5224)) ([b48ce94](https://github.com/camunda/camunda-platform-helm/commit/b48ce945aff8ec16f68b243992bb3a0e6507fc60))
* update camunda-platform-digests ([#5226](https://github.com/camunda/camunda-platform-helm/issues/5226)) ([1a788f6](https://github.com/camunda/camunda-platform-helm/commit/1a788f644ad8422c85c0637615078a19e149042d))
* update camunda-platform-digests ([#5228](https://github.com/camunda/camunda-platform-helm/issues/5228)) ([6965364](https://github.com/camunda/camunda-platform-helm/commit/6965364fc90f3433d154d2461972b025e31476da))
* update camunda-platform-digests ([#5233](https://github.com/camunda/camunda-platform-helm/issues/5233)) ([dfb1e0f](https://github.com/camunda/camunda-platform-helm/commit/dfb1e0fcd02dc5b815419ed9d4c3ceee03327cff))
* update camunda-platform-digests ([#5238](https://github.com/camunda/camunda-platform-helm/issues/5238)) ([7a345e1](https://github.com/camunda/camunda-platform-helm/commit/7a345e19355deebbad2de63885f4088a9715f76b))
* update camunda-platform-digests ([#5246](https://github.com/camunda/camunda-platform-helm/issues/5246)) ([e4ed146](https://github.com/camunda/camunda-platform-helm/commit/e4ed146f26e8bf6d6a8356e203d29af5eb70fbf2))
* update camunda-platform-digests ([#5249](https://github.com/camunda/camunda-platform-helm/issues/5249)) ([1e97ff1](https://github.com/camunda/camunda-platform-helm/commit/1e97ff1f66cc6f476860f2adf65fa6d210272f05))
* update camunda-platform-digests ([#5261](https://github.com/camunda/camunda-platform-helm/issues/5261)) ([7970d3d](https://github.com/camunda/camunda-platform-helm/commit/7970d3d847cf9703716658da692ed855358b2da4))
* update camunda-platform-digests ([#5264](https://github.com/camunda/camunda-platform-helm/issues/5264)) ([b8542e9](https://github.com/camunda/camunda-platform-helm/commit/b8542e9819712ab450e538f75bc445998c73a401))
* update camunda-platform-digests ([#5266](https://github.com/camunda/camunda-platform-helm/issues/5266)) ([8610059](https://github.com/camunda/camunda-platform-helm/commit/8610059efb50b699779f14aaf48536e2603360f1))
* update camunda-platform-digests ([#5276](https://github.com/camunda/camunda-platform-helm/issues/5276)) ([a674370](https://github.com/camunda/camunda-platform-helm/commit/a67437016939fc9d89ab26db8fe5d3a0a8300f77))
* update camunda-platform-digests ([#5282](https://github.com/camunda/camunda-platform-helm/issues/5282)) ([a4a47ae](https://github.com/camunda/camunda-platform-helm/commit/a4a47ae8123d6118bab7a2507f4998b3e0cb0e36))
* update camunda-platform-digests ([#5283](https://github.com/camunda/camunda-platform-helm/issues/5283)) ([bb289fe](https://github.com/camunda/camunda-platform-helm/commit/bb289feea80c20b15b9f3c181df3dee2d4836542))
* update camunda-platform-digests ([#5288](https://github.com/camunda/camunda-platform-helm/issues/5288)) ([2a44d52](https://github.com/camunda/camunda-platform-helm/commit/2a44d52ad165e0c023b18b518cacfe747ea570ef))
* update camunda-platform-digests ([#5296](https://github.com/camunda/camunda-platform-helm/issues/5296)) ([1299383](https://github.com/camunda/camunda-platform-helm/commit/1299383565ed9c1011ee1d53dc8ef20212a59f85))
* update camunda-platform-digests ([#5313](https://github.com/camunda/camunda-platform-helm/issues/5313)) ([febc7e4](https://github.com/camunda/camunda-platform-helm/commit/febc7e4387f12e28793524ee275004e93114dab2))
* update camunda-platform-digests ([#5318](https://github.com/camunda/camunda-platform-helm/issues/5318)) ([6ef2c5a](https://github.com/camunda/camunda-platform-helm/commit/6ef2c5a8908c197039221cf74aee4f7823b0ca5e))
* update camunda-platform-digests ([#5320](https://github.com/camunda/camunda-platform-helm/issues/5320)) ([9ae7535](https://github.com/camunda/camunda-platform-helm/commit/9ae753596fe49bd5be2fd6381d15cbe2d48d12fa))
* update camunda-platform-digests ([#5325](https://github.com/camunda/camunda-platform-helm/issues/5325)) ([89b9d73](https://github.com/camunda/camunda-platform-helm/commit/89b9d7327529dbf460dce9d60b7efb6d58a79993))
* update camunda-platform-digests ([#5326](https://github.com/camunda/camunda-platform-helm/issues/5326)) ([bb78074](https://github.com/camunda/camunda-platform-helm/commit/bb780748f93576b3a08d2ed80c1ae098ffae981e))
* update camunda-platform-digests ([#5327](https://github.com/camunda/camunda-platform-helm/issues/5327)) ([7e9cfca](https://github.com/camunda/camunda-platform-helm/commit/7e9cfca01ca41846606b5a9fe6fa00415b8301c4))
* update camunda-platform-digests ([#5329](https://github.com/camunda/camunda-platform-helm/issues/5329)) ([b3540e8](https://github.com/camunda/camunda-platform-helm/commit/b3540e8367a1c86252edf31b2fb73643a25fee95))
* update camunda-platform-digests ([#5331](https://github.com/camunda/camunda-platform-helm/issues/5331)) ([ae760dd](https://github.com/camunda/camunda-platform-helm/commit/ae760ddab4b661fe88b5f9e99bc84dd963945b65))
* update camunda-platform-digests ([#5333](https://github.com/camunda/camunda-platform-helm/issues/5333)) ([3f821e4](https://github.com/camunda/camunda-platform-helm/commit/3f821e4d7a243140abcdd736f26fb1218a4baf0d))
* update camunda-platform-images (patch) ([#5250](https://github.com/camunda/camunda-platform-helm/issues/5250)) ([d4c3c12](https://github.com/camunda/camunda-platform-helm/commit/d4c3c12a55123638377b94aa2f9b30966dfde4a5))
* update camunda-platform-images (patch) ([#5255](https://github.com/camunda/camunda-platform-helm/issues/5255)) ([4e0e5b7](https://github.com/camunda/camunda-platform-helm/commit/4e0e5b7b9ee99c2d8254693284bb6bc2475eb4dd))
* update camunda-platform-images (patch) ([#5314](https://github.com/camunda/camunda-platform-helm/issues/5314)) ([cd83a8b](https://github.com/camunda/camunda-platform-helm/commit/cd83a8b3f20af64a9dce7b58b6bc94daa5c6ae47))
* update camunda/camunda:snapshot docker digest to 1f4ee72 ([#5236](https://github.com/camunda/camunda-platform-helm/issues/5236)) ([4c0e037](https://github.com/camunda/camunda-platform-helm/commit/4c0e0374ef2674de1529ae48d284c43ec5d76bd1))
* update camunda/camunda:snapshot docker digest to 29878c3 ([#5210](https://github.com/camunda/camunda-platform-helm/issues/5210)) ([5e2ce3d](https://github.com/camunda/camunda-platform-helm/commit/5e2ce3de65076abe3b9087dbebab642604038fc9))
* update camunda/camunda:snapshot docker digest to 3038fd5 ([#5244](https://github.com/camunda/camunda-platform-helm/issues/5244)) ([301335e](https://github.com/camunda/camunda-platform-helm/commit/301335e265bd117d5e057c153f153e0e708d9471))
* update camunda/camunda:snapshot docker digest to 5af942a ([#5281](https://github.com/camunda/camunda-platform-helm/issues/5281)) ([1c7137b](https://github.com/camunda/camunda-platform-helm/commit/1c7137b2e58d1379d7b6336b0555e1ab6e8c1ec6))
* update camunda/camunda:snapshot docker digest to 69ba040 ([#5285](https://github.com/camunda/camunda-platform-helm/issues/5285)) ([b67ea8c](https://github.com/camunda/camunda-platform-helm/commit/b67ea8c609d835a37aca98f5e468177815212281))
* update camunda/camunda:snapshot docker digest to 6add826 ([#5319](https://github.com/camunda/camunda-platform-helm/issues/5319)) ([ce55ff4](https://github.com/camunda/camunda-platform-helm/commit/ce55ff4a3427c567149315574236122a8128df32))
* update camunda/camunda:snapshot docker digest to 9ab6789 ([#5206](https://github.com/camunda/camunda-platform-helm/issues/5206)) ([ebd4396](https://github.com/camunda/camunda-platform-helm/commit/ebd4396c9c43967c67f4f2408b22e8583420d47e))
* update camunda/camunda:snapshot docker digest to acdce52 ([#5262](https://github.com/camunda/camunda-platform-helm/issues/5262)) ([66a91d7](https://github.com/camunda/camunda-platform-helm/commit/66a91d7b89185bf04d82bb9925c78bd523ad91c5))
* update camunda/camunda:snapshot docker digest to c0e207b ([#5268](https://github.com/camunda/camunda-platform-helm/issues/5268)) ([e1486aa](https://github.com/camunda/camunda-platform-helm/commit/e1486aa4eb9e57c756519897332456027bfd9a59))
* update camunda/connectors-bundle:snapshot docker digest to cfbacd1 ([#5299](https://github.com/camunda/camunda-platform-helm/issues/5299)) ([4891658](https://github.com/camunda/camunda-platform-helm/commit/489165890c8387f88c9f9271b5dfed4a4b66ec42))
* update camunda/optimize:8-latest docker digest to 208f345 ([#5290](https://github.com/camunda/camunda-platform-helm/issues/5290)) ([df043d0](https://github.com/camunda/camunda-platform-helm/commit/df043d00caed87681efc7442e705a0042baccf67))
* update camunda/web-modeler-restapi:snapshot docker digest to 31a0acc ([#5332](https://github.com/camunda/camunda-platform-helm/issues/5332)) ([694a2c0](https://github.com/camunda/camunda-platform-helm/commit/694a2c08e19f4bdb583753d4c068d1f3c6a3740f))
* update camunda/web-modeler-restapi:snapshot docker digest to 517ee1b ([#5292](https://github.com/camunda/camunda-platform-helm/issues/5292)) ([626093a](https://github.com/camunda/camunda-platform-helm/commit/626093ae5869f9cf05b34435874a58b9099a4dec))
* update camunda/web-modeler-restapi:snapshot docker digest to 7074542 ([#5330](https://github.com/camunda/camunda-platform-helm/issues/5330)) ([ba40baf](https://github.com/camunda/camunda-platform-helm/commit/ba40bafb225d3c7cd152a39c8e182e90c566ff0c))
* update minor-updates (minor) ([#5031](https://github.com/camunda/camunda-platform-helm/issues/5031)) ([8febe72](https://github.com/camunda/camunda-platform-helm/commit/8febe72311516c68444470bd08c9c59fff1db08f))
* update minor-updates (minor) ([#5190](https://github.com/camunda/camunda-platform-helm/issues/5190)) ([23f46cc](https://github.com/camunda/camunda-platform-helm/commit/23f46cce8eb7a2c6d43b7b4dd1d90871183b8a59))
* update module filippo.io/edwards25519 to v1.1.1 [security] ([#5166](https://github.com/camunda/camunda-platform-helm/issues/5166)) ([09f8c4e](https://github.com/camunda/camunda-platform-helm/commit/09f8c4e42beae75abe4ecd00218eb210c0a7498b))
* update patch-updates (patch) ([#5033](https://github.com/camunda/camunda-platform-helm/issues/5033)) ([246572c](https://github.com/camunda/camunda-platform-helm/commit/246572c06b3508731446b0402aabb8d12b29f512))
* update patch-updates (patch) ([#5183](https://github.com/camunda/camunda-platform-helm/issues/5183)) ([eef71ff](https://github.com/camunda/camunda-platform-helm/commit/eef71ffec59813cb48930eff516249043d603b79))


### Refactors

* clean up redundant helpers in chart 8.9 ([#5124](https://github.com/camunda/camunda-platform-helm/issues/5124)) ([0637b5a](https://github.com/camunda/camunda-platform-helm/commit/0637b5a23286b3b2b3ab83841667ccbd7ce040d5))
* deprecate webModeler.restapi.externalDatabase.user for .username ([#5132](https://github.com/camunda/camunda-platform-helm/issues/5132)) ([e3e0dcf](https://github.com/camunda/camunda-platform-helm/commit/e3e0dcfe39329b10b4919acc64e44234b4b1a879))
* remove secondary storage ([#5141](https://github.com/camunda/camunda-platform-helm/issues/5141)) ([7ad60f3](https://github.com/camunda/camunda-platform-helm/commit/7ad60f3c6318b29a8642cef3e3b870418414ed94))

## [14.0.0-alpha4](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.9-14.0.0-alpha3...camunda-platform-8.9-14.0.0-alpha4) (2026-02-06)


### Features

* add property for authorization initialization ([#4884](https://github.com/camunda/camunda-platform-helm/issues/4884)) ([e764d2f](https://github.com/camunda/camunda-platform-helm/commit/e764d2f46d136e434498249c8940b719474d6097))
* support new restore arguments ([#5076](https://github.com/camunda/camunda-platform-helm/issues/5076)) ([72f3648](https://github.com/camunda/camunda-platform-helm/commit/72f36488776c6c1977b6960cd5afe23bcd5d8932))


### Bug Fixes

* support lower helm cli versions by conditionally using toYamlPretty ([#4978](https://github.com/camunda/camunda-platform-helm/issues/4978)) ([c0e1340](https://github.com/camunda/camunda-platform-helm/commit/c0e13407cd1ed6e1d4a3c2980cd3e43f6e1a473b))
* unify default REST port to 8080 in helm chart 8.9 ([#5065](https://github.com/camunda/camunda-platform-helm/issues/5065)) ([f941f1d](https://github.com/camunda/camunda-platform-helm/commit/f941f1dc9906f04451357a5e00f5ea155337cbe9))


### Documentation

* fix broken optimize documentation link in chart 8.9 ([#5067](https://github.com/camunda/camunda-platform-helm/issues/5067)) ([8230c74](https://github.com/camunda/camunda-platform-helm/commit/8230c74c3cdfda125539daa1920ccab5dfac0694))


### Dependencies

* enterprise elasticsearch version changed to 8.19.9 ([#5052](https://github.com/camunda/camunda-platform-helm/issues/5052)) ([2011d67](https://github.com/camunda/camunda-platform-helm/commit/2011d678d3cfd90a80635265b2fc3ad2ef181f93))
* update camunda-platform-digests ([#5012](https://github.com/camunda/camunda-platform-helm/issues/5012)) ([2bcf375](https://github.com/camunda/camunda-platform-helm/commit/2bcf375cb3005d6d881448e1a14c33299984947e))
* update camunda-platform-digests ([#5028](https://github.com/camunda/camunda-platform-helm/issues/5028)) ([feaccdf](https://github.com/camunda/camunda-platform-helm/commit/feaccdf29e8f61dd4701dbc4e15e0e81edb160dd))
* update camunda-platform-digests ([#5034](https://github.com/camunda/camunda-platform-helm/issues/5034)) ([5be2f99](https://github.com/camunda/camunda-platform-helm/commit/5be2f9990328436752a2ae7ccd81385a935f0f22))
* update camunda-platform-digests ([#5055](https://github.com/camunda/camunda-platform-helm/issues/5055)) ([96301d9](https://github.com/camunda/camunda-platform-helm/commit/96301d9e0b19506e8543af902cebe78bb6f6ac2a))
* update camunda-platform-digests ([#5068](https://github.com/camunda/camunda-platform-helm/issues/5068)) ([32b6f45](https://github.com/camunda/camunda-platform-helm/commit/32b6f453a9fd01b4c5033ce53bb72a3e0e0cbf53))
* update camunda-platform-digests ([#5071](https://github.com/camunda/camunda-platform-helm/issues/5071)) ([5a64ccb](https://github.com/camunda/camunda-platform-helm/commit/5a64ccb2059f8d77ea8b14d37a3c40ab0c7dd6fe))
* update camunda-platform-images (patch) ([#5062](https://github.com/camunda/camunda-platform-helm/issues/5062)) ([3c81c8e](https://github.com/camunda/camunda-platform-helm/commit/3c81c8ee602ce924d0446e12ab03efe6440738f7))
* update camunda-platform-images (patch) ([#5081](https://github.com/camunda/camunda-platform-helm/issues/5081)) ([b95a4c6](https://github.com/camunda/camunda-platform-helm/commit/b95a4c660abce02da7f7cf14457a955bfef826fc))

## [14.0.0-alpha3](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.9-14.0.0-alpha2...camunda-platform-8.9-14.0.0-alpha3) (2026-01-09)


### ⚠ BREAKING CHANGES

* no default secondaryStorage ([#4820](https://github.com/camunda/camunda-platform-helm/issues/4820))

### Features

* **connectors:** use issuer uri preferably if available ([#4904](https://github.com/camunda/camunda-platform-helm/issues/4904)) ([b26ea13](https://github.com/camunda/camunda-platform-helm/commit/b26ea13407f2db1317cc5a49badbd4a15a70fd3d))


### Bug Fixes

* add rdbms type option to values.yaml ([#4883](https://github.com/camunda/camunda-platform-helm/issues/4883)) ([cdf141e](https://github.com/camunda/camunda-platform-helm/commit/cdf141e86e90d23d4b1c6999c1ff82249901e92f))
* apply tpl to issuerBackendUrl ([#4858](https://github.com/camunda/camunda-platform-helm/issues/4858)) ([22b5cd7](https://github.com/camunda/camunda-platform-helm/commit/22b5cd74e7a3e952b17f752541c8233c5cd0f185))
* asdf installation cache passes in an env var that doesn't get rendered ([#4947](https://github.com/camunda/camunda-platform-helm/issues/4947)) ([efc1164](https://github.com/camunda/camunda-platform-helm/commit/efc1164c16631cf1b61d670e51ff92fdb59074df))
* do not redundantly set zeebe exporter options when set in secondary-storage ([#4950](https://github.com/camunda/camunda-platform-helm/issues/4950)) ([6d68fbd](https://github.com/camunda/camunda-platform-helm/commit/6d68fbd9f5fe60dafe7683f3a7972baded350f31))
* no default secondaryStorage ([#4820](https://github.com/camunda/camunda-platform-helm/issues/4820)) ([759c072](https://github.com/camunda/camunda-platform-helm/commit/759c0726068b8b4ae625e2cb35aa2de532c420c9))
* remove constraint of identity.enabled=false and optimize.enabled=true ([#4910](https://github.com/camunda/camunda-platform-helm/issues/4910)) ([0753d6f](https://github.com/camunda/camunda-platform-helm/commit/0753d6fc58336e9538856c9dd88aca52777a40ac))


### Documentation

* update readme dependency section ([#4960](https://github.com/camunda/camunda-platform-helm/issues/4960)) ([3ddfb86](https://github.com/camunda/camunda-platform-helm/commit/3ddfb860ff8c4355a3ef2c0f2a5f71195f929e40))


### Dependencies

* update camunda-platform-digests ([#4846](https://github.com/camunda/camunda-platform-helm/issues/4846)) ([e89a081](https://github.com/camunda/camunda-platform-helm/commit/e89a081f6c53c7b8676917c88c1761d1c07ddc5c))
* update camunda-platform-digests ([#4856](https://github.com/camunda/camunda-platform-helm/issues/4856)) ([1994d36](https://github.com/camunda/camunda-platform-helm/commit/1994d369ec157bf0b474c8e83a59a71ddf8e7ba8))
* update camunda-platform-digests ([#4879](https://github.com/camunda/camunda-platform-helm/issues/4879)) ([f46b3c6](https://github.com/camunda/camunda-platform-helm/commit/f46b3c6ebef8daaae05c91efea3cdc4b4ece4e77))
* update camunda-platform-digests ([#4886](https://github.com/camunda/camunda-platform-helm/issues/4886)) ([101d740](https://github.com/camunda/camunda-platform-helm/commit/101d74019f0766079201fdcd662907a9e0553e44))
* update camunda-platform-digests ([#4891](https://github.com/camunda/camunda-platform-helm/issues/4891)) ([b51a8f0](https://github.com/camunda/camunda-platform-helm/commit/b51a8f0c8661193b738eb3e99bfeb679c3a92741))
* update camunda-platform-digests ([#4939](https://github.com/camunda/camunda-platform-helm/issues/4939)) ([2a45096](https://github.com/camunda/camunda-platform-helm/commit/2a450962ba4b0cc7ab374a13ac13f65ff64650a6))
* update camunda-platform-digests ([#4941](https://github.com/camunda/camunda-platform-helm/issues/4941)) ([e0be86d](https://github.com/camunda/camunda-platform-helm/commit/e0be86d30985e801c04072b66f45db1fe79af21b))
* update camunda-platform-digests ([#4951](https://github.com/camunda/camunda-platform-helm/issues/4951)) ([b5e2e95](https://github.com/camunda/camunda-platform-helm/commit/b5e2e95f0bbcf1c90d4e0936fb3352eb3aea95af))
* update camunda-platform-digests ([#4963](https://github.com/camunda/camunda-platform-helm/issues/4963)) ([e514afd](https://github.com/camunda/camunda-platform-helm/commit/e514afd03d84da86a6dc2a1a2a00fb80d5d235ed))
* update camunda-platform-digests ([#4970](https://github.com/camunda/camunda-platform-helm/issues/4970)) ([6aecda9](https://github.com/camunda/camunda-platform-helm/commit/6aecda9f542d013e01fc962cdfb85e62c8b4b425))
* update camunda-platform-digests ([#4973](https://github.com/camunda/camunda-platform-helm/issues/4973)) ([2561250](https://github.com/camunda/camunda-platform-helm/commit/2561250a80fd13cc74689b4e32b2c6e83f3c2272))
* update camunda-platform-digests ([#4977](https://github.com/camunda/camunda-platform-helm/issues/4977)) ([886cec2](https://github.com/camunda/camunda-platform-helm/commit/886cec2f2f606151c394cf2294fb523b11d79836))
* update camunda-platform-digests ([#4981](https://github.com/camunda/camunda-platform-helm/issues/4981)) ([95e0899](https://github.com/camunda/camunda-platform-helm/commit/95e0899b3e10a84088fd49929e7aa62888f896f2))
* update camunda-platform-images (patch) ([#4848](https://github.com/camunda/camunda-platform-helm/issues/4848)) ([bcc02e8](https://github.com/camunda/camunda-platform-helm/commit/bcc02e832939bfcb6fa643befa11ef0701a883f7))
* update camunda-platform-images (patch) ([#4923](https://github.com/camunda/camunda-platform-helm/issues/4923)) ([94829aa](https://github.com/camunda/camunda-platform-helm/commit/94829aaba5c970f84d0c6ccd01cec67a37d463e9))
* update camunda-platform-images to v8.9.0-alpha3 (patch) ([#4987](https://github.com/camunda/camunda-platform-helm/issues/4987)) ([232128d](https://github.com/camunda/camunda-platform-helm/commit/232128d87089a2676cb802f69ebe16e3c5f8c750))
* update minor-updates (minor) ([#4929](https://github.com/camunda/camunda-platform-helm/issues/4929)) ([6a63cdc](https://github.com/camunda/camunda-platform-helm/commit/6a63cdc23cdc6d17b7cec3aa8ea55c40eae7d372))
* update patch-updates (patch) ([#4860](https://github.com/camunda/camunda-platform-helm/issues/4860)) ([b059be6](https://github.com/camunda/camunda-platform-helm/commit/b059be61080ee33c8d8ee9cfa5f0f4d2f4cdaf35))
* update patch-updates (patch) ([#4924](https://github.com/camunda/camunda-platform-helm/issues/4924)) ([8814e76](https://github.com/camunda/camunda-platform-helm/commit/8814e76c6fa71cc4db57051db959b4cec20ef9a1))


### Refactors

* remove unused identity redirect-url ([#4853](https://github.com/camunda/camunda-platform-helm/issues/4853)) ([90c61e6](https://github.com/camunda/camunda-platform-helm/commit/90c61e66d4676b4ccadee71e6a593ab69df7f6d9))

## [14.0.0-alpha2](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.9-14.0.0-alpha1...camunda-platform-8.9-14.0.0-alpha2) (2025-12-05)


### Features

* add RDBMS support to 8.9 helm charts ([#4572](https://github.com/camunda/camunda-platform-helm/issues/4572)) ([342569e](https://github.com/camunda/camunda-platform-helm/commit/342569e0e2c0c94c555c3886c1b4a2b225542662))
* backport custom client creation to 8.6 and 8.9 ([#4710](https://github.com/camunda/camunda-platform-helm/issues/4710)) ([68bec54](https://github.com/camunda/camunda-platform-helm/commit/68bec54d8f2e7147c2f75ff20c3314533ce0c3a7))
* define custom users through values.yaml ([#4670](https://github.com/camunda/camunda-platform-helm/issues/4670)) ([19ab9eb](https://github.com/camunda/camunda-platform-helm/commit/19ab9eb7e42fe84b76118a1930dd72bb6d302cdf))
* enhance Keycloak integration with realm support and additional client configurations ([911ad7a](https://github.com/camunda/camunda-platform-helm/commit/911ad7a93f41a5b5be6ffffc6e182d55ab867f8c))


### Bug Fixes

* 8.9 version bumps for alpha2 ([#4843](https://github.com/camunda/camunda-platform-helm/issues/4843)) ([4330e7f](https://github.com/camunda/camunda-platform-helm/commit/4330e7fadd3d4ec95054fbf1cf13a32412789e6f))
* add requestBodySize to orchestration multipart config ([#4838](https://github.com/camunda/camunda-platform-helm/issues/4838)) ([8acc157](https://github.com/camunda/camunda-platform-helm/commit/8acc157bacd52c64ff1c480f56d88fd01042b1a1))
* align 8.9 retention config with Wave 1 property names ([#4813](https://github.com/camunda/camunda-platform-helm/issues/4813)) ([65cf7ab](https://github.com/camunda/camunda-platform-helm/commit/65cf7ab3a68f6ede6e163845a4ce3b3051136f7e))
* enable auto-ddl by default ([#4821](https://github.com/camunda/camunda-platform-helm/issues/4821)) ([3d1767c](https://github.com/camunda/camunda-platform-helm/commit/3d1767cffd683db2f9d0ca937daabe3badea7982))
* extraVolumeClaimTemplateTemplate indent for orchestration cluster ([#4697](https://github.com/camunda/camunda-platform-helm/issues/4697)) ([4c5387f](https://github.com/camunda/camunda-platform-helm/commit/4c5387f03688ab9c510e45dc92f97f7c0da9fac7))
* incorrect example for keycloak in readme.md ([#4586](https://github.com/camunda/camunda-platform-helm/issues/4586)) ([f6bf0a9](https://github.com/camunda/camunda-platform-helm/commit/f6bf0a9c125178b2cd3b15d465dc7ed0a59893b8))
* let helm chart support hybrid auth ([#4785](https://github.com/camunda/camunda-platform-helm/issues/4785)) ([cb06ece](https://github.com/camunda/camunda-platform-helm/commit/cb06ece477535c069b03ab5eff3729d9baf93d0a))
* modeler webapp to websockets connection not using override option ([#4812](https://github.com/camunda/camunda-platform-helm/issues/4812)) ([339da02](https://github.com/camunda/camunda-platform-helm/commit/339da02a87add81852177530d2d3b0d5937dd73e))
* refactor tls secrets to use new pattern ([#4599](https://github.com/camunda/camunda-platform-helm/issues/4599)) ([ec98d12](https://github.com/camunda/camunda-platform-helm/commit/ec98d12167c05b959d29d5805630b931efe64a13))
* remap replicas key from legacy to new keys ([#4554](https://github.com/camunda/camunda-platform-helm/issues/4554)) ([7019e4d](https://github.com/camunda/camunda-platform-helm/commit/7019e4d09784b04357465b8ef39f67050c92b6da))
* remove client env vars from qa scenario files ([#4726](https://github.com/camunda/camunda-platform-helm/issues/4726)) ([2c9ea12](https://github.com/camunda/camunda-platform-helm/commit/2c9ea121df9f402b19330e61dddbdd28ffbd4d35))
* remove conditional rendering from management identity configmap ([#4771](https://github.com/camunda/camunda-platform-helm/issues/4771)) ([0dff2df](https://github.com/camunda/camunda-platform-helm/commit/0dff2df28c565b7d75722cd87c18a1dd82433a01))
* remove leftover console secret constraints ([#4749](https://github.com/camunda/camunda-platform-helm/issues/4749)) ([80bd4de](https://github.com/camunda/camunda-platform-helm/commit/80bd4de8835b1216ed4ce52cc55b959f18d09d9a))
* return none for authMethod when component is disabled ([#4810](https://github.com/camunda/camunda-platform-helm/issues/4810)) ([e69c0b7](https://github.com/camunda/camunda-platform-helm/commit/e69c0b7618dcee5d7bd1f6dd844614baca709a7e))
* typo lower case values ([#4737](https://github.com/camunda/camunda-platform-helm/issues/4737)) ([2ec2710](https://github.com/camunda/camunda-platform-helm/commit/2ec2710830d669e53a709bbb176c58ba064e12f2))
* **web-modeler:** align pusher secret usage across components ([#4769](https://github.com/camunda/camunda-platform-helm/issues/4769)) ([bf225b1](https://github.com/camunda/camunda-platform-helm/commit/bf225b13a2c54aa64841e7f421131260a1ef2098))


### Dependencies

* update camunda-platform-digests ([#4704](https://github.com/camunda/camunda-platform-helm/issues/4704)) ([9c31cdc](https://github.com/camunda/camunda-platform-helm/commit/9c31cdc697a4cf9e60fda4c392b8213e9101537d))
* update camunda-platform-digests ([#4720](https://github.com/camunda/camunda-platform-helm/issues/4720)) ([8d69681](https://github.com/camunda/camunda-platform-helm/commit/8d696810a230633f09ff0b4a2921b7e4c954f832))
* update camunda-platform-digests ([#4724](https://github.com/camunda/camunda-platform-helm/issues/4724)) ([390de99](https://github.com/camunda/camunda-platform-helm/commit/390de99e51b6169aeb9ba6c44f9a84fb0f8e0d1a))
* update camunda-platform-digests ([#4743](https://github.com/camunda/camunda-platform-helm/issues/4743)) ([4a2c32a](https://github.com/camunda/camunda-platform-helm/commit/4a2c32a97b1b614a0b6f09a1d1adf78055fc1a4e))
* update camunda-platform-digests ([#4772](https://github.com/camunda/camunda-platform-helm/issues/4772)) ([e44a39b](https://github.com/camunda/camunda-platform-helm/commit/e44a39bc0621393869a94bd026a87042130da061))
* update camunda-platform-digests ([#4787](https://github.com/camunda/camunda-platform-helm/issues/4787)) ([862082d](https://github.com/camunda/camunda-platform-helm/commit/862082d8bb3ce85d499f500140f2029706eab472))
* update camunda-platform-digests ([#4816](https://github.com/camunda/camunda-platform-helm/issues/4816)) ([ac05efc](https://github.com/camunda/camunda-platform-helm/commit/ac05efc33cf8ce730dda8a8878c660e6bdbbb65a))
* update camunda-platform-digests ([#4818](https://github.com/camunda/camunda-platform-helm/issues/4818)) ([965345c](https://github.com/camunda/camunda-platform-helm/commit/965345c6f3f5fbbff806e15c0781baf55710af9f))
* update camunda-platform-digests ([#4828](https://github.com/camunda/camunda-platform-helm/issues/4828)) ([5b459cb](https://github.com/camunda/camunda-platform-helm/commit/5b459cbb7442c04f1f39e6b6d7b76c45dbd854a0))
* update camunda-platform-digests ([#4840](https://github.com/camunda/camunda-platform-helm/issues/4840)) ([6d6c2b4](https://github.com/camunda/camunda-platform-helm/commit/6d6c2b4ae96671b2c1e5405e79a5a9f67acb9677))
* update camunda-platform-images (patch) ([#4713](https://github.com/camunda/camunda-platform-helm/issues/4713)) ([7c59886](https://github.com/camunda/camunda-platform-helm/commit/7c59886d69d49d702bd5b3e1acf5cf22a7af38bf))
* update camunda-platform-images (patch) ([#4792](https://github.com/camunda/camunda-platform-helm/issues/4792)) ([fd7294c](https://github.com/camunda/camunda-platform-helm/commit/fd7294c95d621b4d7d1c1d290b703d6209e61b44))
* update minor-updates (minor) ([#4712](https://github.com/camunda/camunda-platform-helm/issues/4712)) ([4cf435c](https://github.com/camunda/camunda-platform-helm/commit/4cf435c5aa989eaab1b0dde9cbc75fb694774854))
* update minor-updates (minor) ([#4765](https://github.com/camunda/camunda-platform-helm/issues/4765)) ([54dc74d](https://github.com/camunda/camunda-platform-helm/commit/54dc74d5fed86702a26a63f247d7ccc25424946a))
* update module golang.org/x/crypto to v0.45.0 [security] ([#4745](https://github.com/camunda/camunda-platform-helm/issues/4745)) ([1b31ade](https://github.com/camunda/camunda-platform-helm/commit/1b31aded5d1e7297e9648ad2e225b86f716a3b3e))
* update module golang.org/x/oauth2 to v0.27.0 [security] ([#4731](https://github.com/camunda/camunda-platform-helm/issues/4731)) ([ee2f502](https://github.com/camunda/camunda-platform-helm/commit/ee2f5024283bc4ab1992ead7755387435e3bfcc3))
* update patch-updates ([#4761](https://github.com/camunda/camunda-platform-helm/issues/4761)) ([89f5551](https://github.com/camunda/camunda-platform-helm/commit/89f55518ddeaeec8fb0423afd173cd39e631ea95))
* update patch-updates (patch) ([#4762](https://github.com/camunda/camunda-platform-helm/issues/4762)) ([f8e7bbd](https://github.com/camunda/camunda-platform-helm/commit/f8e7bbd242097bb2c7c7bfde54aa53b3a5077af2))
* update patch-updates (patch) ([#4831](https://github.com/camunda/camunda-platform-helm/issues/4831)) ([c77bbe5](https://github.com/camunda/camunda-platform-helm/commit/c77bbe52c428f1a22597a76c19c0b26a40d6a8b7))
