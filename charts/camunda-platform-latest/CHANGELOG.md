# Changelog

## [10.3.0](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-latest-v10.2.1...camunda-platform-latest-10.3.0) (2024-08-10)


### Features

* support migration init container for operate ([#2144](https://github.com/camunda/camunda-platform-helm/issues/2144)) ([0262935](https://github.com/camunda/camunda-platform-helm/commit/026293570c021f97cf73d25047e63083d9f1392f))


### Bug Fixes

* add constraint for contextPath and rest path to be the same for zeebe gateway ([#2166](https://github.com/camunda/camunda-platform-helm/issues/2166)) ([e878815](https://github.com/camunda/camunda-platform-helm/commit/e878815ba43202802b07bdb9f2a807220ebc9ef9))
* add volumeClaimTemplates label selector in zeebe statefulSet ([cba241a](https://github.com/camunda/camunda-platform-helm/commit/cba241ac62d3ceee6e350cd27e3dc1f29f5c10b8))
* changed restAddress in Tasklist with helper function ([#2152](https://github.com/camunda/camunda-platform-helm/issues/2152)) ([9e45221](https://github.com/camunda/camunda-platform-helm/commit/9e45221a4fae96d51d105fb8a6e84fe589c3153c))
* drop namespace from zeebe advertisedHost and initialContactPoints ([#2170](https://github.com/camunda/camunda-platform-helm/issues/2170)) ([564581d](https://github.com/camunda/camunda-platform-helm/commit/564581d1f136f055a72975f362812b567937ccfc))
* existingSecret for OpenSearch password can be used without defining password literal ([#2168](https://github.com/camunda/camunda-platform-helm/issues/2168)) ([71e4e4b](https://github.com/camunda/camunda-platform-helm/commit/71e4e4b7af7cef7e4d4e5348a94f1bcc09b0eea9))
* template grpc url in console config ([#2165](https://github.com/camunda/camunda-platform-helm/issues/2165)) ([2e83c49](https://github.com/camunda/camunda-platform-helm/commit/2e83c491c31947ef65891458bb8b884ba5895a8d))
* use correct operate image for version label ([#2183](https://github.com/camunda/camunda-platform-helm/issues/2183)) ([3ed7fd2](https://github.com/camunda/camunda-platform-helm/commit/3ed7fd2e75f3cd1330af7b56e401c462081165de))
* **web-modeler:** add websockets health endpoint ([165ce26](https://github.com/camunda/camunda-platform-helm/commit/165ce260fd6ecf04bebc94ab2291767fa4a7015b))


### Refactors

* hardcoding strategy for all components and removing ability to configure strategy ([#2155](https://github.com/camunda/camunda-platform-helm/issues/2155)) ([127162a](https://github.com/camunda/camunda-platform-helm/commit/127162a83a13b85c60247926d638a037a1077a36))
* make web-modeler restapi database password secret optional ([#1761](https://github.com/camunda/camunda-platform-helm/issues/1761)) ([d482861](https://github.com/camunda/camunda-platform-helm/commit/d4828619728186747fbc4b7825f2315e40422620))
