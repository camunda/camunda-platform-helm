# Changelog

## [13.5.2](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.8-13.5.1...camunda-platform-8.8-13.5.2) (2026-02-26)


### Dependencies

* update camunda/console docker tag to v8.8.104 ([#5189](https://github.com/camunda/camunda-platform-helm/issues/5189)) ([fa1d1b1](https://github.com/camunda/camunda-platform-helm/commit/fa1d1b14503a17f5aa8839c2f78fbc0357a880d6))

## [13.5.1](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.8-13.5.0...camunda-platform-8.8-13.5.1) (2026-02-25)


### Dependencies

* update camunda-platform-images (patch) ([#5182](https://github.com/camunda/camunda-platform-helm/issues/5182)) ([3ba8e07](https://github.com/camunda/camunda-platform-helm/commit/3ba8e07b58a5e1ca9239081bf3ba0e2d6a5a85e3))

## [13.5.0](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.8-13.4.2...camunda-platform-8.8-13.5.0) (2026-02-19)


### Features

* add configuration for initializing authorization in 8.8 ([#5096](https://github.com/camunda/camunda-platform-helm/issues/5096)) ([1d65af1](https://github.com/camunda/camunda-platform-helm/commit/1d65af1c688817b871f729e264ce8b8d73bf34b6))
* add tpl support to podLabels, podAnnotations, and global.ingress.host ([#5064](https://github.com/camunda/camunda-platform-helm/issues/5064)) ([b1d64aa](https://github.com/camunda/camunda-platform-helm/commit/b1d64aaa2a9d633e2b266a2abae9b7b07b0db6b0))
* expose options in values.yaml for helm v4 support ([#4918](https://github.com/camunda/camunda-platform-helm/issues/4918)) ([ec0fb7f](https://github.com/camunda/camunda-platform-helm/commit/ec0fb7f62af76b07b5fb970099781ddb4901ef68))


### Bug Fixes

* add type to headless orchestration cluster service ([#5107](https://github.com/camunda/camunda-platform-helm/issues/5107)) ([5c57502](https://github.com/camunda/camunda-platform-helm/commit/5c5750232ccf29d39bb68b1509d9131603536429))
* normalize authIssuerBackendUrl to prevent double-slash when contextPath is root ([#5114](https://github.com/camunda/camunda-platform-helm/issues/5114)) ([8525c94](https://github.com/camunda/camunda-platform-helm/commit/8525c942bbaf881200bb58aaca20042f305dea07))


### Dependencies

* update camunda-platform-digests ([#5087](https://github.com/camunda/camunda-platform-helm/issues/5087)) ([16b1eb7](https://github.com/camunda/camunda-platform-helm/commit/16b1eb7877fd61a160b76d730e6b60e8f32847b9))
* update camunda-platform-digests ([#5094](https://github.com/camunda/camunda-platform-helm/issues/5094)) ([4f6dc63](https://github.com/camunda/camunda-platform-helm/commit/4f6dc634c9465737d6f286c9f3182174fc4868cf))
* update camunda-platform-digests ([#5106](https://github.com/camunda/camunda-platform-helm/issues/5106)) ([d1cadd3](https://github.com/camunda/camunda-platform-helm/commit/d1cadd3cbb695287c2ffec7c1f43811681cef29d))
* update camunda-platform-digests ([#5110](https://github.com/camunda/camunda-platform-helm/issues/5110)) ([edae209](https://github.com/camunda/camunda-platform-helm/commit/edae2098f5a9e4f3db231db1ca2cb3e15ee3f0e6))
* update camunda-platform-digests ([#5137](https://github.com/camunda/camunda-platform-helm/issues/5137)) ([4b4678f](https://github.com/camunda/camunda-platform-helm/commit/4b4678fc23eeeffc07f94953234da4709a3e9bd1))
* update camunda-platform-images (patch) ([#5125](https://github.com/camunda/camunda-platform-helm/issues/5125)) ([131d2b5](https://github.com/camunda/camunda-platform-helm/commit/131d2b5efc2189a593f466eddee7f15f9400994b))
* update camunda/camunda docker tag to v8.8.12 ([#5138](https://github.com/camunda/camunda-platform-helm/issues/5138)) ([9ed6738](https://github.com/camunda/camunda-platform-helm/commit/9ed6738c1026d350600c4cfe5a52480dc22a4896))
* update camunda/console docker tag to v8.8.90 ([#5089](https://github.com/camunda/camunda-platform-helm/issues/5089)) ([ef783f0](https://github.com/camunda/camunda-platform-helm/commit/ef783f0757ab374bde65957b059c9f3fee59e379))
* update minor-updates (minor) ([#5031](https://github.com/camunda/camunda-platform-helm/issues/5031)) ([8febe72](https://github.com/camunda/camunda-platform-helm/commit/8febe72311516c68444470bd08c9c59fff1db08f))
* update patch-updates (patch) ([#5033](https://github.com/camunda/camunda-platform-helm/issues/5033)) ([246572c](https://github.com/camunda/camunda-platform-helm/commit/246572c06b3508731446b0402aabb8d12b29f512))

## [13.4.2](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.8-13.4.1...camunda-platform-8.8-13.4.2) (2026-02-05)


### Bug Fixes

* support lower helm cli versions by conditionally using toYamlPretty ([#4978](https://github.com/camunda/camunda-platform-helm/issues/4978)) ([c0e1340](https://github.com/camunda/camunda-platform-helm/commit/c0e13407cd1ed6e1d4a3c2980cd3e43f6e1a473b))


### Dependencies

* update camunda-platform-digests ([#5055](https://github.com/camunda/camunda-platform-helm/issues/5055)) ([96301d9](https://github.com/camunda/camunda-platform-helm/commit/96301d9e0b19506e8543af902cebe78bb6f6ac2a))
* update camunda-platform-digests ([#5068](https://github.com/camunda/camunda-platform-helm/issues/5068)) ([32b6f45](https://github.com/camunda/camunda-platform-helm/commit/32b6f453a9fd01b4c5033ce53bb72a3e0e0cbf53))
* update camunda-platform-images (patch) ([#5053](https://github.com/camunda/camunda-platform-helm/issues/5053)) ([586ee9b](https://github.com/camunda/camunda-platform-helm/commit/586ee9b0ccb8414f9b57d474bb440c528719a2f0))
* update camunda-platform-images (patch) ([#5062](https://github.com/camunda/camunda-platform-helm/issues/5062)) ([3c81c8e](https://github.com/camunda/camunda-platform-helm/commit/3c81c8ee602ce924d0446e12ab03efe6440738f7))
* update camunda-platform-images (patch) ([#5081](https://github.com/camunda/camunda-platform-helm/issues/5081)) ([b95a4c6](https://github.com/camunda/camunda-platform-helm/commit/b95a4c660abce02da7f7cf14457a955bfef826fc))

## [13.4.1](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.8-13.4.0...camunda-platform-8.8-13.4.1) (2026-01-16)


### Dependencies

* update camunda-platform-digests ([#5012](https://github.com/camunda/camunda-platform-helm/issues/5012)) ([2bcf375](https://github.com/camunda/camunda-platform-helm/commit/2bcf375cb3005d6d881448e1a14c33299984947e))
* update camunda-platform-images (patch) ([#5027](https://github.com/camunda/camunda-platform-helm/issues/5027)) ([7ed7062](https://github.com/camunda/camunda-platform-helm/commit/7ed70626fc58c627c70ceb65d6e2db9baa6a0d3c))

## [13.4.0](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.8-13.3.2...camunda-platform-8.8-13.4.0) (2026-01-08)


### Features

* enable first user and add additional users with roles in qa scenario ([#4889](https://github.com/camunda/camunda-platform-helm/issues/4889)) ([159b39f](https://github.com/camunda/camunda-platform-helm/commit/159b39fe82262a24a9830e0b687e04f081ef7ffc))


### Bug Fixes

* remove constraint of identity.enabled=false and optimize.enabled=true ([#4910](https://github.com/camunda/camunda-platform-helm/issues/4910)) ([0753d6f](https://github.com/camunda/camunda-platform-helm/commit/0753d6fc58336e9538856c9dd88aca52777a40ac))


### Documentation

* update readme dependency section ([#4960](https://github.com/camunda/camunda-platform-helm/issues/4960)) ([3ddfb86](https://github.com/camunda/camunda-platform-helm/commit/3ddfb860ff8c4355a3ef2c0f2a5f71195f929e40))


### Dependencies

* update camunda-platform-digests ([#4891](https://github.com/camunda/camunda-platform-helm/issues/4891)) ([b51a8f0](https://github.com/camunda/camunda-platform-helm/commit/b51a8f0c8661193b738eb3e99bfeb679c3a92741))
* update camunda-platform-digests ([#4939](https://github.com/camunda/camunda-platform-helm/issues/4939)) ([2a45096](https://github.com/camunda/camunda-platform-helm/commit/2a450962ba4b0cc7ab374a13ac13f65ff64650a6))
* update camunda-platform-digests ([#4941](https://github.com/camunda/camunda-platform-helm/issues/4941)) ([e0be86d](https://github.com/camunda/camunda-platform-helm/commit/e0be86d30985e801c04072b66f45db1fe79af21b))
* update camunda-platform-digests ([#4963](https://github.com/camunda/camunda-platform-helm/issues/4963)) ([e514afd](https://github.com/camunda/camunda-platform-helm/commit/e514afd03d84da86a6dc2a1a2a00fb80d5d235ed))
* update camunda-platform-images (patch) ([#4923](https://github.com/camunda/camunda-platform-helm/issues/4923)) ([94829aa](https://github.com/camunda/camunda-platform-helm/commit/94829aaba5c970f84d0c6ccd01cec67a37d463e9))
* update camunda-platform-images (patch) ([#4946](https://github.com/camunda/camunda-platform-helm/issues/4946)) ([bceb9d1](https://github.com/camunda/camunda-platform-helm/commit/bceb9d13dee52708b4a625ee31e5f282d868fd99))
* update camunda-platform-images (patch) ([#4964](https://github.com/camunda/camunda-platform-helm/issues/4964)) ([9abc71b](https://github.com/camunda/camunda-platform-helm/commit/9abc71bf7b0d88bf340059fb66ebff3fe05d9120))
* update camunda/camunda docker tag to v8.8.9 ([#4952](https://github.com/camunda/camunda-platform-helm/issues/4952)) ([ce88e07](https://github.com/camunda/camunda-platform-helm/commit/ce88e076a2a87092dcddfb5fb4a7fbc44beabd99))
* update camunda/console docker tag to v8.8.70 ([#4928](https://github.com/camunda/camunda-platform-helm/issues/4928)) ([0ff4033](https://github.com/camunda/camunda-platform-helm/commit/0ff4033df8edd1e1926a265f4cb5c6a884c15e03))
* update camunda/identity docker tag to v8.8.6 ([#4945](https://github.com/camunda/camunda-platform-helm/issues/4945)) ([f3f0616](https://github.com/camunda/camunda-platform-helm/commit/f3f0616b26e9cf9ceba8f5c5a3af3e65d5d01489))
* update minor-updates (minor) ([#4929](https://github.com/camunda/camunda-platform-helm/issues/4929)) ([6a63cdc](https://github.com/camunda/camunda-platform-helm/commit/6a63cdc23cdc6d17b7cec3aa8ea55c40eae7d372))
* update patch-updates (patch) ([#4924](https://github.com/camunda/camunda-platform-helm/issues/4924)) ([8814e76](https://github.com/camunda/camunda-platform-helm/commit/8814e76c6fa71cc4db57051db959b4cec20ef9a1))
* update patch-updates (patch) ([#4965](https://github.com/camunda/camunda-platform-helm/issues/4965)) ([0f1fb59](https://github.com/camunda/camunda-platform-helm/commit/0f1fb590b96d7e124835b10c2a69ddbb31af4e34))

## [13.3.2](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.8-13.3.1...camunda-platform-8.8-13.3.2) (2025-12-11)


### Dependencies

* update camunda-platform-images (patch) ([#4885](https://github.com/camunda/camunda-platform-helm/issues/4885)) ([4ffcd1d](https://github.com/camunda/camunda-platform-helm/commit/4ffcd1dbde8b44b82def6dcb320330c5197e1cd1))

## [13.3.1](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.8-13.3.0...camunda-platform-8.8-13.3.1) (2025-12-10)


### Bug Fixes

* apply tpl to issuerBackendUrl ([#4858](https://github.com/camunda/camunda-platform-helm/issues/4858)) ([22b5cd7](https://github.com/camunda/camunda-platform-helm/commit/22b5cd74e7a3e952b17f752541c8233c5cd0f185))


### Dependencies

* update camunda-platform-digests ([#4846](https://github.com/camunda/camunda-platform-helm/issues/4846)) ([e89a081](https://github.com/camunda/camunda-platform-helm/commit/e89a081f6c53c7b8676917c88c1761d1c07ddc5c))
* update camunda-platform-digests ([#4856](https://github.com/camunda/camunda-platform-helm/issues/4856)) ([1994d36](https://github.com/camunda/camunda-platform-helm/commit/1994d369ec157bf0b474c8e83a59a71ddf8e7ba8))
* update camunda-platform-images (patch) ([#4848](https://github.com/camunda/camunda-platform-helm/issues/4848)) ([bcc02e8](https://github.com/camunda/camunda-platform-helm/commit/bcc02e832939bfcb6fa643befa11ef0701a883f7))
* update camunda-platform-images (patch) ([#4874](https://github.com/camunda/camunda-platform-helm/issues/4874)) ([3099888](https://github.com/camunda/camunda-platform-helm/commit/30998888f89795451f6e8e861b41e50c41707804))
* update camunda/optimize docker tag to v8.8.3 ([#4875](https://github.com/camunda/camunda-platform-helm/issues/4875)) ([a76574c](https://github.com/camunda/camunda-platform-helm/commit/a76574c5b23e3f3d5a20df03fb06bb799d2409f6))
* update patch-updates (patch) ([#4860](https://github.com/camunda/camunda-platform-helm/issues/4860)) ([b059be6](https://github.com/camunda/camunda-platform-helm/commit/b059be61080ee33c8d8ee9cfa5f0f4d2f4cdaf35))


### Refactors

* remove unused identity redirect-url ([#4853](https://github.com/camunda/camunda-platform-helm/issues/4853)) ([90c61e6](https://github.com/camunda/camunda-platform-helm/commit/90c61e66d4676b4ccadee71e6a593ab69df7f6d9))

## [13.3.0](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.8-13.2.2...camunda-platform-8.8-13.3.0) (2025-12-03)


### Features

* support password field for jks ([#4782](https://github.com/camunda/camunda-platform-helm/issues/4782)) ([1877932](https://github.com/camunda/camunda-platform-helm/commit/187793214ca68d989142120a9a018dbe38809deb))


### Bug Fixes

* correct extraVolumeMounts binding in importer deployment ([#4829](https://github.com/camunda/camunda-platform-helm/issues/4829)) ([beb6db3](https://github.com/camunda/camunda-platform-helm/commit/beb6db3115649cb6d8617ee60637ebe6f315b9a7))
* modeler webapp to websockets connection not using override option ([#4812](https://github.com/camunda/camunda-platform-helm/issues/4812)) ([339da02](https://github.com/camunda/camunda-platform-helm/commit/339da02a87add81852177530d2d3b0d5937dd73e))
* replace SNAPSHOT tags with stable versions in 8.8 values-digest.yaml ([#4826](https://github.com/camunda/camunda-platform-helm/issues/4826)) ([9b43fff](https://github.com/camunda/camunda-platform-helm/commit/9b43fff499e399072d9d08cb0365ac2886b0b654))


### Dependencies

* update camunda-platform-digests ([#4818](https://github.com/camunda/camunda-platform-helm/issues/4818)) ([965345c](https://github.com/camunda/camunda-platform-helm/commit/965345c6f3f5fbbff806e15c0781baf55710af9f))
* update camunda-platform-digests ([#4828](https://github.com/camunda/camunda-platform-helm/issues/4828)) ([5b459cb](https://github.com/camunda/camunda-platform-helm/commit/5b459cbb7442c04f1f39e6b6d7b76c45dbd854a0))
* update camunda-platform-images (patch) ([#4830](https://github.com/camunda/camunda-platform-helm/issues/4830)) ([02793c0](https://github.com/camunda/camunda-platform-helm/commit/02793c0cea5cd70ae1e327510a00230fdbaa3ef1))
* update patch-updates (patch) ([#4831](https://github.com/camunda/camunda-platform-helm/issues/4831)) ([c77bbe5](https://github.com/camunda/camunda-platform-helm/commit/c77bbe52c428f1a22597a76c19c0b26a40d6a8b7))

## [13.2.2](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.8-13.2.1...camunda-platform-8.8-13.2.2) (2025-11-28)


### Bug Fixes

* let helm chart support hybrid auth ([#4785](https://github.com/camunda/camunda-platform-helm/issues/4785)) ([cb06ece](https://github.com/camunda/camunda-platform-helm/commit/cb06ece477535c069b03ab5eff3729d9baf93d0a))


### Dependencies

* update camunda-platform-images (patch) ([#4792](https://github.com/camunda/camunda-platform-helm/issues/4792)) ([fd7294c](https://github.com/camunda/camunda-platform-helm/commit/fd7294c95d621b4d7d1c1d290b703d6209e61b44))
* update camunda/console docker tag to v8.8.52 ([#4803](https://github.com/camunda/camunda-platform-helm/issues/4803)) ([f499bc8](https://github.com/camunda/camunda-platform-helm/commit/f499bc812711b0f1cf425350637c175ed9c51609))
* update patch-updates ([#4761](https://github.com/camunda/camunda-platform-helm/issues/4761)) ([89f5551](https://github.com/camunda/camunda-platform-helm/commit/89f55518ddeaeec8fb0423afd173cd39e631ea95))

## [13.2.1](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.8-13.2.0...camunda-platform-8.8-13.2.1) (2025-11-25)


### Bug Fixes

* remove conditional rendering from management identity configmap ([#4771](https://github.com/camunda/camunda-platform-helm/issues/4771)) ([0dff2df](https://github.com/camunda/camunda-platform-helm/commit/0dff2df28c565b7d75722cd87c18a1dd82433a01))


### Dependencies

* update camunda-platform-images (patch) ([#4777](https://github.com/camunda/camunda-platform-helm/issues/4777)) ([18de26f](https://github.com/camunda/camunda-platform-helm/commit/18de26fe264049929c2edddb8e9aa04f3c213b94))

## [13.2.0](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.8-13.1.2...camunda-platform-8.8-13.2.0) (2025-11-21)


### Features

* backport custom client creation to 8.6 and 8.9 ([#4710](https://github.com/camunda/camunda-platform-helm/issues/4710)) ([68bec54](https://github.com/camunda/camunda-platform-helm/commit/68bec54d8f2e7147c2f75ff20c3314533ce0c3a7))
* define custom clients for management identity ([#4653](https://github.com/camunda/camunda-platform-helm/issues/4653)) ([b488a0b](https://github.com/camunda/camunda-platform-helm/commit/b488a0bfd44c3bf6558edcd96c15cdd2f3eb4b5f))
* define custom users through values.yaml ([#4670](https://github.com/camunda/camunda-platform-helm/issues/4670)) ([19ab9eb](https://github.com/camunda/camunda-platform-helm/commit/19ab9eb7e42fe84b76118a1930dd72bb6d302cdf))


### Bug Fixes

* 8.8 values-latest.yaml no longer references SNAPSHOT images ([#4727](https://github.com/camunda/camunda-platform-helm/issues/4727)) ([b9e560f](https://github.com/camunda/camunda-platform-helm/commit/b9e560f6baf44d84c53095f3faa1566dd2da71b4))
* allow for custom jks in migration-data job ([#4722](https://github.com/camunda/camunda-platform-helm/issues/4722)) ([978d127](https://github.com/camunda/camunda-platform-helm/commit/978d12770818badf84906fa46e3def1e68e704c9))
* connectors prefix should not be present if connectors is disabled ([#4570](https://github.com/camunda/camunda-platform-helm/issues/4570)) ([43648ff](https://github.com/camunda/camunda-platform-helm/commit/43648ffffe09aa60ea2f0eb9bd5cfdf58423a623))
* extraVolumeClaimTemplateTemplate indent for orchestration cluster ([#4697](https://github.com/camunda/camunda-platform-helm/issues/4697)) ([4c5387f](https://github.com/camunda/camunda-platform-helm/commit/4c5387f03688ab9c510e45dc92f97f7c0da9fac7))
* incorrect example for keycloak in readme.md ([#4586](https://github.com/camunda/camunda-platform-helm/issues/4586)) ([f6bf0a9](https://github.com/camunda/camunda-platform-helm/commit/f6bf0a9c125178b2cd3b15d465dc7ed0a59893b8))
* orchestration migration configurable init container image ([#4719](https://github.com/camunda/camunda-platform-helm/issues/4719)) ([f3174a0](https://github.com/camunda/camunda-platform-helm/commit/f3174a085c691df7f4c8048b08c87381c1afdacd))
* remove client env vars from qa scenario files ([#4726](https://github.com/camunda/camunda-platform-helm/issues/4726)) ([2c9ea12](https://github.com/camunda/camunda-platform-helm/commit/2c9ea121df9f402b19330e61dddbdd28ffbd4d35))
* remove leftover console secret constraints ([#4749](https://github.com/camunda/camunda-platform-helm/issues/4749)) ([80bd4de](https://github.com/camunda/camunda-platform-helm/commit/80bd4de8835b1216ed4ce52cc55b959f18d09d9a))
* setting zeebe image tag shouldnt disable broker profile ([#4587](https://github.com/camunda/camunda-platform-helm/issues/4587)) ([c2b0c7a](https://github.com/camunda/camunda-platform-helm/commit/c2b0c7a1bd6b9411c56dd17fde017058a1e2fabb))
* typo in webModeler pvc ([#4699](https://github.com/camunda/camunda-platform-helm/issues/4699)) ([d6a08c5](https://github.com/camunda/camunda-platform-helm/commit/d6a08c517fce6cd8e7eea7f59cb09790367e878c))
* typo lower case values ([#4737](https://github.com/camunda/camunda-platform-helm/issues/4737)) ([2ec2710](https://github.com/camunda/camunda-platform-helm/commit/2ec2710830d669e53a709bbb176c58ba064e12f2))


### Dependencies

* update camunda-platform-digests ([#4694](https://github.com/camunda/camunda-platform-helm/issues/4694)) ([b068811](https://github.com/camunda/camunda-platform-helm/commit/b0688112c9a505bd551f5795119a354bdb63afc9))
* update camunda-platform-digests ([#4702](https://github.com/camunda/camunda-platform-helm/issues/4702)) ([751b22b](https://github.com/camunda/camunda-platform-helm/commit/751b22bdfa0978a6a044d06310a303a1517f771f))
* update camunda-platform-digests ([#4704](https://github.com/camunda/camunda-platform-helm/issues/4704)) ([9c31cdc](https://github.com/camunda/camunda-platform-helm/commit/9c31cdc697a4cf9e60fda4c392b8213e9101537d))
* update camunda-platform-digests ([#4720](https://github.com/camunda/camunda-platform-helm/issues/4720)) ([8d69681](https://github.com/camunda/camunda-platform-helm/commit/8d696810a230633f09ff0b4a2921b7e4c954f832))
* update camunda-platform-digests ([#4724](https://github.com/camunda/camunda-platform-helm/issues/4724)) ([390de99](https://github.com/camunda/camunda-platform-helm/commit/390de99e51b6169aeb9ba6c44f9a84fb0f8e0d1a))
* update camunda-platform-digests ([#4743](https://github.com/camunda/camunda-platform-helm/issues/4743)) ([4a2c32a](https://github.com/camunda/camunda-platform-helm/commit/4a2c32a97b1b614a0b6f09a1d1adf78055fc1a4e))
* update camunda-platform-images (patch) ([#4713](https://github.com/camunda/camunda-platform-helm/issues/4713)) ([7c59886](https://github.com/camunda/camunda-platform-helm/commit/7c59886d69d49d702bd5b3e1acf5cf22a7af38bf))
* update camunda-platform-images (patch) ([#4732](https://github.com/camunda/camunda-platform-helm/issues/4732)) ([3445429](https://github.com/camunda/camunda-platform-helm/commit/3445429910e81e4077f8702535ee73659e35bff4))
* update minor-updates (minor) ([#4712](https://github.com/camunda/camunda-platform-helm/issues/4712)) ([4cf435c](https://github.com/camunda/camunda-platform-helm/commit/4cf435c5aa989eaab1b0dde9cbc75fb694774854))
* update module golang.org/x/crypto to v0.45.0 [security] ([#4745](https://github.com/camunda/camunda-platform-helm/issues/4745)) ([1b31ade](https://github.com/camunda/camunda-platform-helm/commit/1b31aded5d1e7297e9648ad2e225b86f716a3b3e))
