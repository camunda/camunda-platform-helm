# Changelog

## [12.4.0](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.7-12.3.0...camunda-platform-8.7-12.4.0) (2025-08-08)


### Features

* enable values to be source from this repo rather than push from the QA repo ([#3885](https://github.com/camunda/camunda-platform-helm/issues/3885)) ([5bcae1a](https://github.com/camunda/camunda-platform-helm/commit/5bcae1a788a86f7e11b1cf23922e1cf55c4450fe))
* introduce e2e tests for helm charts ([#3730](https://github.com/camunda/camunda-platform-helm/issues/3730)) ([e99089e](https://github.com/camunda/camunda-platform-helm/commit/e99089e96592e4e4e5d6d95c141701ed02979000))


### Bug Fixes

* add digest to web-modeler components ([#3873](https://github.com/camunda/camunda-platform-helm/issues/3873)) ([aa86501](https://github.com/camunda/camunda-platform-helm/commit/aa86501c72aee6367940f51f8eb5767955b0901b))
* set zeebe freespace config to application defaults ([#3843](https://github.com/camunda/camunda-platform-helm/issues/3843)) ([f46f65d](https://github.com/camunda/camunda-platform-helm/commit/f46f65d57fd1704e8db166b0ec45695a8e309fc4))
* sysctl images were not updated with the bitnamilegacy repo name ([#3900](https://github.com/camunda/camunda-platform-helm/issues/3900)) ([cd3573e](https://github.com/camunda/camunda-platform-helm/commit/cd3573ec0db17daf60ce17f3b9249234af4934f0))


### Documentation

* zeebe affinity is documented faulty ([#3877](https://github.com/camunda/camunda-platform-helm/issues/3877)) ([088d70b](https://github.com/camunda/camunda-platform-helm/commit/088d70b499c78441d8d64409c56a4bdb1ab68d38))


### Dependencies

* update camunda-platform-8.7 (patch) ([#3786](https://github.com/camunda/camunda-platform-helm/issues/3786)) ([ce9ba97](https://github.com/camunda/camunda-platform-helm/commit/ce9ba977f9d635b8918947f75caf9b3243b81e81))
* update camunda-platform-8.7 (patch) ([#3804](https://github.com/camunda/camunda-platform-helm/issues/3804)) ([83c85bb](https://github.com/camunda/camunda-platform-helm/commit/83c85bb8fe27ecfc9708de028f044cd7f26071c7))
* update camunda-platform-8.7 (patch) ([#3839](https://github.com/camunda/camunda-platform-helm/issues/3839)) ([7ffefb5](https://github.com/camunda/camunda-platform-helm/commit/7ffefb555db879ab73549c6aa0a6ebbbf4df57a2))
* update camunda-platform-8.7 (patch) ([#3864](https://github.com/camunda/camunda-platform-helm/issues/3864)) ([2907d80](https://github.com/camunda/camunda-platform-helm/commit/2907d809e9d03bfea0a0bcb427e00a1b8a5569d6))
* update camunda/console docker tag to v8.7.44 ([#3802](https://github.com/camunda/camunda-platform-helm/issues/3802)) ([148bb1f](https://github.com/camunda/camunda-platform-helm/commit/148bb1f4800389289f805739a2386cc3e58ab71d))
* update camunda/console docker tag to v8.7.47 ([#3827](https://github.com/camunda/camunda-platform-helm/issues/3827)) ([f30ff48](https://github.com/camunda/camunda-platform-helm/commit/f30ff48a9fb8e62d175dc1368eb43c89d90ca494))
* update camunda/optimize docker tag to v8.7.7 ([#3917](https://github.com/camunda/camunda-platform-helm/issues/3917)) ([dc198f3](https://github.com/camunda/camunda-platform-helm/commit/dc198f30ab263f8995175810e56f1e5c2af16617))
* update dependency go to v1.24.5 ([#3624](https://github.com/camunda/camunda-platform-helm/issues/3624)) ([f83452a](https://github.com/camunda/camunda-platform-helm/commit/f83452ae727fbd8e1492f6875af24468a044cfba))
* update dependency go to v1.24.6 ([#3911](https://github.com/camunda/camunda-platform-helm/issues/3911)) ([0502e3a](https://github.com/camunda/camunda-platform-helm/commit/0502e3a15f14dba0a78d6c9e5029b5f1820ec68b))
* update keycloak docker tag to v24.7.7 ([#3796](https://github.com/camunda/camunda-platform-helm/issues/3796)) ([67d71e6](https://github.com/camunda/camunda-platform-helm/commit/67d71e6df30939d542365d61f2bd4b55ac195527))
* update keycloak docker tag to v24.8.0 ([#3828](https://github.com/camunda/camunda-platform-helm/issues/3828)) ([14d08bf](https://github.com/camunda/camunda-platform-helm/commit/14d08bf7cefab5b1db9ffa17453b74293c0757b6))
* update keycloak docker tag to v24.8.1 ([#3848](https://github.com/camunda/camunda-platform-helm/issues/3848)) ([8286288](https://github.com/camunda/camunda-platform-helm/commit/8286288473880fa527b1de616a8a339172073ef7))
* update keycloak docker tag to v24.9.0 ([#3892](https://github.com/camunda/camunda-platform-helm/issues/3892)) ([264e59a](https://github.com/camunda/camunda-platform-helm/commit/264e59a2a81a2fd4470052c222c04ecc2d4ae880))


### Refactors

* change bitnami docker images to bitnamilegacy ([#3890](https://github.com/camunda/camunda-platform-helm/issues/3890)) ([f96858c](https://github.com/camunda/camunda-platform-helm/commit/f96858c3b8a2fc3340892ba82db1111cec35344d))
