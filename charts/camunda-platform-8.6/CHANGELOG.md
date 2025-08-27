# Changelog

## [11.9.0](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.6-11.8.0...camunda-platform-8.6-11.9.0) (2025-08-27)


### Features

* backport - implement joinPath helper for constructing paths ([#3934](https://github.com/camunda/camunda-platform-helm/issues/3934)) ([15052c2](https://github.com/camunda/camunda-platform-helm/commit/15052c2d96a6ae7f2774fdef51f5f0a1a97b080b))
* create extra values file for legacy bitnami repo name ([#3996](https://github.com/camunda/camunda-platform-helm/issues/3996)) ([d1a48ea](https://github.com/camunda/camunda-platform-helm/commit/d1a48ea76fbb4fdae12dc685b842c3925e54ec6f))


### Bug Fixes

* env var CAMUNDA_DATABASE_PASSWORD wasn't set correctly with opensearch ([#3931](https://github.com/camunda/camunda-platform-helm/issues/3931)) ([626a5fa](https://github.com/camunda/camunda-platform-helm/commit/626a5fa6fa9cf2dc7cac0d6f20eda76042cdbd81))


### Dependencies

* update camunda-platform-8.6 to v8.6.25 (patch) ([#3993](https://github.com/camunda/camunda-platform-helm/issues/3993)) ([1426dcf](https://github.com/camunda/camunda-platform-helm/commit/1426dcf3e6152c0f858e9a489e981e8c38171a05))
* update camunda/identity docker tag to v8.6.19 ([#3989](https://github.com/camunda/camunda-platform-helm/issues/3989)) ([1ecaa47](https://github.com/camunda/camunda-platform-helm/commit/1ecaa476b29c91c31a2c5db01f270facea506320))
* update camunda/optimize docker tag to v8.6.15 ([#4000](https://github.com/camunda/camunda-platform-helm/issues/4000)) ([f2f79b9](https://github.com/camunda/camunda-platform-helm/commit/f2f79b978d65a492eba4d377ff033dbf15806f19))
* update dependency go to v1.25.0 ([#3936](https://github.com/camunda/camunda-platform-helm/issues/3936)) ([42a7c73](https://github.com/camunda/camunda-platform-helm/commit/42a7c7357cfbca23760c9ba3c4977f776a7f6282))
* update module github.com/stretchr/testify to v1.11.0 ([#3986](https://github.com/camunda/camunda-platform-helm/issues/3986)) ([5af5659](https://github.com/camunda/camunda-platform-helm/commit/5af565966743f543149b225e68fa55c4f5ee3084))
* update registry.camunda.cloud/vendor-ee/postgresql docker tag to v14.19.0 ([#3947](https://github.com/camunda/camunda-platform-helm/issues/3947)) ([7537b24](https://github.com/camunda/camunda-platform-helm/commit/7537b242cd80e09ebfd08d0b01f5a3471eaf08f4))
* update registry.camunda.cloud/vendor-ee/postgresql docker tag to v15.13.0 ([#3776](https://github.com/camunda/camunda-platform-helm/issues/3776)) ([be096f8](https://github.com/camunda/camunda-platform-helm/commit/be096f89b20900cd376baa1b8b465242ce0b4a42))
* update registry.camunda.cloud/vendor-ee/postgresql docker tag to v15.14.0 ([#3969](https://github.com/camunda/camunda-platform-helm/issues/3969)) ([f587663](https://github.com/camunda/camunda-platform-helm/commit/f58766398a368777a69ae1f33c398f41dfc580fd))
