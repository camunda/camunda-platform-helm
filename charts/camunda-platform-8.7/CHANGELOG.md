# Changelog

## [12.5.0](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.7-12.4.0...camunda-platform-8.7-12.5.0) (2025-08-26)


### Features

* backport - implement joinPath helper for constructing paths ([#3934](https://github.com/camunda/camunda-platform-helm/issues/3934)) ([15052c2](https://github.com/camunda/camunda-platform-helm/commit/15052c2d96a6ae7f2774fdef51f5f0a1a97b080b))
* create extra values file for legacy bitnami repo name ([#3996](https://github.com/camunda/camunda-platform-helm/issues/3996)) ([d1a48ea](https://github.com/camunda/camunda-platform-helm/commit/d1a48ea76fbb4fdae12dc685b842c3925e54ec6f))


### Bug Fixes

* env var CAMUNDA_DATABASE_PASSWORD wasn't set correctly with opensearch ([#3931](https://github.com/camunda/camunda-platform-helm/issues/3931)) ([626a5fa](https://github.com/camunda/camunda-platform-helm/commit/626a5fa6fa9cf2dc7cac0d6f20eda76042cdbd81))
* the region is mapped to the document store env ([#3623](https://github.com/camunda/camunda-platform-helm/issues/3623)) ([d224c44](https://github.com/camunda/camunda-platform-helm/commit/d224c44d384f093b0f878340f4eb4611990b0170))


### Dependencies

* update bitnamilegacy/keycloak docker tag to v26.3.3 ([#3976](https://github.com/camunda/camunda-platform-helm/issues/3976)) ([4e52467](https://github.com/camunda/camunda-platform-helm/commit/4e5246714aa1536daa43df1fa5b841c20a056832))
* update camunda-platform-8.7 to v8.7.11 (patch) ([#3998](https://github.com/camunda/camunda-platform-helm/issues/3998)) ([469a9af](https://github.com/camunda/camunda-platform-helm/commit/469a9af7fed16b86ad501f685bc0c51b6c3d5b5c))
* update camunda/identity docker tag to v8.7.6 ([#3990](https://github.com/camunda/camunda-platform-helm/issues/3990)) ([7a4876d](https://github.com/camunda/camunda-platform-helm/commit/7a4876d4ac6baaee5dce3bbf8217979f974b61bb))
* update camunda/optimize docker tag to v8.7.8 ([#4001](https://github.com/camunda/camunda-platform-helm/issues/4001)) ([271429a](https://github.com/camunda/camunda-platform-helm/commit/271429a9da3a93945b51874e1bfae8bbc4daf769))
* update dependency go to v1.25.0 ([#3936](https://github.com/camunda/camunda-platform-helm/issues/3936)) ([42a7c73](https://github.com/camunda/camunda-platform-helm/commit/42a7c7357cfbca23760c9ba3c4977f776a7f6282))
* update module github.com/stretchr/testify to v1.11.0 ([#3986](https://github.com/camunda/camunda-platform-helm/issues/3986)) ([5af5659](https://github.com/camunda/camunda-platform-helm/commit/5af565966743f543149b225e68fa55c4f5ee3084))
* update registry.camunda.cloud/vendor-ee/postgresql docker tag to v14.19.0 ([#3947](https://github.com/camunda/camunda-platform-helm/issues/3947)) ([7537b24](https://github.com/camunda/camunda-platform-helm/commit/7537b242cd80e09ebfd08d0b01f5a3471eaf08f4))
* update registry.camunda.cloud/vendor-ee/postgresql docker tag to v15.13.0 ([#3776](https://github.com/camunda/camunda-platform-helm/issues/3776)) ([be096f8](https://github.com/camunda/camunda-platform-helm/commit/be096f89b20900cd376baa1b8b465242ce0b4a42))
* update registry.camunda.cloud/vendor-ee/postgresql docker tag to v15.14.0 ([#3969](https://github.com/camunda/camunda-platform-helm/issues/3969)) ([f587663](https://github.com/camunda/camunda-platform-helm/commit/f58766398a368777a69ae1f33c398f41dfc580fd))
