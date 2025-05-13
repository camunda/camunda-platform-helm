# Changelog

## [11.4.0](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.6-11.3.1...camunda-platform-8.6-11.4.0) (2025-05-13)


### Features

* dedicated opensearch.prefix support ([#3379](https://github.com/camunda/camunda-platform-helm/issues/3379)) ([672886b](https://github.com/camunda/camunda-platform-helm/commit/672886bf8e308f56a0455f97fc62433d94aa6b1b))


### Bug Fixes

* add apiVersion and kind to PersistentVolumeClaims ([#2321](https://github.com/camunda/camunda-platform-helm/issues/2321)) ([b7c6092](https://github.com/camunda/camunda-platform-helm/commit/b7c6092654001387834c40e15b8adfffa7896b50))
* add elasticsearch golden files ([#3334](https://github.com/camunda/camunda-platform-helm/issues/3334)) ([1faf57e](https://github.com/camunda/camunda-platform-helm/commit/1faf57e08452710cfaeca42165498c680c407278))
* add gateway enabled ([#3307](https://github.com/camunda/camunda-platform-helm/issues/3307)) ([112dcb4](https://github.com/camunda/camunda-platform-helm/commit/112dcb4512a6060f649bae74fc6898c2e10daeb9))
* grpc port of zeebe gateway for venom tests ([#3364](https://github.com/camunda/camunda-platform-helm/issues/3364)) ([9228eb5](https://github.com/camunda/camunda-platform-helm/commit/9228eb542eaebb263c8fef88087291a4d82daf93))
* openshift elasticsearch disabled error ([#3472](https://github.com/camunda/camunda-platform-helm/issues/3472)) ([1f7d2f3](https://github.com/camunda/camunda-platform-helm/commit/1f7d2f38b6f149c7ef5d8d1b37c5ba5e6256f998))
* refactor of zeebe unit tests in 8.6 ([#3320](https://github.com/camunda/camunda-platform-helm/issues/3320)) ([83207e0](https://github.com/camunda/camunda-platform-helm/commit/83207e00ad9f7b08d2544a4d906e1692e881bbb2))
* refactor unit tests in webmodeller of 8.6 to new style ([#3319](https://github.com/camunda/camunda-platform-helm/issues/3319)) ([8b5a1d0](https://github.com/camunda/camunda-platform-helm/commit/8b5a1d03d846dce09cbc4c09954f59fb0e0a3ed6))
* refactor unit tests in zeebe-gateway to new style on 8.6 ([#3321](https://github.com/camunda/camunda-platform-helm/issues/3321)) ([972806e](https://github.com/camunda/camunda-platform-helm/commit/972806e85238da13d2485b61736c22447094a315))
* refactored optimize unit tests to the new style for 8.6 ([#3300](https://github.com/camunda/camunda-platform-helm/issues/3300)) ([619c20e](https://github.com/camunda/camunda-platform-helm/commit/619c20ef74a7f935fd65a5220f73cff6fb100180))
* revert "feat: dedicated opensearch.prefix support" ([#3482](https://github.com/camunda/camunda-platform-helm/issues/3482)) ([bcfe32a](https://github.com/camunda/camunda-platform-helm/commit/bcfe32a1b1e9fbe9ca516a70831080bf3f7bb7d0))
* unit tests in tasklist on 8.6 have been refactored to the new style ([#3318](https://github.com/camunda/camunda-platform-helm/issues/3318)) ([83f03bd](https://github.com/camunda/camunda-platform-helm/commit/83f03bd4e055c3e777f334e66569c5ea54845dc5))
* update existingSecret params for 8.6 8.7 and 8.8 ([#3299](https://github.com/camunda/camunda-platform-helm/issues/3299)) ([057f855](https://github.com/camunda/camunda-platform-helm/commit/057f855936311fc1a90fc261aca3179f9172163c))


### Dependencies

* update bitnami/postgresql docker tag to v14.18.0-debian-12-r0 ([#3464](https://github.com/camunda/camunda-platform-helm/issues/3464)) ([b21a6ba](https://github.com/camunda/camunda-platform-helm/commit/b21a6baab9ed62dfb122d78fa3950f8d63183dba))
* update camunda-platform-8.6 (patch) ([#3313](https://github.com/camunda/camunda-platform-helm/issues/3313)) ([704d59c](https://github.com/camunda/camunda-platform-helm/commit/704d59c6481e2e2aeba1c92415a344679445f154))
* update camunda-platform-8.6 (patch) ([#3470](https://github.com/camunda/camunda-platform-helm/issues/3470)) ([8d02ea1](https://github.com/camunda/camunda-platform-helm/commit/8d02ea12fca76cf56d84dc09a7833a61ad521bea))
* update elasticsearch docker tag to v21.6.2 ([#3390](https://github.com/camunda/camunda-platform-helm/issues/3390)) ([7499fd5](https://github.com/camunda/camunda-platform-helm/commit/7499fd5fc3769b065cde8ae6f7601e757967f248))
* update module gopkg.in/yaml.v2 to v3 ([#3398](https://github.com/camunda/camunda-platform-helm/issues/3398)) ([4e8231c](https://github.com/camunda/camunda-platform-helm/commit/4e8231c4faacae58570136cf64bd58e3449944fe))
