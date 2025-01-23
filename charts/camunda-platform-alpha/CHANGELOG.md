# Changelog

## [0.0.0-8.7.0-alpha2](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-alpha-v12.0.0-alpha3...camunda-platform-alpha-0.0.0-8.7.0-alpha2) (2025-01-23)


### ⚠ BREAKING CHANGES

* remove separated ingress functionality and only keep the global combined ingress
* Zeebe, Zeebe Gateway, Operate, and Tasklist have been replaced with Orchestration Core. Also, the Connectors configuration syntax was updated.

### Features

* add camunda exporter support to zeebe  ([#2476](https://github.com/camunda/camunda-platform-helm/issues/2476)) ([61ded39](https://github.com/camunda/camunda-platform-helm/commit/61ded39d2360a4b2aa8ce326af7ece2b346e8613))
* add core unified prefix for Elasticsearch/OpenSearch ([#2643](https://github.com/camunda/camunda-platform-helm/issues/2643)) ([68a5e5b](https://github.com/camunda/camunda-platform-helm/commit/68a5e5bff96c32c7a54977ebad52913a377b35e6))
* add support for authorizations configuration ([#2593](https://github.com/camunda/camunda-platform-helm/issues/2593)) ([ac4410e](https://github.com/camunda/camunda-platform-helm/commit/ac4410e6e02a61f6ef49a2def904a13f5fcf37c6))
* add tags and custom-properties keys to the default console config ([#2499](https://github.com/camunda/camunda-platform-helm/issues/2499)) ([2b16155](https://github.com/camunda/camunda-platform-helm/commit/2b161557735ee2a6aaabe88dac26266e52d4b831))
* adding `adaptSecurityContext` option in values.yaml for OpenShift SCC ([#2212](https://github.com/camunda/camunda-platform-helm/issues/2212)) ([d9aae33](https://github.com/camunda/camunda-platform-helm/commit/d9aae33801d9e58459199f116b984ea5101c4c50))
* adding application override to console ([#2594](https://github.com/camunda/camunda-platform-helm/issues/2594)) ([e9d0c48](https://github.com/camunda/camunda-platform-helm/commit/e9d0c4827d5e10a666abacce5d00d94b443418aa))
* adding aws config in apps for OpenSearch ([#2232](https://github.com/camunda/camunda-platform-helm/issues/2232)) ([9b77ec5](https://github.com/camunda/camunda-platform-helm/commit/9b77ec59cb71bd68438e503d8423c318777c03ed))
* adding play env var to web-modeler ([#2269](https://github.com/camunda/camunda-platform-helm/issues/2269)) ([fa99b6d](https://github.com/camunda/camunda-platform-helm/commit/fa99b6d7dee41857330307074ece21e7e78fd719))
* adding schema.json file to alpha (8.7) version ([#2537](https://github.com/camunda/camunda-platform-helm/issues/2537)) ([d7b4530](https://github.com/camunda/camunda-platform-helm/commit/d7b453030c533d6bfb2fd7508e444a48de99789c))
* adding TLS support to console ([#2505](https://github.com/camunda/camunda-platform-helm/issues/2505)) ([c32f5d4](https://github.com/camunda/camunda-platform-helm/commit/c32f5d4a911d0484001219df7b9a05f836c1b69f))
* **alpha:** add Connectors to release items ([#2659](https://github.com/camunda/camunda-platform-helm/issues/2659)) ([50a8392](https://github.com/camunda/camunda-platform-helm/commit/50a839276a485b33421b8e624a255fb3adfb7482))
* configure web-modeler restapi JWK Set URI ([#2704](https://github.com/camunda/camunda-platform-helm/issues/2704)) ([0be3045](https://github.com/camunda/camunda-platform-helm/commit/0be304587c72c25644f08e3520089065eff55a8a))
* execution identity integration ([#2247](https://github.com/camunda/camunda-platform-helm/issues/2247)) ([31a8ab1](https://github.com/camunda/camunda-platform-helm/commit/31a8ab1debf6eca66a516ff700b161cdfaf2db22))
* increase elasticsearch nodes to 3 ([#2298](https://github.com/camunda/camunda-platform-helm/issues/2298)) ([3b8f5eb](https://github.com/camunda/camunda-platform-helm/commit/3b8f5eb64e0d3ad72885d77d1d9e07adf310fe13))
* support Keycloak v25 ([#2255](https://github.com/camunda/camunda-platform-helm/issues/2255)) ([e72000e](https://github.com/camunda/camunda-platform-helm/commit/e72000e47075a9817371a18adbc0cf660ff335b3))
* support optional chart secrets auto-generation ([#2257](https://github.com/camunda/camunda-platform-helm/issues/2257)) ([37085aa](https://github.com/camunda/camunda-platform-helm/commit/37085aa650208e20568553b72f813a7c6a1216eb))
* support setting logging levels in all components ([#2324](https://github.com/camunda/camunda-platform-helm/issues/2324)) ([4c12bb8](https://github.com/camunda/camunda-platform-helm/commit/4c12bb8d5154dee3d6214632fd03e339934d1b35))
* **web-modeler:** add support for configuring Zeebe clusters ([#2489](https://github.com/camunda/camunda-platform-helm/issues/2489)) ([cdc6b70](https://github.com/camunda/camunda-platform-helm/commit/cdc6b70d78ad9d5e0a675467e7018c0de6a8e5a8))


### Bug Fixes

* **8.7:** wrong roles for identity user ([#2639](https://github.com/camunda/camunda-platform-helm/issues/2639)) ([13fc9dc](https://github.com/camunda/camunda-platform-helm/commit/13fc9dc4d1d6a5658dc852cbe197694544d5ad48))
* add default webModler url when ingress is disabled ([#2566](https://github.com/camunda/camunda-platform-helm/issues/2566)) ([678da17](https://github.com/camunda/camunda-platform-helm/commit/678da176b47323e3c63247e0b805a4d44f3979ed))
* add missing components when identity disabled. ([#2627](https://github.com/camunda/camunda-platform-helm/issues/2627)) ([7cf7f98](https://github.com/camunda/camunda-platform-helm/commit/7cf7f98665ebd906f803b49c0ae00d5af74c8b34))
* add zeebe opensearch retention to app config file ([#2250](https://github.com/camunda/camunda-platform-helm/issues/2250)) ([62c9c31](https://github.com/camunda/camunda-platform-helm/commit/62c9c31e3cb9c9bd92208bf65c6cb82ca7715152))
* added helper function smtp auth for webmodeler ([#2245](https://github.com/camunda/camunda-platform-helm/issues/2245)) ([b54fa13](https://github.com/camunda/camunda-platform-helm/commit/b54fa13a1de20e2ae54c143449fcd11dbec85afa))
* added support for optimize env vars for migration initcontainer ([#2710](https://github.com/camunda/camunda-platform-helm/issues/2710)) ([8fc1265](https://github.com/camunda/camunda-platform-helm/commit/8fc1265feba2e9ab5b2d386b53e54e6e0cea47b5))
* adding http/https options to readinessProbes for console ([#2529](https://github.com/camunda/camunda-platform-helm/issues/2529)) ([64a37f6](https://github.com/camunda/camunda-platform-helm/commit/64a37f66227ceb32b67c4f58b729206f6a5c5392))
* advertisedHost relies on global.multiregion.regions and not global.multiregion.enabled ([#2226](https://github.com/camunda/camunda-platform-helm/issues/2226)) ([59ac01a](https://github.com/camunda/camunda-platform-helm/commit/59ac01a20ef7f2b673e4fb9ee8de3ad126559440))
* **alpha:** add connectors init secret to identity ([245a28e](https://github.com/camunda/camunda-platform-helm/commit/245a28e1f18c3742607ff4884ee603d307259382))
* **alpha:** add connectors init secret to identity ([0a4441e](https://github.com/camunda/camunda-platform-helm/commit/0a4441e59cf23f10a699fd5df333fe611a029a23))
* **alpha:** default camunda license key ([cc675cf](https://github.com/camunda/camunda-platform-helm/commit/cc675cf9bd0fc53adf8361c4a18b22d93641714a))
* **alpha:** encrypt camunda license key ([62ab593](https://github.com/camunda/camunda-platform-helm/commit/62ab59344a782cdb06354dbe8df6562c5089f230))
* **alpha:** move core sts command to correct location ([#2712](https://github.com/camunda/camunda-platform-helm/issues/2712)) ([0b88c6f](https://github.com/camunda/camunda-platform-helm/commit/0b88c6fb0eb215feb7951bba74a6a2e3c6141b22))
* **alpha:** move identity admin client to presets ([#2714](https://github.com/camunda/camunda-platform-helm/issues/2714)) ([c671090](https://github.com/camunda/camunda-platform-helm/commit/c6710909bcb41259520de87a23947c4c8b52bb5e))
* **alpha:** set the zeebe prefix correctly ([ae9c825](https://github.com/camunda/camunda-platform-helm/commit/ae9c82512781c1c307eb20a96589d0b7575aa3e0))
* **alpha:** show error on usage of core.ingress.rest not core.ingress ([9318266](https://github.com/camunda/camunda-platform-helm/commit/93182668ade9cd99ce51423fca6869ea09504e82))
* assign Zeebe role to demo user ([#2510](https://github.com/camunda/camunda-platform-helm/issues/2510)) ([cd419a3](https://github.com/camunda/camunda-platform-helm/commit/cd419a3da7d3e1859bdbbf742bda554b4fd42eaa))
* changing `existingSecret.name` to comply with schema ([#2726](https://github.com/camunda/camunda-platform-helm/issues/2726)) ([c399f09](https://github.com/camunda/camunda-platform-helm/commit/c399f09e82d21cf11cbbcfd6ae9c61cd09d7b965))
* client-secret should only be present when string literal provided for oidc ([#2733](https://github.com/camunda/camunda-platform-helm/issues/2733)) ([6cd9313](https://github.com/camunda/camunda-platform-helm/commit/6cd9313aed1474d2c92143e7ea8b33ae3bd3a634))
* **core:** disable core client secret if identity is disabled ([#2584](https://github.com/camunda/camunda-platform-helm/issues/2584)) ([89a1333](https://github.com/camunda/camunda-platform-helm/commit/89a13330b2f71cfe30e3932c7e738d22b9d9711b))
* **core:** Identity authorization request not found ([#2641](https://github.com/camunda/camunda-platform-helm/issues/2641)) ([8d65dea](https://github.com/camunda/camunda-platform-helm/commit/8d65dea804d53bd2acf325e47958f232833857b3))
* **core:** small bug fixes and set correct values ([#2629](https://github.com/camunda/camunda-platform-helm/issues/2629)) ([e53ceaf](https://github.com/camunda/camunda-platform-helm/commit/e53ceafcd1d1fc25a324c619dfaad1157d94500e))
* correct ingress nginx annotation to activate proxy buffering by default ([#2304](https://github.com/camunda/camunda-platform-helm/issues/2304)) ([1e260e9](https://github.com/camunda/camunda-platform-helm/commit/1e260e9db34c349420237251156575f235d077f2))
* correctly intend operate migration envs ([#2238](https://github.com/camunda/camunda-platform-helm/issues/2238)) ([b795cfe](https://github.com/camunda/camunda-platform-helm/commit/b795cfea0c672b7598250b91621967acb161a0ff))
* define Web Modeler Admin role in identity ([#2395](https://github.com/camunda/camunda-platform-helm/issues/2395)) ([0c0263c](https://github.com/camunda/camunda-platform-helm/commit/0c0263c7b53aaf8ff9b3e2f28a5edae031bfbc2e))
* **deps:** update camunda-platform-alpha (patch) ([#2701](https://github.com/camunda/camunda-platform-helm/issues/2701)) ([a2661e2](https://github.com/camunda/camunda-platform-helm/commit/a2661e2767a6aaf1ff75bc485db152133f2a8116))
* **deps:** update module github.com/gruntwork-io/terratest to v0.47.2 ([#2399](https://github.com/camunda/camunda-platform-helm/issues/2399)) ([7753edf](https://github.com/camunda/camunda-platform-helm/commit/7753edf02055b1e9bfd9c5a42c5ba579bb1b41ce))
* **deps:** update module github.com/gruntwork-io/terratest to v0.48.0 ([#2665](https://github.com/camunda/camunda-platform-helm/issues/2665)) ([8027e66](https://github.com/camunda/camunda-platform-helm/commit/8027e66d9a4e27a53b2fe1e42ad0e385d0bc6bdd))
* **deps:** update module github.com/stretchr/testify to v1.10.0 ([#2609](https://github.com/camunda/camunda-platform-helm/issues/2609)) ([90097de](https://github.com/camunda/camunda-platform-helm/commit/90097dea2a6bfa678d405f2aa9ee6165c2cb57c3))
* do not set prefix ([#2506](https://github.com/camunda/camunda-platform-helm/issues/2506)) ([2fc97e7](https://github.com/camunda/camunda-platform-helm/commit/2fc97e78495ef8116ba4f0b0ce152896e62aea80))
* double-slash issue in health check paths and constraints for Zeebe Gateway ([#2355](https://github.com/camunda/camunda-platform-helm/issues/2355)) ([5a96d15](https://github.com/camunda/camunda-platform-helm/commit/5a96d15d03428a15612495987396acc0f17cb5fc))
* empty commit for releasable unit (release-please) ([023dca3](https://github.com/camunda/camunda-platform-helm/commit/023dca334710faf63a57da8aec970379a446f3a6))
* empty commit for releasable unit (release-please) ([#2766](https://github.com/camunda/camunda-platform-helm/issues/2766)) ([7c81e3d](https://github.com/camunda/camunda-platform-helm/commit/7c81e3db92a47be163a8bb7a4efe26cdfab10551))
* enable secrets deprecation flag in alpha by default ([#2081](https://github.com/camunda/camunda-platform-helm/issues/2081)) ([b791f4c](https://github.com/camunda/camunda-platform-helm/commit/b791f4cd6ac3859112b07a89fa6bc89a46d08313))
* **follow-up:** correct existingSecretKey for connectors inbound auth ([712ea6a](https://github.com/camunda/camunda-platform-helm/commit/712ea6a6b387f063e67238321b8a59134d4b2d16))
* gives port-forward hostnames to external urls when no ingress is… ([#1897](https://github.com/camunda/camunda-platform-helm/issues/1897)) ([d28a790](https://github.com/camunda/camunda-platform-helm/commit/d28a7902237340350027fb4709daa3bc278c9d21))
* identity baseUrl supplied regardless of multitenancy ([#2389](https://github.com/camunda/camunda-platform-helm/issues/2389)) ([a7e26b8](https://github.com/camunda/camunda-platform-helm/commit/a7e26b8e415228cc8c3619b4937e494388cbf527))
* Include opensearch env vars in operate initContainer (alpha) ([#2364](https://github.com/camunda/camunda-platform-helm/issues/2364)) ([a1f17e7](https://github.com/camunda/camunda-platform-helm/commit/a1f17e70eaad3677d38bc2ea201161529d3177e7))
* **openshift:** allow usage of the route with the default router ([#2646](https://github.com/camunda/camunda-platform-helm/issues/2646)) ([0b37e0f](https://github.com/camunda/camunda-platform-helm/commit/0b37e0fdd4c9de40f19a5ee1893668c54e8574e2))
* rearranged env vars necessary for oidc to work with console ([#2390](https://github.com/camunda/camunda-platform-helm/issues/2390)) ([f1edafd](https://github.com/camunda/camunda-platform-helm/commit/f1edafd5baab5031393f5988e248e3fad3a168f3))
* reload identity when its config changed ([#2234](https://github.com/camunda/camunda-platform-helm/issues/2234)) ([cb41059](https://github.com/camunda/camunda-platform-helm/commit/cb41059630597c4239886dff577c33b8488cb3f8))
* set identity client secret env var in alpha ([#2703](https://github.com/camunda/camunda-platform-helm/issues/2703)) ([d48086c](https://github.com/camunda/camunda-platform-helm/commit/d48086cb9f3a0d9b8b2a5fa3ff47b8bf12c478c6))
* set optimize global elasticsearch prefix ([#2491](https://github.com/camunda/camunda-platform-helm/issues/2491)) ([2805de0](https://github.com/camunda/camunda-platform-helm/commit/2805de0a10dfff30f511b8c7a96d9d9da2e1e941))
* up tasklist timeoutSeconds to 5 ([#2417](https://github.com/camunda/camunda-platform-helm/issues/2417)) ([9f3d199](https://github.com/camunda/camunda-platform-helm/commit/9f3d1999db4d7407c963f712dd4b338ef99bd0ae))
* update postgresql image tag to avoid bitnami broken release ([#2556](https://github.com/camunda/camunda-platform-helm/issues/2556)) ([d985bf2](https://github.com/camunda/camunda-platform-helm/commit/d985bf24092265feeddde859aa55d3e9f5199a00))
* warning added to NOTES about installationType ([#2328](https://github.com/camunda/camunda-platform-helm/issues/2328)) ([48e0356](https://github.com/camunda/camunda-platform-helm/commit/48e0356242a487ebfb151df798116843d2d02c09))
* zeebe gateway incorrect rest ingress path  ([#2516](https://github.com/camunda/camunda-platform-helm/issues/2516)) ([c9f6e3b](https://github.com/camunda/camunda-platform-helm/commit/c9f6e3bb387a5dc7d70e7746761def4ff64245b4))
* **zeebe-grpc-ingress:** class check for openshift was not checked ([#2678](https://github.com/camunda/camunda-platform-helm/issues/2678)) ([873dbd0](https://github.com/camunda/camunda-platform-helm/commit/873dbd08ca63292312e5965b2d5d43daeaa7da4f))


### Documentation

* notice of separated ingress being deprecated ([#2263](https://github.com/camunda/camunda-platform-helm/issues/2263)) ([2091f38](https://github.com/camunda/camunda-platform-helm/commit/2091f381d6d278a2ee4750e5270cef33a0c805a7))
* update of outdated url in the local kubernetes  section ([#2274](https://github.com/camunda/camunda-platform-helm/issues/2274)) ([83f8230](https://github.com/camunda/camunda-platform-helm/commit/83f8230d8f5b34d52294e6d3d1be449ffe6aee9c))


### Refactors

* **alpha:** adjust resources for core sts ([#2702](https://github.com/camunda/camunda-platform-helm/issues/2702)) ([b9102ee](https://github.com/camunda/camunda-platform-helm/commit/b9102ee8ff3ddf78378d4fb5b776ee7a31476749))
* **alpha:** enable connectors readinessProbe again ([cc74f31](https://github.com/camunda/camunda-platform-helm/commit/cc74f31c5d41a17e6636714cf39dfcabf8b06948))
* **alpha:** update connectors config for 8.7 ([#2657](https://github.com/camunda/camunda-platform-helm/issues/2657)) ([bdef34b](https://github.com/camunda/camunda-platform-helm/commit/bdef34bb6b65eb6baa6d87fcd18fdbd7f5699b07))
* **core:** add identity admin client ([#2602](https://github.com/camunda/camunda-platform-helm/issues/2602)) ([9acebbe](https://github.com/camunda/camunda-platform-helm/commit/9acebbeb81642664f0dc8b44df30fb009ca72890))
* **core:** rename core statefulset to avoid upgrade downtime ([#2581](https://github.com/camunda/camunda-platform-helm/issues/2581)) ([061b06d](https://github.com/camunda/camunda-platform-helm/commit/061b06d35936bf8995f03a1ea4bec276ecb6a94f))
* default keycloak ingress pathType to Prefix ([#2372](https://github.com/camunda/camunda-platform-helm/issues/2372)) ([377c18f](https://github.com/camunda/camunda-platform-helm/commit/377c18fc9e0316c6ee0d43b89759c8ffdaa58540))
* move identity remaining env vars to config ([#2400](https://github.com/camunda/camunda-platform-helm/issues/2400)) ([e6d2cb6](https://github.com/camunda/camunda-platform-helm/commit/e6d2cb660960f6a2815b9045fb14b7613d2e7884))
* parametrized hard-coded identity auth vars ([#2512](https://github.com/camunda/camunda-platform-helm/issues/2512)) ([8f5801b](https://github.com/camunda/camunda-platform-helm/commit/8f5801b866c348c4045ec76341e0de233c27a4d1))
* remove separated ingress functionality ([#2586](https://github.com/camunda/camunda-platform-helm/issues/2586)) ([3d12988](https://github.com/camunda/camunda-platform-helm/commit/3d12988720594bb6cc160bf246999cba89fecdea))
* remove support for global.multiregion.installationType ([#2588](https://github.com/camunda/camunda-platform-helm/issues/2588)) ([a04f88a](https://github.com/camunda/camunda-platform-helm/commit/a04f88a4073a130f715094d6bca9d5d4b4c419b0))
* remove unused web-modeler config map entries ([#2708](https://github.com/camunda/camunda-platform-helm/issues/2708)) ([bd74490](https://github.com/camunda/camunda-platform-helm/commit/bd744904ded9f3f308d07c2b3e62755ef6429cdc))
* remove/rename deprecated helm values file keys ([#2615](https://github.com/camunda/camunda-platform-helm/issues/2615)) ([9a7f111](https://github.com/camunda/camunda-platform-helm/commit/9a7f111f3615fff9c2c9a41bdef12daa0276fedf))
* replace zeebe and web-apps with camunda orchestration core ([28d7927](https://github.com/camunda/camunda-platform-helm/commit/28d79278105b365a61b51974ce5efb0400d160e0))
* switch 8.7 to 8.6 chart structure ([#2790](https://github.com/camunda/camunda-platform-helm/issues/2790)) ([24e9c1a](https://github.com/camunda/camunda-platform-helm/commit/24e9c1a2d57025dcd08a14fb2a324c4af4cdcbac))
* unify authorization configuration ([#2640](https://github.com/camunda/camunda-platform-helm/issues/2640)) ([3449db1](https://github.com/camunda/camunda-platform-helm/commit/3449db1c207bafd450d1d8f83f661816096d3718))
* update console docker repository ([#2379](https://github.com/camunda/camunda-platform-helm/issues/2379)) ([135d148](https://github.com/camunda/camunda-platform-helm/commit/135d148ea66650c3fce5b89ca6c449b61627eef8))
* using bitnami oci chart repository ([#2356](https://github.com/camunda/camunda-platform-helm/issues/2356)) ([18fa53e](https://github.com/camunda/camunda-platform-helm/commit/18fa53e914c4acca314014dada47b057c69cb2db))
