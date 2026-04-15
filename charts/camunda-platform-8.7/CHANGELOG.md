# Changelog

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
