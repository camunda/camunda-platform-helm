# Changelog

## 0.0.0-8.7.0-alpha2 (2024-12-10)


### ⚠ BREAKING CHANGES

* remove separated ingress functionality and only keep the global combined ingress
* Zeebe, Zeebe Gateway, Operate, and Tasklist have been replaced with Orchestration Core. Also, the Connectors configuration syntax was updated.

### Features

* add core unified prefix for Elasticsearch/OpenSearch ([#2643](https://github.com/camunda/camunda-platform-helm/issues/2643)) ([68a5e5b](https://github.com/camunda/camunda-platform-helm/commit/68a5e5bff96c32c7a54977ebad52913a377b35e6))
* add support for authorizations configuration ([#2593](https://github.com/camunda/camunda-platform-helm/issues/2593)) ([ac4410e](https://github.com/camunda/camunda-platform-helm/commit/ac4410e6e02a61f6ef49a2def904a13f5fcf37c6))
* adding application override to console ([#2594](https://github.com/camunda/camunda-platform-helm/issues/2594)) ([e9d0c48](https://github.com/camunda/camunda-platform-helm/commit/e9d0c4827d5e10a666abacce5d00d94b443418aa))
* adding TLS support to console ([#2505](https://github.com/camunda/camunda-platform-helm/issues/2505)) ([c32f5d4](https://github.com/camunda/camunda-platform-helm/commit/c32f5d4a911d0484001219df7b9a05f836c1b69f))


### Bug Fixes

* **8.7:** wrong roles for identity user ([#2639](https://github.com/camunda/camunda-platform-helm/issues/2639)) ([13fc9dc](https://github.com/camunda/camunda-platform-helm/commit/13fc9dc4d1d6a5658dc852cbe197694544d5ad48))
* add default webModler url when ingress is disabled ([#2566](https://github.com/camunda/camunda-platform-helm/issues/2566)) ([678da17](https://github.com/camunda/camunda-platform-helm/commit/678da176b47323e3c63247e0b805a4d44f3979ed))
* add missing components when identity disabled. ([#2627](https://github.com/camunda/camunda-platform-helm/issues/2627)) ([7cf7f98](https://github.com/camunda/camunda-platform-helm/commit/7cf7f98665ebd906f803b49c0ae00d5af74c8b34))
* adding http/https options to readinessProbes for console ([#2529](https://github.com/camunda/camunda-platform-helm/issues/2529)) ([64a37f6](https://github.com/camunda/camunda-platform-helm/commit/64a37f66227ceb32b67c4f58b729206f6a5c5392))
* **alpha:** set the zeebe prefix correctly ([ae9c825](https://github.com/camunda/camunda-platform-helm/commit/ae9c82512781c1c307eb20a96589d0b7575aa3e0))
* assign Zeebe role to demo user ([#2510](https://github.com/camunda/camunda-platform-helm/issues/2510)) ([cd419a3](https://github.com/camunda/camunda-platform-helm/commit/cd419a3da7d3e1859bdbbf742bda554b4fd42eaa))
* **core:** disable core client secret if identity is disabled ([#2584](https://github.com/camunda/camunda-platform-helm/issues/2584)) ([89a1333](https://github.com/camunda/camunda-platform-helm/commit/89a13330b2f71cfe30e3932c7e738d22b9d9711b))
* **core:** Identity authorization request not found ([#2641](https://github.com/camunda/camunda-platform-helm/issues/2641)) ([8d65dea](https://github.com/camunda/camunda-platform-helm/commit/8d65dea804d53bd2acf325e47958f232833857b3))
* **core:** small bug fixes and set correct values ([#2629](https://github.com/camunda/camunda-platform-helm/issues/2629)) ([e53ceaf](https://github.com/camunda/camunda-platform-helm/commit/e53ceafcd1d1fc25a324c619dfaad1157d94500e))
* **deps:** update module github.com/gruntwork-io/terratest to v0.48.0 ([#2665](https://github.com/camunda/camunda-platform-helm/issues/2665)) ([8027e66](https://github.com/camunda/camunda-platform-helm/commit/8027e66d9a4e27a53b2fe1e42ad0e385d0bc6bdd))
* **deps:** update module github.com/stretchr/testify to v1.10.0 ([#2609](https://github.com/camunda/camunda-platform-helm/issues/2609)) ([90097de](https://github.com/camunda/camunda-platform-helm/commit/90097dea2a6bfa678d405f2aa9ee6165c2cb57c3))
* **openshift:** allow usage of the route with the default router ([#2646](https://github.com/camunda/camunda-platform-helm/issues/2646)) ([0b37e0f](https://github.com/camunda/camunda-platform-helm/commit/0b37e0fdd4c9de40f19a5ee1893668c54e8574e2))


### Refactors

* **alpha:** update connectors config for 8.7 ([#2657](https://github.com/camunda/camunda-platform-helm/issues/2657)) ([bdef34b](https://github.com/camunda/camunda-platform-helm/commit/bdef34bb6b65eb6baa6d87fcd18fdbd7f5699b07))
* **core:** add identity admin client ([#2602](https://github.com/camunda/camunda-platform-helm/issues/2602)) ([9acebbe](https://github.com/camunda/camunda-platform-helm/commit/9acebbeb81642664f0dc8b44df30fb009ca72890))
* **core:** rename core statefulset to avoid upgrade downtime ([#2581](https://github.com/camunda/camunda-platform-helm/issues/2581)) ([061b06d](https://github.com/camunda/camunda-platform-helm/commit/061b06d35936bf8995f03a1ea4bec276ecb6a94f))
* remove separated ingress functionality ([#2586](https://github.com/camunda/camunda-platform-helm/issues/2586)) ([3d12988](https://github.com/camunda/camunda-platform-helm/commit/3d12988720594bb6cc160bf246999cba89fecdea))
* remove support for global.multiregion.installationType ([#2588](https://github.com/camunda/camunda-platform-helm/issues/2588)) ([a04f88a](https://github.com/camunda/camunda-platform-helm/commit/a04f88a4073a130f715094d6bca9d5d4b4c419b0))
* replace zeebe and web-apps with camunda orchestration core ([28d7927](https://github.com/camunda/camunda-platform-helm/commit/28d79278105b365a61b51974ce5efb0400d160e0))
* unify authorization configuration ([#2640](https://github.com/camunda/camunda-platform-helm/issues/2640)) ([3449db1](https://github.com/camunda/camunda-platform-helm/commit/3449db1c207bafd450d1d8f83f661816096d3718))
