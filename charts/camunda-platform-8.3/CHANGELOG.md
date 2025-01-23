# Changelog

## [8.3.24](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.3-v8.3.23...camunda-platform-8.3-8.3.24) (2025-01-23)


### Bug Fixes

* correct ingress nginx annotation to activate proxy buffering by default ([#2304](https://github.com/camunda/camunda-platform-helm/issues/2304)) ([1e260e9](https://github.com/camunda/camunda-platform-helm/commit/1e260e9db34c349420237251156575f235d077f2))
* **deps:** update module github.com/stretchr/testify to v1.10.0 ([#2608](https://github.com/camunda/camunda-platform-helm/issues/2608)) ([b55d386](https://github.com/camunda/camunda-platform-helm/commit/b55d386d0009a86312a58dd69332c8b54874a1cf))
* empty commit for releasable unit (release-please) ([023dca3](https://github.com/camunda/camunda-platform-helm/commit/023dca334710faf63a57da8aec970379a446f3a6))
* empty commit for releasable unit (release-please) ([#2766](https://github.com/camunda/camunda-platform-helm/issues/2766)) ([7c81e3d](https://github.com/camunda/camunda-platform-helm/commit/7c81e3db92a47be163a8bb7a4efe26cdfab10551))
* enable secrets deprecation flag in alpha by default ([#2081](https://github.com/camunda/camunda-platform-helm/issues/2081)) ([b791f4c](https://github.com/camunda/camunda-platform-helm/commit/b791f4cd6ac3859112b07a89fa6bc89a46d08313))
* renovate disable elasticsearch minor upgrades and revert elasticsearch upgrade ([#2666](https://github.com/camunda/camunda-platform-helm/issues/2666)) ([8ce8485](https://github.com/camunda/camunda-platform-helm/commit/8ce848551d375f56fccdc41b99e4f4bf0f8cf3b5))
* set optimize global elasticsearch prefix ([#2491](https://github.com/camunda/camunda-platform-helm/issues/2491)) ([2805de0](https://github.com/camunda/camunda-platform-helm/commit/2805de0a10dfff30f511b8c7a96d9d9da2e1e941))
* support optional chart secrets auto-generation - camunda 8.3 ([#2742](https://github.com/camunda/camunda-platform-helm/issues/2742)) ([653b31d](https://github.com/camunda/camunda-platform-helm/commit/653b31dd109393c33b749cf6a8e25f8f7e4e40e8))
* update camunda-platform-8.3 ([a6164af](https://github.com/camunda/camunda-platform-helm/commit/a6164af3e69b4bb046bf8c1fadeee526f7255df1))
* update postgresql image tag to avoid bitnami broken release ([#2556](https://github.com/camunda/camunda-platform-helm/issues/2556)) ([d985bf2](https://github.com/camunda/camunda-platform-helm/commit/d985bf24092265feeddde859aa55d3e9f5199a00))
* use integration-test-credentials for web modeler secret ([#2745](https://github.com/camunda/camunda-platform-helm/issues/2745)) ([b5e49ac](https://github.com/camunda/camunda-platform-helm/commit/b5e49ac530034729044bc7251d2fa35bf5ed4bb5))


### Documentation

* update of outdated url in the local kubernetes  section ([#2274](https://github.com/camunda/camunda-platform-helm/issues/2274)) ([83f8230](https://github.com/camunda/camunda-platform-helm/commit/83f8230d8f5b34d52294e6d3d1be449ffe6aee9c))


### Refactors

* default keycloak ingress pathType to Prefix ([#2372](https://github.com/camunda/camunda-platform-helm/issues/2372)) ([377c18f](https://github.com/camunda/camunda-platform-helm/commit/377c18fc9e0316c6ee0d43b89759c8ffdaa58540))
* using bitnami oci chart repository ([#2356](https://github.com/camunda/camunda-platform-helm/issues/2356)) ([18fa53e](https://github.com/camunda/camunda-platform-helm/commit/18fa53e914c4acca314014dada47b057c69cb2db))
