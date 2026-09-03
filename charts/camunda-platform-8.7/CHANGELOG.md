# Changelog

## [12.13.4](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.7-12.13.3...camunda-platform-8.7-12.13.4) (2026-09-03)


### Bug Fixes

* stop exposing credentials in deployment output ([#6796](https://github.com/camunda/camunda-platform-helm/issues/6796)) ([df46015](https://github.com/camunda/camunda-platform-helm/commit/df460152d7606161360becd97e6265be9d26e295))


### Documentation

* document stable pod labels for Web Modeler anti-affinity rules ([#6931](https://github.com/camunda/camunda-platform-helm/issues/6931)) ([a374ffb](https://github.com/camunda/camunda-platform-helm/commit/a374ffb748e291947afe1874565870c4a744cc98))


### Dependencies

* update camunda-platform-images (patch) ([#6875](https://github.com/camunda/camunda-platform-helm/issues/6875)) ([f3d2215](https://github.com/camunda/camunda-platform-helm/commit/f3d22153b8e95e3349f02a2eb287f3dfb6c39f5a))
* update camunda-platform-images (patch) ([#6993](https://github.com/camunda/camunda-platform-helm/issues/6993)) ([b02f60f](https://github.com/camunda/camunda-platform-helm/commit/b02f60fbcf9115be8e9b997fdb5ad42576afb82a))
* update camunda-platform-images (patch) ([#7005](https://github.com/camunda/camunda-platform-helm/issues/7005)) ([a18abd0](https://github.com/camunda/camunda-platform-helm/commit/a18abd09d3cd717539791e74b7728f43a9a855dd))
* update camunda/connectors-bundle docker tag to v8.7.24 ([#6963](https://github.com/camunda/camunda-platform-helm/issues/6963)) ([98db95c](https://github.com/camunda/camunda-platform-helm/commit/98db95cbdb4b4181b04e893f284575cf57135815))
* update camunda/identity docker tag to v8.7.24 ([#6989](https://github.com/camunda/camunda-platform-helm/issues/6989)) ([3714865](https://github.com/camunda/camunda-platform-helm/commit/371486505eb2e2a0a28b2118fae660742c3538ec))
* update camunda/zeebe docker tag to v8.7.38 ([#6995](https://github.com/camunda/camunda-platform-helm/issues/6995)) ([821eac6](https://github.com/camunda/camunda-platform-helm/commit/821eac6e38d9211320fc446dccfce3de305f2224))
* update minor-updates (minor) ([#6936](https://github.com/camunda/camunda-platform-helm/issues/6936)) ([d9a1bfe](https://github.com/camunda/camunda-platform-helm/commit/d9a1bfe638bc454d75478bd49389ade83291a594))
* update patch-updates (patch) ([#6872](https://github.com/camunda/camunda-platform-helm/issues/6872)) ([dba7942](https://github.com/camunda/camunda-platform-helm/commit/dba794262404ddc4d329e6306e932990c9111fc9))
* update patch-updates (patch) ([#6923](https://github.com/camunda/camunda-platform-helm/issues/6923)) ([c9e6e67](https://github.com/camunda/camunda-platform-helm/commit/c9e6e6774fe84a904e87c249cf31db7c3d690b90))
* update patch-updates (patch) ([#7007](https://github.com/camunda/camunda-platform-helm/issues/7007)) ([3b6ae47](https://github.com/camunda/camunda-platform-helm/commit/3b6ae4724dd3d625a574b6c797e5ec408eb56707))
* update registry.camunda.cloud/keycloak-ee/keycloak docker tag to v26.7.2 ([#6930](https://github.com/camunda/camunda-platform-helm/issues/6930)) ([e9c539c](https://github.com/camunda/camunda-platform-helm/commit/e9c539c30c1d4d4affdccb4ccae953a990364b54))

## [12.13.3](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.7-12.13.2...camunda-platform-8.7-12.13.3) (2026-08-14)


### Dependencies

* update camunda-platform-images (patch) ([#6865](https://github.com/camunda/camunda-platform-helm/issues/6865)) ([0a2c6d3](https://github.com/camunda/camunda-platform-helm/commit/0a2c6d3d3538c3008d3c57925e1b2f1100373adc))
* update camunda-platform-images (patch) ([#6866](https://github.com/camunda/camunda-platform-helm/issues/6866)) ([2fe7b43](https://github.com/camunda/camunda-platform-helm/commit/2fe7b439178a9273777400aaa1c91688778b6045))
* update camunda/console docker tag to v8.7.107 ([#6844](https://github.com/camunda/camunda-platform-helm/issues/6844)) ([844fa1d](https://github.com/camunda/camunda-platform-helm/commit/844fa1dbdf887de07c862b5c801b54eb69f74971))
* update camunda/console docker tag to v8.7.108 ([#6864](https://github.com/camunda/camunda-platform-helm/issues/6864)) ([ccfced0](https://github.com/camunda/camunda-platform-helm/commit/ccfced034d0e78ce1add8f663fea9ce312f7b302))
* update registry.camunda.cloud/vendor-ee/elasticsearch docker tag to v8.19.20 ([#6852](https://github.com/camunda/camunda-platform-helm/issues/6852)) ([46e0807](https://github.com/camunda/camunda-platform-helm/commit/46e080702e140cecf581113906f8e58d69b36686))

## [12.13.2](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.7-12.13.1...camunda-platform-8.7-12.13.2) (2026-08-05)


### Bug Fixes

* decouple requestBodySize from Zeebe maxMessageSize ([#6628](https://github.com/camunda/camunda-platform-helm/issues/6628)) ([7193ef6](https://github.com/camunda/camunda-platform-helm/commit/7193ef6c415587f1c97586d2274db385d042be31))
* omit AWS document-store credentials when secret is unset ([#6800](https://github.com/camunda/camunda-platform-helm/issues/6800)) ([619f62b](https://github.com/camunda/camunda-platform-helm/commit/619f62b2dad46299c6207bdf43cb0657efd056df))
* omit web-modeler restapi datasource password when none is configured ([#6578](https://github.com/camunda/camunda-platform-helm/issues/6578)) ([22cd3c3](https://github.com/camunda/camunda-platform-helm/commit/22cd3c392a7da56acb984b76dffdfbf2d53c8673))
* only use document store configuration where required ([#6724](https://github.com/camunda/camunda-platform-helm/issues/6724)) ([7f3bc57](https://github.com/camunda/camunda-platform-helm/commit/7f3bc5765dd4daa0329be92283075bb33ca1267f))
* respect explicit empty global.identity.keycloak.contextPath ([#6577](https://github.com/camunda/camunda-platform-helm/issues/6577)) ([d75be11](https://github.com/camunda/camunda-platform-helm/commit/d75be1136c945b0598abfe8d588242e1fb8b5238))
* revert vendor-ee/postgresql tag back to -r14 ([#6632](https://github.com/camunda/camunda-platform-helm/issues/6632)) ([2ac9425](https://github.com/camunda/camunda-platform-helm/commit/2ac942546be562d0712761a70fd3ab3f14375440))
* revert vendor-ee/postgresql to -r2 to restore amd64 image pulls ([#6622](https://github.com/camunda/camunda-platform-helm/issues/6622)) ([53fecde](https://github.com/camunda/camunda-platform-helm/commit/53fecded663c0473771bfba6ee5cfb28a93ada0b))


### Documentation

* clarify configMap describes the mounted ConfigMap volume settings, not config content ([#6604](https://github.com/camunda/camunda-platform-helm/issues/6604)) ([5713ac4](https://github.com/camunda/camunda-platform-helm/commit/5713ac44724416a3703d3114e728b871c5ce6bf9))
* signpost deploy-camunda config surface for external users ([#6564](https://github.com/camunda/camunda-platform-helm/issues/6564)) ([9d850bc](https://github.com/camunda/camunda-platform-helm/commit/9d850bc0ee25d5aebfcebf4685f5ec190a0e50f8))


### Dependencies

* update camunda-platform-images (patch) ([#6666](https://github.com/camunda/camunda-platform-helm/issues/6666)) ([aa1ea36](https://github.com/camunda/camunda-platform-helm/commit/aa1ea36df957686ecb2b9cfffa5a85b93fbe1325))
* update camunda-platform-images (patch) ([#6744](https://github.com/camunda/camunda-platform-helm/issues/6744)) ([23a3e0a](https://github.com/camunda/camunda-platform-helm/commit/23a3e0a18db3f6a8ed22006fb260c69da5585479))
* update camunda-platform-images (patch) ([#6765](https://github.com/camunda/camunda-platform-helm/issues/6765)) ([1626564](https://github.com/camunda/camunda-platform-helm/commit/1626564b81dd7a89d37e16ae6818a5c9c4ed1ea8))
* update camunda-platform-images (patch) ([#6777](https://github.com/camunda/camunda-platform-helm/issues/6777)) ([1f75621](https://github.com/camunda/camunda-platform-helm/commit/1f75621da4db53899d34b112f8841300a246e1e4))
* update camunda-platform-images (patch) ([#6786](https://github.com/camunda/camunda-platform-helm/issues/6786)) ([d81c52b](https://github.com/camunda/camunda-platform-helm/commit/d81c52b2ac91f85f8d99816af220c12cff521baa))
* update camunda-platform-images (patch) ([#6799](https://github.com/camunda/camunda-platform-helm/issues/6799)) ([b0b006b](https://github.com/camunda/camunda-platform-helm/commit/b0b006be9f9eb9d6680244bee4ab8cdffed56a05))
* update camunda/zeebe docker tag to v8.7.36 ([#6764](https://github.com/camunda/camunda-platform-helm/issues/6764)) ([1c54f22](https://github.com/camunda/camunda-platform-helm/commit/1c54f22ee2f8ee374333d574e9f88552674eff20))
* update minor-updates (minor) ([#6574](https://github.com/camunda/camunda-platform-helm/issues/6574)) ([3c346fd](https://github.com/camunda/camunda-platform-helm/commit/3c346fd1e6ffa130edae21b3e814c4202beed63d))
* update module golang.org/x/crypto to v0.52.0 [security] ([#6538](https://github.com/camunda/camunda-platform-helm/issues/6538)) ([619ea17](https://github.com/camunda/camunda-platform-helm/commit/619ea17cf0b69ba9bc5eedcf15d9cc9c7b771266))
* update patch-updates (patch) ([#6575](https://github.com/camunda/camunda-platform-helm/issues/6575)) ([ba9b3e6](https://github.com/camunda/camunda-platform-helm/commit/ba9b3e6b07eb7a754bc47636d8033cbcfb8b0ab0))
* update patch-updates (patch) ([#6625](https://github.com/camunda/camunda-platform-helm/issues/6625)) ([e673379](https://github.com/camunda/camunda-platform-helm/commit/e673379119693e926e0f8e35ec9816690f5a2c40))
* update patch-updates (patch) ([#6682](https://github.com/camunda/camunda-platform-helm/issues/6682)) ([759f232](https://github.com/camunda/camunda-platform-helm/commit/759f23228c3547d221c70d3da8981660165e7041))
* update patch-updates (patch) ([#6753](https://github.com/camunda/camunda-platform-helm/issues/6753)) ([077511c](https://github.com/camunda/camunda-platform-helm/commit/077511c5ab6d3c1b5ec34266cafbfca604332d94))

## [12.13.1](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.7-12.13.0...camunda-platform-8.7-12.13.1) (2026-07-08)


### Bug Fixes

* repair failing scheduled workflows for docs links, lychee, and values-latest ([#6492](https://github.com/camunda/camunda-platform-helm/issues/6492)) ([721fbc8](https://github.com/camunda/camunda-platform-helm/commit/721fbc88a0e792d511a29f0fe0ee16b885cce0a7))
* surface deprecation warnings on the gitops render path ([#6530](https://github.com/camunda/camunda-platform-helm/issues/6530)) ([7dd8905](https://github.com/camunda/camunda-platform-helm/commit/7dd89052c300a0c5146f0cc6f49b69a8be810271))


### Dependencies

* update camunda-platform-images (patch) ([#6465](https://github.com/camunda/camunda-platform-helm/issues/6465)) ([907dab6](https://github.com/camunda/camunda-platform-helm/commit/907dab62afdec4b0516392e6cb82a17deff5e786))
* update camunda-platform-images (patch) ([#6481](https://github.com/camunda/camunda-platform-helm/issues/6481)) ([cd112dc](https://github.com/camunda/camunda-platform-helm/commit/cd112dca1a8ac03beff048b1d6af12e7311f3458))
* update camunda-platform-images (patch) ([#6515](https://github.com/camunda/camunda-platform-helm/issues/6515)) ([6de8336](https://github.com/camunda/camunda-platform-helm/commit/6de8336137e7098e7e0c82d7ee3d70f757691785))
* update camunda-platform-images (patch) ([#6527](https://github.com/camunda/camunda-platform-helm/issues/6527)) ([8eaa968](https://github.com/camunda/camunda-platform-helm/commit/8eaa968cb3a587644edaf9e600915a71d147f9c4))
* update camunda-platform-images (patch) ([#6534](https://github.com/camunda/camunda-platform-helm/issues/6534)) ([c5f4544](https://github.com/camunda/camunda-platform-helm/commit/c5f45448579968dd0a543743db63eb23d1c9ec0b))
* update module golang.org/x/net to v0.55.0 [security] ([#6504](https://github.com/camunda/camunda-platform-helm/issues/6504)) ([0014dfb](https://github.com/camunda/camunda-platform-helm/commit/0014dfbf7757e665ccaaf6049d8811e94a8b75be))
* update registry.camunda.cloud/keycloak-ee/keycloak docker tag to v26.6.4 ([#6502](https://github.com/camunda/camunda-platform-helm/issues/6502)) ([a58c147](https://github.com/camunda/camunda-platform-helm/commit/a58c147cc7caffea0c4237f0ce30b5489fcdbab6))
* update registry.camunda.cloud/vendor-ee/elasticsearch docker tag to v8.19.18 ([#6491](https://github.com/camunda/camunda-platform-helm/issues/6491)) ([9f68888](https://github.com/camunda/camunda-platform-helm/commit/9f6888876b8203cf9eb4bc553055116874f4af22))

## [12.13.0](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.7-12.12.1...camunda-platform-8.7-12.13.0) (2026-06-26)


### Features

* **renovate:** source vendor-ee Bitnami images from the published datasource ([#6435](https://github.com/camunda/camunda-platform-helm/issues/6435)) ([9c57a9d](https://github.com/camunda/camunda-platform-helm/commit/9c57a9d6ccd87efd4b59faf3a5970c29b9ad6531))


### Dependencies

* update camunda-platform-images (patch) ([#6389](https://github.com/camunda/camunda-platform-helm/issues/6389)) ([040152b](https://github.com/camunda/camunda-platform-helm/commit/040152b82eee17ff26140e56bee8a2217d97b975))
* update camunda-platform-images (patch) ([#6437](https://github.com/camunda/camunda-platform-helm/issues/6437)) ([49c6fa5](https://github.com/camunda/camunda-platform-helm/commit/49c6fa53a8d7bc57e951a01f0489f876a2c06dda))
* update camunda-platform-images (patch) ([#6443](https://github.com/camunda/camunda-platform-helm/issues/6443)) ([9415cda](https://github.com/camunda/camunda-platform-helm/commit/9415cda25c24a4a4d09e6dfb9755b0a6bdcd0d62))
* update camunda-platform-images (patch) ([#6456](https://github.com/camunda/camunda-platform-helm/issues/6456)) ([82415fc](https://github.com/camunda/camunda-platform-helm/commit/82415fc4adbc993f013982a31cc58de623fc00af))
* update camunda-platform-images to v8.7.34 ([#6445](https://github.com/camunda/camunda-platform-helm/issues/6445)) ([5b06bc0](https://github.com/camunda/camunda-platform-helm/commit/5b06bc097262b7bfeba70881973d668bb31aa387))

## [12.12.1](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.7-12.12.0...camunda-platform-8.7-12.12.1) (2026-06-17)


### Documentation

* **version-matrix:** fix and version Bitnami Enterprise guide URL ([#6216](https://github.com/camunda/camunda-platform-helm/issues/6216)) ([c7b738f](https://github.com/camunda/camunda-platform-helm/commit/c7b738fcd3b91d875a2907970d5a382e489a2806))

## [12.12.0](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.7-12.11.0...camunda-platform-8.7-12.12.0) (2026-06-11)


### Features

* **8.7:** support password field for jks ([#5995](https://github.com/camunda/camunda-platform-helm/issues/5995)) ([c52c6f0](https://github.com/camunda/camunda-platform-helm/commit/c52c6f06feaee0e8ff311898b8e2963e7aa5bf5f))


### Dependencies

* update camunda-platform-images (patch) ([#6154](https://github.com/camunda/camunda-platform-helm/issues/6154)) ([2598594](https://github.com/camunda/camunda-platform-helm/commit/2598594cff5b3936e3f4478c679d4f8b4087235a))
* update camunda-platform-images (patch) ([#6367](https://github.com/camunda/camunda-platform-helm/issues/6367)) ([0973851](https://github.com/camunda/camunda-platform-helm/commit/0973851ba691c0d87f2fc0c0d49caac47dacf667))
* update minor-updates (minor) ([#6112](https://github.com/camunda/camunda-platform-helm/issues/6112)) ([02a27f3](https://github.com/camunda/camunda-platform-helm/commit/02a27f3da084312a971cdeac3699a661d28071c8))
* update patch-updates (patch) ([#6289](https://github.com/camunda/camunda-platform-helm/issues/6289)) ([2164f17](https://github.com/camunda/camunda-platform-helm/commit/2164f176bc967536bb9c9b1f279d9201f0711f50))

## [12.11.0](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.7-12.10.0...camunda-platform-8.7-12.11.0) (2026-06-05)


### Features

* **8.7:** add component-persistence CI scenario for issue 4767 ([#6007](https://github.com/camunda/camunda-platform-helm/issues/6007)) ([7e5a385](https://github.com/camunda/camunda-platform-helm/commit/7e5a385b81e48886da16b2b1312db728e7990fce))
* **8.7:** support password field for jks ([#5995](https://github.com/camunda/camunda-platform-helm/issues/5995)) ([c52c6f0](https://github.com/camunda/camunda-platform-helm/commit/c52c6f06feaee0e8ff311898b8e2963e7aa5bf5f))
* added service account labels ([#5877](https://github.com/camunda/camunda-platform-helm/issues/5877)) ([a616db5](https://github.com/camunda/camunda-platform-helm/commit/a616db54ef09b5f45ca1db868bc7918b05b18c7c))


### Bug Fixes

* **8.7:** bump enterprise Keycloak image to 26.3.4 ([#6213](https://github.com/camunda/camunda-platform-helm/issues/6213)) ([7e8c77b](https://github.com/camunda/camunda-platform-helm/commit/7e8c77bcd999378e9f5d48899024eec0d3cbfaaf))
* align upload size with Zeebe message limits ([#6279](https://github.com/camunda/camunda-platform-helm/issues/6279)) ([f61f37c](https://github.com/camunda/camunda-platform-helm/commit/f61f37c7cecdb073d4330a6a15f0c57fec0f1f78))
* correct enterprise Elasticsearch image tag to existing 8.19.15 ([#6307](https://github.com/camunda/camunda-platform-helm/issues/6307)) ([1efbd25](https://github.com/camunda/camunda-platform-helm/commit/1efbd25424e9f91591c652f9dd9e5591d0ee2e4a))


### Documentation

* **version-matrix:** add helm CLI versions per chart minor ([#6155](https://github.com/camunda/camunda-platform-helm/issues/6155)) ([20faed1](https://github.com/camunda/camunda-platform-helm/commit/20faed1e517e7581a903057c51fa131c790b44db))


### Dependencies

* update camunda-platform-images (patch) ([#6154](https://github.com/camunda/camunda-platform-helm/issues/6154)) ([2598594](https://github.com/camunda/camunda-platform-helm/commit/2598594cff5b3936e3f4478c679d4f8b4087235a))
* update camunda-platform-images (patch) ([#6367](https://github.com/camunda/camunda-platform-helm/issues/6367)) ([0973851](https://github.com/camunda/camunda-platform-helm/commit/0973851ba691c0d87f2fc0c0d49caac47dacf667))
* update hashicorp/vault-action action to v4 ([#6176](https://github.com/camunda/camunda-platform-helm/issues/6176)) ([5c4f611](https://github.com/camunda/camunda-platform-helm/commit/5c4f611acff4d3bcdea89ae8e6e5706078fb06fb))
* update minor-updates (minor) ([#6112](https://github.com/camunda/camunda-platform-helm/issues/6112)) ([02a27f3](https://github.com/camunda/camunda-platform-helm/commit/02a27f3da084312a971cdeac3699a661d28071c8))
* update module github.com/jackc/pgx/v5 to v5.9.2 [security] ([#5843](https://github.com/camunda/camunda-platform-helm/issues/5843)) ([b37bb34](https://github.com/camunda/camunda-platform-helm/commit/b37bb34c7b90ed6187598c92f6a0069801a810dc))
* update patch-updates (patch) ([#5837](https://github.com/camunda/camunda-platform-helm/issues/5837)) ([baee40a](https://github.com/camunda/camunda-platform-helm/commit/baee40afccd73d91a8943f2723017ee80116bc41))
* update patch-updates (patch) ([#6289](https://github.com/camunda/camunda-platform-helm/issues/6289)) ([2164f17](https://github.com/camunda/camunda-platform-helm/commit/2164f176bc967536bb9c9b1f279d9201f0711f50))


### Refactors

* **8.10:** drop bundled Bitnami subcharts ([#6146](https://github.com/camunda/camunda-platform-helm/issues/6146)) ([4374cff](https://github.com/camunda/camunda-platform-helm/commit/4374cff37d2e3b7cfaa2e661b0893705e5fbc14e))

## [12.10.0](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.7-12.9.0...camunda-platform-8.7-12.10.0) (2026-05-19)


### Features

* explicit lifecycle fixtures in ci-test-config.yaml ([#6103](https://github.com/camunda/camunda-platform-helm/issues/6103)) ([027f327](https://github.com/camunda/camunda-platform-helm/commit/027f327aae50a2e968912efe939153f59fab6991))


### Bug Fixes

* **8.7:** remove duplicate migration env vars rejected by Helm v4 SSA ([#6158](https://github.com/camunda/camunda-platform-helm/issues/6158)) ([72bc2f7](https://github.com/camunda/camunda-platform-helm/commit/72bc2f7c61c5a1ca9d3a8abdff02eb0adfb8c727))
* **ci:** apply SNAPSHOT image tags to 8.8 OpenSearch QA scenarios ([#6097](https://github.com/camunda/camunda-platform-helm/issues/6097)) ([65ad7a4](https://github.com/camunda/camunda-platform-helm/commit/65ad7a4dc14d65b1be6d54ee0250c2967a78bc5b))
* normalize trailing whitespace in golden file writer ([#6159](https://github.com/camunda/camunda-platform-helm/issues/6159)) ([e3ab8f5](https://github.com/camunda/camunda-platform-helm/commit/e3ab8f555626bfc00b1f9ff6f98999148ffc2b3e))
* test Helm 3 or 4 and use helm v4 for dev ([#5918](https://github.com/camunda/camunda-platform-helm/issues/5918)) ([8ddffec](https://github.com/camunda/camunda-platform-helm/commit/8ddffec8afa14e000b63f7d4033d3fc72e095d6c))


### Dependencies

* update camunda-platform-images (patch) ([#6102](https://github.com/camunda/camunda-platform-helm/issues/6102)) ([75a61d9](https://github.com/camunda/camunda-platform-helm/commit/75a61d9f863aff8ac4ddb27189681aa6e13f1dc6))
* update module github.com/gruntwork-io/terratest to v1 ([#6157](https://github.com/camunda/camunda-platform-helm/issues/6157)) ([eaf63e7](https://github.com/camunda/camunda-platform-helm/commit/eaf63e750cd23490b8b0cc1dc01b4ffcf41d2be5))

## [12.9.0](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.7-12.8.6...camunda-platform-8.7-12.9.0) (2026-05-07)


### Features

* **8.7:** add global.rba.enabled to consolidate RBA configuration ([#5999](https://github.com/camunda/camunda-platform-helm/issues/5999)) ([e73ed1c](https://github.com/camunda/camunda-platform-helm/commit/e73ed1c91f8ac832753080ba9e3f200b8a2f60ed))
* add elasticsearch-self-signed-upgrade scenario for 8.7-&gt;8.8 TLS upgrade testing ([#5974](https://github.com/camunda/camunda-platform-helm/issues/5974)) ([77145ba](https://github.com/camunda/camunda-platform-helm/commit/77145ba9550f65fc57044e4cef6e09e2cef5d1b9))


### Bug Fixes

* **ci:** extend nightly matrix coverage for missing platform/flow combinations ([#6064](https://github.com/camunda/camunda-platform-helm/issues/6064)) ([05e1f6d](https://github.com/camunda/camunda-platform-helm/commit/05e1f6d2fe50c9409970cfbcb35faaefe83c7656))
* **images:** use keycloak-ee/keycloak for 8.7 enterprise values ([#5969](https://github.com/camunda/camunda-platform-helm/issues/5969)) ([8dd5baa](https://github.com/camunda/camunda-platform-helm/commit/8dd5baad9aab86231118ad60505a4011d6e84bdf))
* resolve SM nightly upgrade scenario failures for groups 2-4 ([#5984](https://github.com/camunda/camunda-platform-helm/issues/5984)) ([1b1e8e4](https://github.com/camunda/camunda-platform-helm/commit/1b1e8e44f49c5c247239c5a3e2d2bbf98c2273e3))

## [12.8.6](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.7-12.8.5...camunda-platform-8.7-12.8.6) (2026-04-22)


### Dependencies

* update camunda-platform-images (patch) ([#5901](https://github.com/camunda/camunda-platform-helm/issues/5901)) ([5ae870b](https://github.com/camunda/camunda-platform-helm/commit/5ae870b4c47c9688e503e08701111326f279540c))
* update camunda-platform-images to v8.7.28 (patch) ([#5898](https://github.com/camunda/camunda-platform-helm/issues/5898)) ([9aec1cc](https://github.com/camunda/camunda-platform-helm/commit/9aec1ccae6fdeeeabe446c397b4a2b7561c2fd70))
* update camunda/identity docker tag to v8.7.18 ([#5891](https://github.com/camunda/camunda-platform-helm/issues/5891)) ([3f2877d](https://github.com/camunda/camunda-platform-helm/commit/3f2877d3dcc4701eb49d979376b35783f2ae18fc))
* update module github.com/moby/spdystream to v0.5.1 [security] ([#5842](https://github.com/camunda/camunda-platform-helm/issues/5842)) ([7f9533c](https://github.com/camunda/camunda-platform-helm/commit/7f9533c434223464ac1da6c34240c71fe6cea8e5))


### Refactors

* remove integration tests ([#5540](https://github.com/camunda/camunda-platform-helm/issues/5540)) ([8389340](https://github.com/camunda/camunda-platform-helm/commit/8389340f13038f60f80b77bc8e53b4b6a43374f2))

## [12.8.5](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.7-12.8.4...camunda-platform-8.7-12.8.5) (2026-04-15)


### Dependencies

* update camunda-platform-images (patch) ([#5739](https://github.com/camunda/camunda-platform-helm/issues/5739)) ([b9a99d7](https://github.com/camunda/camunda-platform-helm/commit/b9a99d7ca234f9e69c8354f715e3c0b08eb112c3))
* update camunda-platform-images to v8.7.27 (patch) ([#5789](https://github.com/camunda/camunda-platform-helm/issues/5789)) ([3d185c5](https://github.com/camunda/camunda-platform-helm/commit/3d185c500ad1197e3092cc25a84becb01d56cb4c))
* update camunda/optimize docker tag to v8.7.20 ([#5769](https://github.com/camunda/camunda-platform-helm/issues/5769)) ([ac313e3](https://github.com/camunda/camunda-platform-helm/commit/ac313e3652d4c6e0c87cb20fa7ff9c5404780a20))
* update module github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs to v1.65.0 [security] ([#5720](https://github.com/camunda/camunda-platform-helm/issues/5720)) ([c53bc9b](https://github.com/camunda/camunda-platform-helm/commit/c53bc9bda53b668ae1bfaca577ec1d7914d9fa6f))
* update module github.com/aws/aws-sdk-go-v2/service/lambda to v1.88.5 [security] ([#5721](https://github.com/camunda/camunda-platform-helm/issues/5721)) ([b9dd70c](https://github.com/camunda/camunda-platform-helm/commit/b9dd70c346e78f5f23493435026757a4a6c57274))
* update module github.com/aws/aws-sdk-go-v2/service/s3 to v1.97.3 [security] ([#5722](https://github.com/camunda/camunda-platform-helm/issues/5722)) ([87d69be](https://github.com/camunda/camunda-platform-helm/commit/87d69be31469874a55bf5c0b92760aa7aa167abc))
* update patch-updates (patch) ([#5758](https://github.com/camunda/camunda-platform-helm/issues/5758)) ([64ec2a1](https://github.com/camunda/camunda-platform-helm/commit/64ec2a1f0c44c88a2f19d5432655cc30d27ff632))

## [12.8.4](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.7-12.8.3...camunda-platform-8.7-12.8.4) (2026-04-02)


### Bug Fixes

* reference bitnami subcharts internally rather than relying on external repositories ([#5565](https://github.com/camunda/camunda-platform-helm/issues/5565)) ([f59f837](https://github.com/camunda/camunda-platform-helm/commit/f59f837d1b901909cde938b281873c12fe947a1c))


### Dependencies

* update camunda-platform-images (patch) ([#5556](https://github.com/camunda/camunda-platform-helm/issues/5556)) ([4e81945](https://github.com/camunda/camunda-platform-helm/commit/4e81945a5c1b1c88db533993e9c7c65f5fd0fa7b))
* update camunda-platform-images (patch) ([#5592](https://github.com/camunda/camunda-platform-helm/issues/5592)) ([a83b976](https://github.com/camunda/camunda-platform-helm/commit/a83b97661385f0381cd882948ca0ddb65da26de1))
* update camunda-platform-images (patch) ([#5603](https://github.com/camunda/camunda-platform-helm/issues/5603)) ([39fb7e2](https://github.com/camunda/camunda-platform-helm/commit/39fb7e2ae9d3a8cd9c66fa5bc5df674eedc74cfe))
* update camunda-platform-images (patch) ([#5608](https://github.com/camunda/camunda-platform-helm/issues/5608)) ([c9774ff](https://github.com/camunda/camunda-platform-helm/commit/c9774ffe2e35f119880cb2e767b37fdda8902930))
* update camunda-platform-images (patch) ([#5634](https://github.com/camunda/camunda-platform-helm/issues/5634)) ([d53ed59](https://github.com/camunda/camunda-platform-helm/commit/d53ed59531b98d08cca5002023607e9947a3993f))
* update camunda-platform-images (patch) ([#5642](https://github.com/camunda/camunda-platform-helm/issues/5642)) ([f12eb64](https://github.com/camunda/camunda-platform-helm/commit/f12eb64a7431e3e476b3827aa05bb405b7eaf84c))
* update patch-updates (patch) ([#5518](https://github.com/camunda/camunda-platform-helm/issues/5518)) ([520fe5b](https://github.com/camunda/camunda-platform-helm/commit/520fe5b5b3d2cfc1e9ae807a989f1f4edda956aa))

## [12.8.3](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.7-12.8.2...camunda-platform-8.7-12.8.3) (2026-03-16)


### Dependencies

* update camunda-platform-images (patch) ([#5449](https://github.com/camunda/camunda-platform-helm/issues/5449)) ([2d7ca81](https://github.com/camunda/camunda-platform-helm/commit/2d7ca81db5ba0f296c395d882015e9633a81b9f2))

## [12.8.2](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.7-12.8.1...camunda-platform-8.7-12.8.2) (2026-03-09)


### Dependencies

* update camunda-platform-images (patch) ([#5336](https://github.com/camunda/camunda-platform-helm/issues/5336)) ([d4c7bd1](https://github.com/camunda/camunda-platform-helm/commit/d4c7bd1f718ae2a8353b1f9b80fb29b1e9bfc1ac))

## [12.8.1](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.7-12.8.0...camunda-platform-8.7-12.8.1) (2026-03-06)


### Bug Fixes

* **documentStore:** allow IRSA AWS usage ([#5026](https://github.com/camunda/camunda-platform-helm/issues/5026)) ([b625076](https://github.com/camunda/camunda-platform-helm/commit/b6250760e1a41f4b477bb1cca408064153482b54))
* **openshift:** when es is disabled, fix templating error of label ([#5020](https://github.com/camunda/camunda-platform-helm/issues/5020)) ([50552d7](https://github.com/camunda/camunda-platform-helm/commit/50552d7ed4f97b9706989a9c89e2956aa5d8fac5))


### Dependencies

* update camunda-platform-images (patch) ([#5225](https://github.com/camunda/camunda-platform-helm/issues/5225)) ([b72cf31](https://github.com/camunda/camunda-platform-helm/commit/b72cf31da48f181f4dfe8b7248dfb37e97ee1263))
* update camunda-platform-images (patch) ([#5250](https://github.com/camunda/camunda-platform-helm/issues/5250)) ([d4c3c12](https://github.com/camunda/camunda-platform-helm/commit/d4c3c12a55123638377b94aa2f9b30966dfde4a5))
* update camunda-platform-images (patch) ([#5255](https://github.com/camunda/camunda-platform-helm/issues/5255)) ([4e0e5b7](https://github.com/camunda/camunda-platform-helm/commit/4e0e5b7b9ee99c2d8254693284bb6bc2475eb4dd))
* update camunda-platform-images (patch) ([#5265](https://github.com/camunda/camunda-platform-helm/issues/5265)) ([8dafa9f](https://github.com/camunda/camunda-platform-helm/commit/8dafa9f315d8fcd6e75e4268f9d0c15b70fd5e0b))
* update camunda-platform-images (patch) ([#5284](https://github.com/camunda/camunda-platform-helm/issues/5284)) ([d46a9c8](https://github.com/camunda/camunda-platform-helm/commit/d46a9c8293afa477fe5fd491212684bb3cf79e62))
* update minor-updates (minor) ([#5190](https://github.com/camunda/camunda-platform-helm/issues/5190)) ([23f46cc](https://github.com/camunda/camunda-platform-helm/commit/23f46cce8eb7a2c6d43b7b4dd1d90871183b8a59))
* update module filippo.io/edwards25519 to v1.1.1 [security] ([#5166](https://github.com/camunda/camunda-platform-helm/issues/5166)) ([09f8c4e](https://github.com/camunda/camunda-platform-helm/commit/09f8c4e42beae75abe4ecd00218eb210c0a7498b))
* update patch-updates (patch) ([#5183](https://github.com/camunda/camunda-platform-helm/issues/5183)) ([eef71ff](https://github.com/camunda/camunda-platform-helm/commit/eef71ffec59813cb48930eff516249043d603b79))

## [12.8.0](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.7-12.7.6...camunda-platform-8.7-12.8.0) (2026-02-26)


### Features

* expose options in values.yaml for helm v4 support ([#4918](https://github.com/camunda/camunda-platform-helm/issues/4918)) ([ec0fb7f](https://github.com/camunda/camunda-platform-helm/commit/ec0fb7f62af76b07b5fb970099781ddb4901ef68))


### Dependencies

* update camunda-platform-digests ([#5071](https://github.com/camunda/camunda-platform-helm/issues/5071)) ([5a64ccb](https://github.com/camunda/camunda-platform-helm/commit/5a64ccb2059f8d77ea8b14d37a3c40ab0c7dd6fe))
* update camunda-platform-images (patch) ([#5125](https://github.com/camunda/camunda-platform-helm/issues/5125)) ([131d2b5](https://github.com/camunda/camunda-platform-helm/commit/131d2b5efc2189a593f466eddee7f15f9400994b))
* update camunda-platform-images (patch) ([#5152](https://github.com/camunda/camunda-platform-helm/issues/5152)) ([bcc995a](https://github.com/camunda/camunda-platform-helm/commit/bcc995afe87e1af02b58e39f856dcfd1d5ca91a6))
* update camunda-platform-images (patch) ([#5182](https://github.com/camunda/camunda-platform-helm/issues/5182)) ([3ba8e07](https://github.com/camunda/camunda-platform-helm/commit/3ba8e07b58a5e1ca9239081bf3ba0e2d6a5a85e3))
* update minor-updates (minor) ([#5031](https://github.com/camunda/camunda-platform-helm/issues/5031)) ([8febe72](https://github.com/camunda/camunda-platform-helm/commit/8febe72311516c68444470bd08c9c59fff1db08f))
* update patch-updates (patch) ([#5033](https://github.com/camunda/camunda-platform-helm/issues/5033)) ([246572c](https://github.com/camunda/camunda-platform-helm/commit/246572c06b3508731446b0402aabb8d12b29f512))

## [12.7.6](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.7-12.7.5...camunda-platform-8.7-12.7.6) (2026-02-04)


### Dependencies

* update camunda-platform-images (patch) ([#5027](https://github.com/camunda/camunda-platform-helm/issues/5027)) ([7ed7062](https://github.com/camunda/camunda-platform-helm/commit/7ed70626fc58c627c70ceb65d6e2db9baa6a0d3c))
* update camunda-platform-images (patch) ([#5053](https://github.com/camunda/camunda-platform-helm/issues/5053)) ([586ee9b](https://github.com/camunda/camunda-platform-helm/commit/586ee9b0ccb8414f9b57d474bb440c528719a2f0))
* update camunda-platform-images (patch) ([#5062](https://github.com/camunda/camunda-platform-helm/issues/5062)) ([3c81c8e](https://github.com/camunda/camunda-platform-helm/commit/3c81c8ee602ce924d0446e12ab03efe6440738f7))

## [12.7.5](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.7-12.7.4...camunda-platform-8.7-12.7.5) (2026-01-08)


### Documentation

* update readme dependency section ([#4960](https://github.com/camunda/camunda-platform-helm/issues/4960)) ([3ddfb86](https://github.com/camunda/camunda-platform-helm/commit/3ddfb860ff8c4355a3ef2c0f2a5f71195f929e40))


### Dependencies

* update camunda-platform-images (patch) ([#4885](https://github.com/camunda/camunda-platform-helm/issues/4885)) ([4ffcd1d](https://github.com/camunda/camunda-platform-helm/commit/4ffcd1dbde8b44b82def6dcb320330c5197e1cd1))
* update camunda-platform-images (patch) ([#4923](https://github.com/camunda/camunda-platform-helm/issues/4923)) ([94829aa](https://github.com/camunda/camunda-platform-helm/commit/94829aaba5c970f84d0c6ccd01cec67a37d463e9))
* update camunda-platform-images (patch) ([#4946](https://github.com/camunda/camunda-platform-helm/issues/4946)) ([bceb9d1](https://github.com/camunda/camunda-platform-helm/commit/bceb9d13dee52708b4a625ee31e5f282d868fd99))
* update camunda-platform-images (patch) ([#4964](https://github.com/camunda/camunda-platform-helm/issues/4964)) ([9abc71b](https://github.com/camunda/camunda-platform-helm/commit/9abc71bf7b0d88bf340059fb66ebff3fe05d9120))
* update minor-updates (minor) ([#4929](https://github.com/camunda/camunda-platform-helm/issues/4929)) ([6a63cdc](https://github.com/camunda/camunda-platform-helm/commit/6a63cdc23cdc6d17b7cec3aa8ea55c40eae7d372))
* update patch-updates (patch) ([#4924](https://github.com/camunda/camunda-platform-helm/issues/4924)) ([8814e76](https://github.com/camunda/camunda-platform-helm/commit/8814e76c6fa71cc4db57051db959b4cec20ef9a1))

## [12.7.4](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.7-12.7.3...camunda-platform-8.7-12.7.4) (2025-12-10)


### Bug Fixes

* apply tpl to issuerBackendUrl ([#4858](https://github.com/camunda/camunda-platform-helm/issues/4858)) ([22b5cd7](https://github.com/camunda/camunda-platform-helm/commit/22b5cd74e7a3e952b17f752541c8233c5cd0f185))


### Dependencies

* update camunda-platform-images (patch) ([#4874](https://github.com/camunda/camunda-platform-helm/issues/4874)) ([3099888](https://github.com/camunda/camunda-platform-helm/commit/30998888f89795451f6e8e861b41e50c41707804))
* update patch-updates (patch) ([#4860](https://github.com/camunda/camunda-platform-helm/issues/4860)) ([b059be6](https://github.com/camunda/camunda-platform-helm/commit/b059be61080ee33c8d8ee9cfa5f0f4d2f4cdaf35))


### Refactors

* remove unused identity redirect-url ([#4853](https://github.com/camunda/camunda-platform-helm/issues/4853)) ([90c61e6](https://github.com/camunda/camunda-platform-helm/commit/90c61e66d4676b4ccadee71e6a593ab69df7f6d9))

## [12.7.3](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.7-12.7.2...camunda-platform-8.7-12.7.3) (2025-12-03)


### Bug Fixes

* add back zeebe secret referenced as string ([#4822](https://github.com/camunda/camunda-platform-helm/issues/4822)) ([9eaf035](https://github.com/camunda/camunda-platform-helm/commit/9eaf035cc70b918a6dad54452438ef230efa8b37))


### Dependencies

* update camunda-platform-images (patch) ([#4830](https://github.com/camunda/camunda-platform-helm/issues/4830)) ([02793c0](https://github.com/camunda/camunda-platform-helm/commit/02793c0cea5cd70ae1e327510a00230fdbaa3ef1))
* update patch-updates (patch) ([#4831](https://github.com/camunda/camunda-platform-helm/issues/4831)) ([c77bbe5](https://github.com/camunda/camunda-platform-helm/commit/c77bbe52c428f1a22597a76c19c0b26a40d6a8b7))

## [12.7.2](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.7-12.7.1...camunda-platform-8.7-12.7.2) (2025-11-28)


### Dependencies

* update camunda-platform-images (patch) ([#4792](https://github.com/camunda/camunda-platform-helm/issues/4792)) ([fd7294c](https://github.com/camunda/camunda-platform-helm/commit/fd7294c95d621b4d7d1c1d290b703d6209e61b44))
* update patch-updates ([#4761](https://github.com/camunda/camunda-platform-helm/issues/4761)) ([89f5551](https://github.com/camunda/camunda-platform-helm/commit/89f55518ddeaeec8fb0423afd173cd39e631ea95))

## [12.7.1](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.7-12.7.0...camunda-platform-8.7-12.7.1) (2025-11-25)


### Dependencies

* update camunda-platform-images (patch) ([#4763](https://github.com/camunda/camunda-platform-helm/issues/4763)) ([3fb6957](https://github.com/camunda/camunda-platform-helm/commit/3fb6957238e9daa4e7c6acdc9357dc6adeb2c0a9))
* update camunda-platform-images (patch) ([#4777](https://github.com/camunda/camunda-platform-helm/issues/4777)) ([18de26f](https://github.com/camunda/camunda-platform-helm/commit/18de26fe264049929c2edddb8e9aa04f3c213b94))
* update minor-updates (minor) ([#4712](https://github.com/camunda/camunda-platform-helm/issues/4712)) ([4cf435c](https://github.com/camunda/camunda-platform-helm/commit/4cf435c5aa989eaab1b0dde9cbc75fb694774854))
* update minor-updates (minor) ([#4765](https://github.com/camunda/camunda-platform-helm/issues/4765)) ([54dc74d](https://github.com/camunda/camunda-platform-helm/commit/54dc74d5fed86702a26a63f247d7ccc25424946a))
* update module golang.org/x/crypto to v0.45.0 [security] ([#4745](https://github.com/camunda/camunda-platform-helm/issues/4745)) ([1b31ade](https://github.com/camunda/camunda-platform-helm/commit/1b31aded5d1e7297e9648ad2e225b86f716a3b3e))
* update patch-updates (patch) ([#4762](https://github.com/camunda/camunda-platform-helm/issues/4762)) ([f8e7bbd](https://github.com/camunda/camunda-platform-helm/commit/f8e7bbd242097bb2c7c7bfde54aa53b3a5077af2))

## [12.7.0](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.7-12.6.4...camunda-platform-8.7-12.7.0) (2025-11-20)


### Features

* backport custom client creation to 8.6 and 8.9 ([#4710](https://github.com/camunda/camunda-platform-helm/issues/4710)) ([68bec54](https://github.com/camunda/camunda-platform-helm/commit/68bec54d8f2e7147c2f75ff20c3314533ce0c3a7))
* define custom clients for management identity ([#4653](https://github.com/camunda/camunda-platform-helm/issues/4653)) ([b488a0b](https://github.com/camunda/camunda-platform-helm/commit/b488a0bfd44c3bf6558edcd96c15cdd2f3eb4b5f))
* define custom users through values.yaml ([#4670](https://github.com/camunda/camunda-platform-helm/issues/4670)) ([19ab9eb](https://github.com/camunda/camunda-platform-helm/commit/19ab9eb7e42fe84b76118a1930dd72bb6d302cdf))


### Bug Fixes

* incorrect example for keycloak in readme.md ([#4586](https://github.com/camunda/camunda-platform-helm/issues/4586)) ([f6bf0a9](https://github.com/camunda/camunda-platform-helm/commit/f6bf0a9c125178b2cd3b15d465dc7ed0a59893b8))
* remove client env vars from qa scenario files ([#4726](https://github.com/camunda/camunda-platform-helm/issues/4726)) ([2c9ea12](https://github.com/camunda/camunda-platform-helm/commit/2c9ea121df9f402b19330e61dddbdd28ffbd4d35))
* typo lower case values ([#4737](https://github.com/camunda/camunda-platform-helm/issues/4737)) ([2ec2710](https://github.com/camunda/camunda-platform-helm/commit/2ec2710830d669e53a709bbb176c58ba064e12f2))
* zeebe gateway has a context path ([#4690](https://github.com/camunda/camunda-platform-helm/issues/4690)) ([bdf4a61](https://github.com/camunda/camunda-platform-helm/commit/bdf4a618f885f374c07b6269cae623d87f93d57e))


### Dependencies

* update camunda-platform-images (patch) ([#4732](https://github.com/camunda/camunda-platform-helm/issues/4732)) ([3445429](https://github.com/camunda/camunda-platform-helm/commit/3445429910e81e4077f8702535ee73659e35bff4))
