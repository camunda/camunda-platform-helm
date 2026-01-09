# Changelog

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
