# Changelog

## [8.3.0](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.2-v8.2.34...camunda-platform-8.2-8.3.0) (2025-01-23)


### Features

* add optimize migration init container ([efcb96f](https://github.com/camunda/camunda-platform-helm/commit/efcb96f1a0ca39ca8aef4e5e3913b319ae8a137e))


### Bug Fixes

* correct ingress nginx annotation to activate proxy buffering by default ([#2304](https://github.com/camunda/camunda-platform-helm/issues/2304)) ([1e260e9](https://github.com/camunda/camunda-platform-helm/commit/1e260e9db34c349420237251156575f235d077f2))
* **deps:** update camunda-platform-8.2 (patch) ([#2396](https://github.com/camunda/camunda-platform-helm/issues/2396)) ([867d05b](https://github.com/camunda/camunda-platform-helm/commit/867d05b56b490df1e7159d0013888a011714b9df))
* **deps:** update module github.com/stretchr/testify to v1.10.0 ([#2608](https://github.com/camunda/camunda-platform-helm/issues/2608)) ([b55d386](https://github.com/camunda/camunda-platform-helm/commit/b55d386d0009a86312a58dd69332c8b54874a1cf))
* empty commit for releasable unit (release-please) ([023dca3](https://github.com/camunda/camunda-platform-helm/commit/023dca334710faf63a57da8aec970379a446f3a6))
* empty commit for releasable unit (release-please) ([#2766](https://github.com/camunda/camunda-platform-helm/issues/2766)) ([7c81e3d](https://github.com/camunda/camunda-platform-helm/commit/7c81e3db92a47be163a8bb7a4efe26cdfab10551))
* enable secrets deprecation flag in alpha by default ([#2081](https://github.com/camunda/camunda-platform-helm/issues/2081)) ([b791f4c](https://github.com/camunda/camunda-platform-helm/commit/b791f4cd6ac3859112b07a89fa6bc89a46d08313))
* update camunda-platform-8.3 ([a6164af](https://github.com/camunda/camunda-platform-helm/commit/a6164af3e69b4bb046bf8c1fadeee526f7255df1))


### Refactors

* default keycloak ingress pathType to Prefix ([#2372](https://github.com/camunda/camunda-platform-helm/issues/2372)) ([377c18f](https://github.com/camunda/camunda-platform-helm/commit/377c18fc9e0316c6ee0d43b89759c8ffdaa58540))
* disable ingest.geoip.downloader.enabled ([10b2697](https://github.com/camunda/camunda-platform-helm/commit/10b2697e0408856d7e48eb46d716324ccb075cd5))
* using bitnami oci chart repository ([#2356](https://github.com/camunda/camunda-platform-helm/issues/2356)) ([18fa53e](https://github.com/camunda/camunda-platform-helm/commit/18fa53e914c4acca314014dada47b057c69cb2db))
