# Changelog

## [8.2.30](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.2.29...camunda-platform-8.2.30) (2024-07-03)


### Bug Fixes

* **deps:** update module github.com/gruntwork-io/terratest to v0.46.15 ([#1965](https://github.com/camunda/camunda-platform-helm/issues/1965)) ([61c7194](https://github.com/camunda/camunda-platform-helm/commit/61c71944417d0ef9c7e9ac9e1293188ba8bbc145))
* **deps:** update module github.com/gruntwork-io/terratest to v0.46.16 ([#2088](https://github.com/camunda/camunda-platform-helm/issues/2088)) ([8fe27b5](https://github.com/camunda/camunda-platform-helm/commit/8fe27b55966a4577e5f72c720bd85aac5bd63d63))
* **deps:** update module github.com/stretchr/testify to v1.9.0 ([#1948](https://github.com/camunda/camunda-platform-helm/issues/1948)) ([0e2f389](https://github.com/camunda/camunda-platform-helm/commit/0e2f3897ebff7e13722ee42dca801ca94d1de292))
* **deps:** update module k8s.io/api to v0.27.15 ([#1962](https://github.com/camunda/camunda-platform-helm/issues/1962)) ([d77ed70](https://github.com/camunda/camunda-platform-helm/commit/d77ed7071abacc3e2f18764f835184d88bc42237))
* **openshift:** make post-render script compatible with mac ([#1970](https://github.com/camunda/camunda-platform-helm/issues/1970)) ([c326f48](https://github.com/camunda/camunda-platform-helm/commit/c326f4892f3728fd3e9f8bfe72db9beb219f6b5e))


### Refactors

* remove the global image tag value and use it from the components - 8.2, 8.3, and 8.4 ([#2080](https://github.com/camunda/camunda-platform-helm/issues/2080)) ([3c1a207](https://github.com/camunda/camunda-platform-helm/commit/3c1a20702705dc5ae4029eddd4fbe06e66796937))
* update zeebe gateway readiness probe endpoint ([adc6abc](https://github.com/camunda/camunda-platform-helm/commit/adc6abcd3b4ecadb9c2c27d34612eb0022ce2471))
