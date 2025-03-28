# Changelog

## [0.0.0-8.7.0-alpha2](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.7-12.0.0-alpha5...camunda-platform-8.7-0.0.0-8.7.0-alpha2) (2025-03-28)


### ⚠ BREAKING CHANGES

* remove separated ingress functionality and only keep the global combined ingress
* Zeebe, Zeebe Gateway, Operate, and Tasklist have been replaced with Orchestration Core. Also, the Connectors configuration syntax was updated.

### Features

* add core unified prefix for Elasticsearch/OpenSearch ([#2643](https://github.com/camunda/camunda-platform-helm/issues/2643)) ([68a5e5b](https://github.com/camunda/camunda-platform-helm/commit/68a5e5bff96c32c7a54977ebad52913a377b35e6))
* add global.extraManifests support for injecting arbitrary YAML ([#3050](https://github.com/camunda/camunda-platform-helm/issues/3050)) ([b22c596](https://github.com/camunda/camunda-platform-helm/commit/b22c59629a7e7e87f34a3c41cb05d53934ba18c4))
* add multi-backend document-store configuration ([#2836](https://github.com/camunda/camunda-platform-helm/issues/2836)) ([012e9d9](https://github.com/camunda/camunda-platform-helm/commit/012e9d93caa518167b8eac2a39b8c3de0ff7a2fb))
* add support for authorizations configuration ([#2593](https://github.com/camunda/camunda-platform-helm/issues/2593)) ([ac4410e](https://github.com/camunda/camunda-platform-helm/commit/ac4410e6e02a61f6ef49a2def904a13f5fcf37c6))
* added connectors headless svc to alpha charts ([#3244](https://github.com/camunda/camunda-platform-helm/issues/3244)) ([73fa397](https://github.com/camunda/camunda-platform-helm/commit/73fa397d12dd1f0e0163a057db76872ffb12bc4c))
* added connectors to console config ([#3015](https://github.com/camunda/camunda-platform-helm/issues/3015)) ([28576b8](https://github.com/camunda/camunda-platform-helm/commit/28576b863561b2972db600bbf06e8866e55cd6ef))
* adding application override to console ([#2594](https://github.com/camunda/camunda-platform-helm/issues/2594)) ([e9d0c48](https://github.com/camunda/camunda-platform-helm/commit/e9d0c4827d5e10a666abacce5d00d94b443418aa))
* adding schema.json file to alpha (8.7) version ([#2537](https://github.com/camunda/camunda-platform-helm/issues/2537)) ([d7b4530](https://github.com/camunda/camunda-platform-helm/commit/d7b453030c533d6bfb2fd7508e444a48de99789c))
* adding TLS support to console ([#2505](https://github.com/camunda/camunda-platform-helm/issues/2505)) ([c32f5d4](https://github.com/camunda/camunda-platform-helm/commit/c32f5d4a911d0484001219df7b9a05f836c1b69f))
* **alpha:** add Connectors to release items ([#2659](https://github.com/camunda/camunda-platform-helm/issues/2659)) ([50a8392](https://github.com/camunda/camunda-platform-helm/commit/50a839276a485b33421b8e624a255fb3adfb7482))
* bump Keycloak version for 8.7 and 8.8 to match expected support ([#3034](https://github.com/camunda/camunda-platform-helm/issues/3034)) ([d6e0388](https://github.com/camunda/camunda-platform-helm/commit/d6e0388c926c279df6a1059dd1191b0fad2c48f0))
* configure web-modeler restapi JWK Set URI ([#2704](https://github.com/camunda/camunda-platform-helm/issues/2704)) ([0be3045](https://github.com/camunda/camunda-platform-helm/commit/0be304587c72c25644f08e3520089065eff55a8a))
* **web-modeler:** add support for configuring Zeebe clusters [backport] ([#2853](https://github.com/camunda/camunda-platform-helm/issues/2853)) ([b769947](https://github.com/camunda/camunda-platform-helm/commit/b76994737808f5a6ce32cdbb76a63c8d073b3004))


### Bug Fixes

* **8.7:** wrong roles for identity user ([#2639](https://github.com/camunda/camunda-platform-helm/issues/2639)) ([13fc9dc](https://github.com/camunda/camunda-platform-helm/commit/13fc9dc4d1d6a5658dc852cbe197694544d5ad48))
* add default webModler url when ingress is disabled ([#2566](https://github.com/camunda/camunda-platform-helm/issues/2566)) ([678da17](https://github.com/camunda/camunda-platform-helm/commit/678da176b47323e3c63247e0b805a4d44f3979ed))
* add empty commit to alpha ([#2901](https://github.com/camunda/camunda-platform-helm/issues/2901)) ([755fce0](https://github.com/camunda/camunda-platform-helm/commit/755fce0044ceaec4895f8e54ce7871a96b54bcea))
* add missing components when identity disabled. ([#2627](https://github.com/camunda/camunda-platform-helm/issues/2627)) ([7cf7f98](https://github.com/camunda/camunda-platform-helm/commit/7cf7f98665ebd906f803b49c0ae00d5af74c8b34))
* add new line ([#2908](https://github.com/camunda/camunda-platform-helm/issues/2908)) ([5dbe614](https://github.com/camunda/camunda-platform-helm/commit/5dbe614594f960bf4d525fd3831de0064164c0a7))
* add unit test for documentstore configurations ([#2975](https://github.com/camunda/camunda-platform-helm/issues/2975)) ([e13f551](https://github.com/camunda/camunda-platform-helm/commit/e13f551d35dd59bcae11439a34b6b90681fe508d))
* add zeebe role ([#2889](https://github.com/camunda/camunda-platform-helm/issues/2889)) ([7be99e7](https://github.com/camunda/camunda-platform-helm/commit/7be99e7ef6d2bc2e2ca02919ba40ed48eeedc7cb))
* added support for optimize env vars for migration initcontainer ([#2710](https://github.com/camunda/camunda-platform-helm/issues/2710)) ([8fc1265](https://github.com/camunda/camunda-platform-helm/commit/8fc1265feba2e9ab5b2d386b53e54e6e0cea47b5))
* adding http/https options to readinessProbes for console ([#2529](https://github.com/camunda/camunda-platform-helm/issues/2529)) ([64a37f6](https://github.com/camunda/camunda-platform-helm/commit/64a37f66227ceb32b67c4f58b729206f6a5c5392))
* **alpha:** add connectors init secret to identity ([245a28e](https://github.com/camunda/camunda-platform-helm/commit/245a28e1f18c3742607ff4884ee603d307259382))
* **alpha:** add connectors init secret to identity ([0a4441e](https://github.com/camunda/camunda-platform-helm/commit/0a4441e59cf23f10a699fd5df333fe611a029a23))
* **alpha:** move core sts command to correct location ([#2712](https://github.com/camunda/camunda-platform-helm/issues/2712)) ([0b88c6f](https://github.com/camunda/camunda-platform-helm/commit/0b88c6fb0eb215feb7951bba74a6a2e3c6141b22))
* **alpha:** move identity admin client to presets ([#2714](https://github.com/camunda/camunda-platform-helm/issues/2714)) ([c671090](https://github.com/camunda/camunda-platform-helm/commit/c6710909bcb41259520de87a23947c4c8b52bb5e))
* **alpha:** set the zeebe prefix correctly ([ae9c825](https://github.com/camunda/camunda-platform-helm/commit/ae9c82512781c1c307eb20a96589d0b7575aa3e0))
* **alpha:** show error on usage of core.ingress.rest not core.ingress ([9318266](https://github.com/camunda/camunda-platform-helm/commit/93182668ade9cd99ce51423fca6869ea09504e82))
* apply password key from subchart ([#2799](https://github.com/camunda/camunda-platform-helm/issues/2799)) ([a708991](https://github.com/camunda/camunda-platform-helm/commit/a70899107aafc9360aee09c47316ba3f19ec1262))
* assign Zeebe role to demo user ([#2510](https://github.com/camunda/camunda-platform-helm/issues/2510)) ([cd419a3](https://github.com/camunda/camunda-platform-helm/commit/cd419a3da7d3e1859bdbbf742bda554b4fd42eaa))
* backport console override from 8.8 to 8.7 ([#2891](https://github.com/camunda/camunda-platform-helm/issues/2891)) ([78436ba](https://github.com/camunda/camunda-platform-helm/commit/78436ba59d8656e907dde16c502a644155f93ac8))
* changing `existingSecret.name` to comply with schema ([#2726](https://github.com/camunda/camunda-platform-helm/issues/2726)) ([c399f09](https://github.com/camunda/camunda-platform-helm/commit/c399f09e82d21cf11cbbcfd6ae9c61cd09d7b965))
* changing helper function for identityURL ([#3084](https://github.com/camunda/camunda-platform-helm/issues/3084)) ([722b95e](https://github.com/camunda/camunda-platform-helm/commit/722b95ed70a8463fd90f170a25668a249c8fb492))
* client-secret should only be present when string literal provided for oidc ([#2733](https://github.com/camunda/camunda-platform-helm/issues/2733)) ([6cd9313](https://github.com/camunda/camunda-platform-helm/commit/6cd9313aed1474d2c92143e7ea8b33ae3bd3a634))
* configMap and values revised for documentStore ([#2998](https://github.com/camunda/camunda-platform-helm/issues/2998)) ([d7816ec](https://github.com/camunda/camunda-platform-helm/commit/d7816ec696adbf33f25e21f811ba71dd432c579b))
* **connectors:** correct config for new 8.7 ([#2855](https://github.com/camunda/camunda-platform-helm/issues/2855)) ([92fb1f4](https://github.com/camunda/camunda-platform-helm/commit/92fb1f42db69f401bd3999471aadb1cabe1dcc67))
* **core:** disable core client secret if identity is disabled ([#2584](https://github.com/camunda/camunda-platform-helm/issues/2584)) ([89a1333](https://github.com/camunda/camunda-platform-helm/commit/89a13330b2f71cfe30e3932c7e738d22b9d9711b))
* **core:** Identity authorization request not found ([#2641](https://github.com/camunda/camunda-platform-helm/issues/2641)) ([8d65dea](https://github.com/camunda/camunda-platform-helm/commit/8d65dea804d53bd2acf325e47958f232833857b3))
* **core:** small bug fixes and set correct values ([#2629](https://github.com/camunda/camunda-platform-helm/issues/2629)) ([e53ceaf](https://github.com/camunda/camunda-platform-helm/commit/e53ceafcd1d1fc25a324c619dfaad1157d94500e))
* correct linter subchart identitykeycloak ([#2944](https://github.com/camunda/camunda-platform-helm/issues/2944)) ([95546f1](https://github.com/camunda/camunda-platform-helm/commit/95546f137a17f10c05d610729637eff39014ad84))
* **deps:** update camunda-platform-alpha (patch) ([#2701](https://github.com/camunda/camunda-platform-helm/issues/2701)) ([a2661e2](https://github.com/camunda/camunda-platform-helm/commit/a2661e2767a6aaf1ff75bc485db152133f2a8116))
* **deps:** update module github.com/gruntwork-io/terratest to v0.48.0 ([#2665](https://github.com/camunda/camunda-platform-helm/issues/2665)) ([8027e66](https://github.com/camunda/camunda-platform-helm/commit/8027e66d9a4e27a53b2fe1e42ad0e385d0bc6bdd))
* **deps:** update module github.com/stretchr/testify to v1.10.0 ([#2609](https://github.com/camunda/camunda-platform-helm/issues/2609)) ([90097de](https://github.com/camunda/camunda-platform-helm/commit/90097dea2a6bfa678d405f2aa9ee6165c2cb57c3))
* disable secret autoGenerated flag since it causes race condition ([#2906](https://github.com/camunda/camunda-platform-helm/issues/2906)) ([ddbccd9](https://github.com/camunda/camunda-platform-helm/commit/ddbccd9089c517ba12cf401e1f2617ffda55738e))
* empty commit for releasable unit (release-please) ([023dca3](https://github.com/camunda/camunda-platform-helm/commit/023dca334710faf63a57da8aec970379a446f3a6))
* empty commit for releasable unit (release-please) ([#2766](https://github.com/camunda/camunda-platform-helm/issues/2766)) ([7c81e3d](https://github.com/camunda/camunda-platform-helm/commit/7c81e3db92a47be163a8bb7a4efe26cdfab10551))
* ensure app configs rendered correctly in ConfigMap ([#3071](https://github.com/camunda/camunda-platform-helm/issues/3071)) ([36fcfe3](https://github.com/camunda/camunda-platform-helm/commit/36fcfe3d7eef93b4d613ca6891ac18161e3add37))
* increase zeebeGateway resources ([#3025](https://github.com/camunda/camunda-platform-helm/issues/3025)) ([bb34dba](https://github.com/camunda/camunda-platform-helm/commit/bb34dba9a8d7062d03bbfde9c075b9c71e56e397))
* **openshift:** allow usage of the route with the default router ([#2646](https://github.com/camunda/camunda-platform-helm/issues/2646)) ([0b37e0f](https://github.com/camunda/camunda-platform-helm/commit/0b37e0fdd4c9de40f19a5ee1893668c54e8574e2))
* remove local storage unit test for doc handling ([#3141](https://github.com/camunda/camunda-platform-helm/issues/3141)) ([b109b02](https://github.com/camunda/camunda-platform-helm/commit/b109b026f3654a56130e565299cf8ebbc4793cfd))
* remove localstorage support for documentStore ([#3013](https://github.com/camunda/camunda-platform-helm/issues/3013)) ([4f8f687](https://github.com/camunda/camunda-platform-helm/commit/4f8f687cedeb20cdb6476ef0e680ebfbbe21a008))
* remove unused test connection pod ([#3001](https://github.com/camunda/camunda-platform-helm/issues/3001)) ([9d2309a](https://github.com/camunda/camunda-platform-helm/commit/9d2309ab50c3bc1e3bb0fb2d0b7e6a27ed587200))
* revert "test: disable tests for disabled components ([#3151](https://github.com/camunda/camunda-platform-helm/issues/3151))" ([#3159](https://github.com/camunda/camunda-platform-helm/issues/3159)) ([1dae2a9](https://github.com/camunda/camunda-platform-helm/commit/1dae2a9bd2f414746d1fc052051988c42104992e))
* set bucketTtl for aws as int instead of string ([#2982](https://github.com/camunda/camunda-platform-helm/issues/2982)) ([243c842](https://github.com/camunda/camunda-platform-helm/commit/243c8423cfe9328cdf12bc6aaf053df4347f03b7))
* set identity client secret env var in alpha ([#2703](https://github.com/camunda/camunda-platform-helm/issues/2703)) ([d48086c](https://github.com/camunda/camunda-platform-helm/commit/d48086cb9f3a0d9b8b2a5fa3ff47b8bf12c478c6))
* update postgresql image tag to avoid bitnami broken release ([#2556](https://github.com/camunda/camunda-platform-helm/issues/2556)) ([d985bf2](https://github.com/camunda/camunda-platform-helm/commit/d985bf24092265feeddde859aa55d3e9f5199a00))
* use client credentials authentication for default cluster in Web Modeler if Entra ID is used ([#3232](https://github.com/camunda/camunda-platform-helm/issues/3232)) ([a7bcef1](https://github.com/camunda/camunda-platform-helm/commit/a7bcef10eb038d92d825f24da7ccd56d0ee9cfce))
* **webmodeler:** correct indentation for GCP volumeMounts ([#2979](https://github.com/camunda/camunda-platform-helm/issues/2979)) ([9939548](https://github.com/camunda/camunda-platform-helm/commit/9939548603702640c9015298925ce3bc4d08ea3b))
* **zeebe-grpc-ingress:** class check for openshift was not checked ([#2678](https://github.com/camunda/camunda-platform-helm/issues/2678)) ([873dbd0](https://github.com/camunda/camunda-platform-helm/commit/873dbd08ca63292312e5965b2d5d43daeaa7da4f))


### Documentation

* add prerelease annotation to alpha charts ([#3040](https://github.com/camunda/camunda-platform-helm/issues/3040)) ([aec4b5e](https://github.com/camunda/camunda-platform-helm/commit/aec4b5eed22e8d0928da8dae804e172cbe26033a))
* remove executionIdentity as it's still non-stable ([#2846](https://github.com/camunda/camunda-platform-helm/issues/2846)) ([53dcb07](https://github.com/camunda/camunda-platform-helm/commit/53dcb07f1ad234e3feede53752b9aab24b5312f1))
* update keywords of charts ([#3027](https://github.com/camunda/camunda-platform-helm/issues/3027)) ([7ce4275](https://github.com/camunda/camunda-platform-helm/commit/7ce4275968bb4ba4504a254ac4f02d2318be47d7))


### Dependencies

* update bitnami/postgresql docker tag to v14.17.0-debian-12-r2 ([#3043](https://github.com/camunda/camunda-platform-helm/issues/3043)) ([c895297](https://github.com/camunda/camunda-platform-helm/commit/c895297360eb03e238bacb98529a49e1e01b16a4))
* update bitnami/postgresql docker tag to v14.17.0-debian-12-r8 ([#3188](https://github.com/camunda/camunda-platform-helm/issues/3188)) ([07d4474](https://github.com/camunda/camunda-platform-helm/commit/07d44744a8e77c4bfce5ed36da402edc5c4f25e1))
* update bitnami/postgresql docker tag to v14.17.0-debian-12-r9 ([#3221](https://github.com/camunda/camunda-platform-helm/issues/3221)) ([0ef1b82](https://github.com/camunda/camunda-platform-helm/commit/0ef1b82247c4ad0fb7307ec977f7a8ec74a1805d))
* update camunda-platform-alpha (minor) ([#2794](https://github.com/camunda/camunda-platform-helm/issues/2794)) ([db8a69b](https://github.com/camunda/camunda-platform-helm/commit/db8a69b735c9ae1c66a52d4d7a76510df24a2007))
* update camunda-platform-alpha (patch) ([#2797](https://github.com/camunda/camunda-platform-helm/issues/2797)) ([aa5aee4](https://github.com/camunda/camunda-platform-helm/commit/aa5aee427f3978fb825c907ebb24ae3d8270f0d1))
* update camunda-platform-alpha (patch) ([#3054](https://github.com/camunda/camunda-platform-helm/issues/3054)) ([0d503cc](https://github.com/camunda/camunda-platform-helm/commit/0d503ccb8ad424349f1b5d8e4a3c97ec60d77ee1))
* update camunda-platform-alpha (patch) ([#3061](https://github.com/camunda/camunda-platform-helm/issues/3061)) ([e57f76b](https://github.com/camunda/camunda-platform-helm/commit/e57f76b1d376b361b7bd091a43050b04059a8480))
* update camunda-platform-alpha (patch) ([#3174](https://github.com/camunda/camunda-platform-helm/issues/3174)) ([31febd2](https://github.com/camunda/camunda-platform-helm/commit/31febd2c7b22c5be89949a6a598ec51230260542))
* update camunda-platform-alpha (patch) ([#3229](https://github.com/camunda/camunda-platform-helm/issues/3229)) ([284f058](https://github.com/camunda/camunda-platform-helm/commit/284f058fd43c4191285a82cb1cee7dce6368379c))
* update camunda-platform-alpha to v8.7.0-alpha5 (patch) ([#3065](https://github.com/camunda/camunda-platform-helm/issues/3065)) ([4e19513](https://github.com/camunda/camunda-platform-helm/commit/4e195139e5a26c47a910f5bafb634f3327e82a50))
* update camunda/console docker tag to v8.7.0-alpha5 ([#3030](https://github.com/camunda/camunda-platform-helm/issues/3030)) ([86fcb57](https://github.com/camunda/camunda-platform-helm/commit/86fcb579505cfa612b14e78aad84bf9e42f9c6a2))
* update camunda/optimize docker tag to v8.7.0-alpha5 ([#3074](https://github.com/camunda/camunda-platform-helm/issues/3074)) ([7cffc52](https://github.com/camunda/camunda-platform-helm/commit/7cffc52c36e8e28ed5fc2f25071c353da5bf9287))
* update elasticsearch docker tag to v21.4.6 ([#2978](https://github.com/camunda/camunda-platform-helm/issues/2978)) ([c99e27c](https://github.com/camunda/camunda-platform-helm/commit/c99e27cae623cb79d4464733ee59575551cbce7f))
* update elasticsearch docker tag to v21.4.7 ([#3023](https://github.com/camunda/camunda-platform-helm/issues/3023)) ([0dd6589](https://github.com/camunda/camunda-platform-helm/commit/0dd658902d090310205c29220a523a6405cb6eb3))
* update elasticsearch docker tag to v21.4.8 ([#3069](https://github.com/camunda/camunda-platform-helm/issues/3069)) ([b1f8f3e](https://github.com/camunda/camunda-platform-helm/commit/b1f8f3ea3806f1bbc7ced478046704307df33436))
* update elasticsearch docker tag to v21.4.9 ([#3231](https://github.com/camunda/camunda-platform-helm/issues/3231)) ([06483a7](https://github.com/camunda/camunda-platform-helm/commit/06483a760a179154cb78b0e54512e29a381c7f58))
* update keycloak docker tag to v24.4.12 ([#3156](https://github.com/camunda/camunda-platform-helm/issues/3156)) ([a7379ad](https://github.com/camunda/camunda-platform-helm/commit/a7379adf4ee668351be4809eb3e062b0aed686a2))
* update module github.com/burntsushi/toml to v1.5.0 ([#3198](https://github.com/camunda/camunda-platform-helm/issues/3198)) ([dda4671](https://github.com/camunda/camunda-platform-helm/commit/dda46718fcc3cfe40fce3fac3b676261671c22e4))


### Refactors

* **alpha:** adjust resources for core sts ([#2702](https://github.com/camunda/camunda-platform-helm/issues/2702)) ([b9102ee](https://github.com/camunda/camunda-platform-helm/commit/b9102ee8ff3ddf78378d4fb5b776ee7a31476749))
* **alpha:** enable connectors readinessProbe again ([cc74f31](https://github.com/camunda/camunda-platform-helm/commit/cc74f31c5d41a17e6636714cf39dfcabf8b06948))
* **alpha:** update connectors config for 8.7 ([#2657](https://github.com/camunda/camunda-platform-helm/issues/2657)) ([bdef34b](https://github.com/camunda/camunda-platform-helm/commit/bdef34bb6b65eb6baa6d87fcd18fdbd7f5699b07))
* **core:** add identity admin client ([#2602](https://github.com/camunda/camunda-platform-helm/issues/2602)) ([9acebbe](https://github.com/camunda/camunda-platform-helm/commit/9acebbeb81642664f0dc8b44df30fb009ca72890))
* **core:** rename core statefulset to avoid upgrade downtime ([#2581](https://github.com/camunda/camunda-platform-helm/issues/2581)) ([061b06d](https://github.com/camunda/camunda-platform-helm/commit/061b06d35936bf8995f03a1ea4bec276ecb6a94f))
* remove separated ingress functionality ([#2586](https://github.com/camunda/camunda-platform-helm/issues/2586)) ([3d12988](https://github.com/camunda/camunda-platform-helm/commit/3d12988720594bb6cc160bf246999cba89fecdea))
* remove support for global.multiregion.installationType ([#2588](https://github.com/camunda/camunda-platform-helm/issues/2588)) ([a04f88a](https://github.com/camunda/camunda-platform-helm/commit/a04f88a4073a130f715094d6bca9d5d4b4c419b0))
* remove unused web-modeler config map entries ([#2708](https://github.com/camunda/camunda-platform-helm/issues/2708)) ([bd74490](https://github.com/camunda/camunda-platform-helm/commit/bd744904ded9f3f308d07c2b3e62755ef6429cdc))
* remove/rename deprecated helm values file keys ([#2615](https://github.com/camunda/camunda-platform-helm/issues/2615)) ([9a7f111](https://github.com/camunda/camunda-platform-helm/commit/9a7f111f3615fff9c2c9a41bdef12daa0276fedf))
* replace zeebe and web-apps with camunda orchestration core ([28d7927](https://github.com/camunda/camunda-platform-helm/commit/28d79278105b365a61b51974ce5efb0400d160e0))
* switch 8.7 to 8.6 chart structure ([#2790](https://github.com/camunda/camunda-platform-helm/issues/2790)) ([24e9c1a](https://github.com/camunda/camunda-platform-helm/commit/24e9c1a2d57025dcd08a14fb2a324c4af4cdcbac))
* unify authorization configuration ([#2640](https://github.com/camunda/camunda-platform-helm/issues/2640)) ([3449db1](https://github.com/camunda/camunda-platform-helm/commit/3449db1c207bafd450d1d8f83f661816096d3718))
* update apps deps ([c554fc5](https://github.com/camunda/camunda-platform-helm/commit/c554fc5354c4807172f55a39d0d74a51bd9031b4))
