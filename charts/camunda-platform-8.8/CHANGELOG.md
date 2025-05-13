# Changelog

## [13.0.0-alpha4](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.8-13.0.0-alpha3...camunda-platform-8.8-13.0.0-alpha4) (2025-05-13)


### Features

* add basic auth for web-modeler ([#3396](https://github.com/camunda/camunda-platform-helm/issues/3396)) ([985c04c](https://github.com/camunda/camunda-platform-helm/commit/985c04c946d46442baae13074f5ac204fa27decb))


### Bug Fixes

* add apiVersion and kind to PersistentVolumeClaims ([#2321](https://github.com/camunda/camunda-platform-helm/issues/2321)) ([b7c6092](https://github.com/camunda/camunda-platform-helm/commit/b7c6092654001387834c40e15b8adfffa7896b50))
* add elasticsearch golden files ([#3334](https://github.com/camunda/camunda-platform-helm/issues/3334)) ([1faf57e](https://github.com/camunda/camunda-platform-helm/commit/1faf57e08452710cfaeca42165498c680c407278))
* **alpha:** correct the keycloak proto function to get the svc port ([#3382](https://github.com/camunda/camunda-platform-helm/issues/3382)) ([76ba509](https://github.com/camunda/camunda-platform-helm/commit/76ba5091d8b60b45c59cce7773114727af1c8237))
* changed name of zeebegateway in console configmap ([#2660](https://github.com/camunda/camunda-platform-helm/issues/2660)) ([21e99c0](https://github.com/camunda/camunda-platform-helm/commit/21e99c025204d0846399946a7715a67a1a48e507))
* elasticsearch.extraConfig is a table ([#3353](https://github.com/camunda/camunda-platform-helm/issues/3353)) ([2db5f0b](https://github.com/camunda/camunda-platform-helm/commit/2db5f0b802a4cc5068ea0e553fec58933fbf7b38))
* ingress port not rendering when identityKeycloak enabled ([#3461](https://github.com/camunda/camunda-platform-helm/issues/3461)) ([0d92490](https://github.com/camunda/camunda-platform-helm/commit/0d924907a9d8efd559a020e303ca5e183d77476e))
* refactor 8 8 unit test camunda ([#3368](https://github.com/camunda/camunda-platform-helm/issues/3368)) ([7f66172](https://github.com/camunda/camunda-platform-helm/commit/7f66172247a57e690194cde1ad04af1347f29882))
* refactor 8 8 unit test connectors ([#3369](https://github.com/camunda/camunda-platform-helm/issues/3369)) ([1be555e](https://github.com/camunda/camunda-platform-helm/commit/1be555e9f5118086d2a75f936c156bab43386992))
* refactor 8 8 unit test console ([#3367](https://github.com/camunda/camunda-platform-helm/issues/3367)) ([62ab365](https://github.com/camunda/camunda-platform-helm/commit/62ab365c13fdb0b66f81b08002ad6b2996f901b6))
* refactor 8 8 unit test core ([#3371](https://github.com/camunda/camunda-platform-helm/issues/3371)) ([f790b8b](https://github.com/camunda/camunda-platform-helm/commit/f790b8bb57d315069070fc8c5162afa7f6f8c329))
* refactor 8 8 unit test identity ([#3373](https://github.com/camunda/camunda-platform-helm/issues/3373)) ([d2246d7](https://github.com/camunda/camunda-platform-helm/commit/d2246d7efb150d99e6fa7e955f7bac3a18f66643))
* refactor 8 8 unit test optimize ([#3370](https://github.com/camunda/camunda-platform-helm/issues/3370)) ([95e39be](https://github.com/camunda/camunda-platform-helm/commit/95e39be63b06b81fda37f362958d251ecfc1e437))
* refactor 8 8 unit test web modeler ([#3372](https://github.com/camunda/camunda-platform-helm/issues/3372)) ([0fd0dd2](https://github.com/camunda/camunda-platform-helm/commit/0fd0dd2ffebae1a35173bf3a5306116da0ebe768))
* skip elasticsearch.extraConfig ([#3397](https://github.com/camunda/camunda-platform-helm/issues/3397)) ([f25b1f0](https://github.com/camunda/camunda-platform-helm/commit/f25b1f068c1d5168659544739c3e959b4138ffc3))
* update existingSecret params for 8.6 8.7 and 8.8 ([#3299](https://github.com/camunda/camunda-platform-helm/issues/3299)) ([057f855](https://github.com/camunda/camunda-platform-helm/commit/057f855936311fc1a90fc261aca3179f9172163c))


### Dependencies

* update bitnami/postgresql docker tag to v14.17.0-debian-12-r15 ([#3315](https://github.com/camunda/camunda-platform-helm/issues/3315)) ([8544f88](https://github.com/camunda/camunda-platform-helm/commit/8544f886c2046ce02425794832dba7b566225c6d))
* update bitnami/postgresql docker tag to v14.17.0-debian-12-r16 ([#3401](https://github.com/camunda/camunda-platform-helm/issues/3401)) ([0dd3eea](https://github.com/camunda/camunda-platform-helm/commit/0dd3eea95072832280098d06c650e43ef9aaab6c))
* update bitnami/postgresql docker tag to v14.17.0-debian-12-r17 ([#3403](https://github.com/camunda/camunda-platform-helm/issues/3403)) ([4c0463f](https://github.com/camunda/camunda-platform-helm/commit/4c0463f0c7cef51c2ae88270e50e34079d128b97))
* update camunda-platform-8.8 (minor) ([#3306](https://github.com/camunda/camunda-platform-helm/issues/3306)) ([a821721](https://github.com/camunda/camunda-platform-helm/commit/a821721403526a5337e36d033aacba8b716e2913))
* update camunda-platform-8.8 (patch) ([#3433](https://github.com/camunda/camunda-platform-helm/issues/3433)) ([7448cfd](https://github.com/camunda/camunda-platform-helm/commit/7448cfd22716bda1885b6085140e42fc211681ee))
* update camunda-platform-8.8 to v8.8.0-alpha4 (patch) ([#3467](https://github.com/camunda/camunda-platform-helm/issues/3467)) ([568bdf0](https://github.com/camunda/camunda-platform-helm/commit/568bdf0bcb22255f19da4f816fcc89bb565e4f1b))
* update camunda/connectors-bundle docker tag to v8.8.0-alpha4 ([#3444](https://github.com/camunda/camunda-platform-helm/issues/3444)) ([2511be9](https://github.com/camunda/camunda-platform-helm/commit/2511be9e3bdcc11be0072299674585121d93450c))
* update camunda/web-modeler-restapi docker tag to v8.8.0-alpha4 ([#3421](https://github.com/camunda/camunda-platform-helm/issues/3421)) ([16934dd](https://github.com/camunda/camunda-platform-helm/commit/16934ddcbececb804d7d4d27df5f993f6dfee07b))
* update module gopkg.in/yaml.v2 to v3 ([#3398](https://github.com/camunda/camunda-platform-helm/issues/3398)) ([4e8231c](https://github.com/camunda/camunda-platform-helm/commit/4e8231c4faacae58570136cf64bd58e3449944fe))


### Refactors

* update core configMap with retention parameters ([#3308](https://github.com/camunda/camunda-platform-helm/issues/3308)) ([83f44df](https://github.com/camunda/camunda-platform-helm/commit/83f44dfb8ed1bb71e635ec92d00c4a7418ee0444))
