# Changelog

## [8.2.30](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.2.29...camunda-platform-8.2.30) (2024-07-13)


### Bug Fixes

* **deps:** update module github.com/gruntwork-io/terratest to v0.46.15 ([#1965](https://github.com/camunda/camunda-platform-helm/issues/1965)) ([5487142](https://github.com/camunda/camunda-platform-helm/commit/548714296ae6ade07b7585111f8973d221e80983))
* **deps:** update module github.com/gruntwork-io/terratest to v0.46.16 ([#2088](https://github.com/camunda/camunda-platform-helm/issues/2088)) ([33d5b61](https://github.com/camunda/camunda-platform-helm/commit/33d5b61e27fb4a6e3e30506fb557c65626995130))
* **deps:** update module github.com/stretchr/testify to v1.9.0 ([#1948](https://github.com/camunda/camunda-platform-helm/issues/1948)) ([11afba6](https://github.com/camunda/camunda-platform-helm/commit/11afba60edf6de35429174b381b0d06964e8b6de))
* **deps:** update module k8s.io/api to v0.27.15 ([#1962](https://github.com/camunda/camunda-platform-helm/issues/1962)) ([e68d48b](https://github.com/camunda/camunda-platform-helm/commit/e68d48b7af48f6fbaf2aff0c1e8714c1659f4479))
* **openshift:** make post-render script compatible with mac ([#1970](https://github.com/camunda/camunda-platform-helm/issues/1970)) ([5a43425](https://github.com/camunda/camunda-platform-helm/commit/5a43425b2b59c674de4495b7e2ae13209156d29b))


### Refactors

* remove the global image tag value and use it from the components - 8.2, 8.3, and 8.4 ([#2080](https://github.com/camunda/camunda-platform-helm/issues/2080)) ([30a3724](https://github.com/camunda/camunda-platform-helm/commit/30a3724c62c9c97b54eb9f78dea2a95b0953d3bb))
* update zeebe gateway readiness probe endpoint ([a28f661](https://github.com/camunda/camunda-platform-helm/commit/a28f6616d0c3f0268709aceb8406ee9fe651d722))
