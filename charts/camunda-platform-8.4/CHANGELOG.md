# Changelog

## [9.5.0](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.4-v9.4.6...camunda-platform-8.4-9.5.0) (2025-01-23)


### Features

* support optional chart secrets auto-generation ([#2290](https://github.com/camunda/camunda-platform-helm/issues/2290)) ([5d8156c](https://github.com/camunda/camunda-platform-helm/commit/5d8156c3574d8e097ae182a2672d4b2764b51744))


### Bug Fixes

* added contextpath to 8.4 identity ([#2310](https://github.com/camunda/camunda-platform-helm/issues/2310)) ([a821813](https://github.com/camunda/camunda-platform-helm/commit/a821813f91090f31aa9411cc796ef88820fecb68))
* correct ingress nginx annotation to activate proxy buffering by default ([#2304](https://github.com/camunda/camunda-platform-helm/issues/2304)) ([1e260e9](https://github.com/camunda/camunda-platform-helm/commit/1e260e9db34c349420237251156575f235d077f2))
* **deps:** update module github.com/stretchr/testify to v1.10.0 ([#2608](https://github.com/camunda/camunda-platform-helm/issues/2608)) ([b55d386](https://github.com/camunda/camunda-platform-helm/commit/b55d386d0009a86312a58dd69332c8b54874a1cf))
* disable console in 8.4 ([#2309](https://github.com/camunda/camunda-platform-helm/issues/2309)) ([15158d8](https://github.com/camunda/camunda-platform-helm/commit/15158d850832d79a1554cc2f1d1933e5c05b5f88))
* empty commit for releasable unit (release-please) ([023dca3](https://github.com/camunda/camunda-platform-helm/commit/023dca334710faf63a57da8aec970379a446f3a6))
* empty commit for releasable unit (release-please) ([#2766](https://github.com/camunda/camunda-platform-helm/issues/2766)) ([7c81e3d](https://github.com/camunda/camunda-platform-helm/commit/7c81e3db92a47be163a8bb7a4efe26cdfab10551))
* enable secrets deprecation flag in alpha by default ([#2081](https://github.com/camunda/camunda-platform-helm/issues/2081)) ([b791f4c](https://github.com/camunda/camunda-platform-helm/commit/b791f4cd6ac3859112b07a89fa6bc89a46d08313))
* **follow-up:** correct existingSecretKey for connectors inbound auth ([712ea6a](https://github.com/camunda/camunda-platform-helm/commit/712ea6a6b387f063e67238321b8a59134d4b2d16))
* identity firstuser existingsecretkey has no effect ([#2370](https://github.com/camunda/camunda-platform-helm/issues/2370)) ([0aecce9](https://github.com/camunda/camunda-platform-helm/commit/0aecce930c3b5ea0ba8ef225ee117b5c6b393352))
* identity secret should be referenced ([#2487](https://github.com/camunda/camunda-platform-helm/issues/2487)) ([f1d9262](https://github.com/camunda/camunda-platform-helm/commit/f1d92628f0bf5c2dae75d048bc94713f0d9f076a))
* renovate disable elasticsearch minor upgrades and revert elasticsearch upgrade ([#2666](https://github.com/camunda/camunda-platform-helm/issues/2666)) ([8ce8485](https://github.com/camunda/camunda-platform-helm/commit/8ce848551d375f56fccdc41b99e4f4bf0f8cf3b5))
* set optimize global elasticsearch prefix ([#2491](https://github.com/camunda/camunda-platform-helm/issues/2491)) ([2805de0](https://github.com/camunda/camunda-platform-helm/commit/2805de0a10dfff30f511b8c7a96d9d9da2e1e941))
* update camunda-platform-8.3 ([a6164af](https://github.com/camunda/camunda-platform-helm/commit/a6164af3e69b4bb046bf8c1fadeee526f7255df1))
* update camunda-platform-8.4 ([1c93674](https://github.com/camunda/camunda-platform-helm/commit/1c936740de03e81efe8da4507859cd0823939db9))
* update postgresql image tag to avoid bitnami broken release ([#2556](https://github.com/camunda/camunda-platform-helm/issues/2556)) ([d985bf2](https://github.com/camunda/camunda-platform-helm/commit/d985bf24092265feeddde859aa55d3e9f5199a00))


### Documentation

* update of outdated url in the local kubernetes  section ([#2274](https://github.com/camunda/camunda-platform-helm/issues/2274)) ([83f8230](https://github.com/camunda/camunda-platform-helm/commit/83f8230d8f5b34d52294e6d3d1be449ffe6aee9c))


### Refactors

* default keycloak ingress pathType to Prefix ([#2372](https://github.com/camunda/camunda-platform-helm/issues/2372)) ([377c18f](https://github.com/camunda/camunda-platform-helm/commit/377c18fc9e0316c6ee0d43b89759c8ffdaa58540))
* parametrized hard-coded identity auth vars ([#2512](https://github.com/camunda/camunda-platform-helm/issues/2512)) ([8f5801b](https://github.com/camunda/camunda-platform-helm/commit/8f5801b866c348c4045ec76341e0de233c27a4d1))
* using bitnami oci chart repository ([#2356](https://github.com/camunda/camunda-platform-helm/issues/2356)) ([18fa53e](https://github.com/camunda/camunda-platform-helm/commit/18fa53e914c4acca314014dada47b057c69cb2db))
