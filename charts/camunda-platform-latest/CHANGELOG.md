# Changelog

## [10.3.0](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-10.2.0...camunda-platform-10.3.0) (2024-07-25)


### Features

* add console auth vars ([#1782](https://github.com/camunda/camunda-platform-helm/issues/1782)) ([81da51b](https://github.com/camunda/camunda-platform-helm/commit/81da51b4dc22e3419c5e210c626ff2a52edd4328))
* configurable update strategy ([#2036](https://github.com/camunda/camunda-platform-helm/issues/2036)) ([675ce34](https://github.com/camunda/camunda-platform-helm/commit/675ce341395987f42707592a2e00b4e47c749b6d))
* support dnsPolicy and dnsConfig for all components ([#2009](https://github.com/camunda/camunda-platform-helm/issues/2009)) ([6e3045c](https://github.com/camunda/camunda-platform-helm/commit/6e3045c6247af3d356564541dcae980eec5d7419))


### Bug Fixes

* added recreate strategy to all Operate deployments ([#2143](https://github.com/camunda/camunda-platform-helm/issues/2143)) ([c2d70dc](https://github.com/camunda/camunda-platform-helm/commit/c2d70dc36088e67c5acb6fc5e51cc1fc64cf9e33))
* **deps:** update module github.com/gruntwork-io/terratest to v0.46.16 ([#2088](https://github.com/camunda/camunda-platform-helm/issues/2088)) ([33d5b61](https://github.com/camunda/camunda-platform-helm/commit/33d5b61e27fb4a6e3e30506fb557c65626995130))
* **deps:** update module github.com/gruntwork-io/terratest to v0.47.0 ([#2122](https://github.com/camunda/camunda-platform-helm/issues/2122)) ([b5fe511](https://github.com/camunda/camunda-platform-helm/commit/b5fe5117cf7323456e2e1797863b85f45cb09e14))
* identity base url not configured ([#2028](https://github.com/camunda/camunda-platform-helm/issues/2028)) ([890d202](https://github.com/camunda/camunda-platform-helm/commit/890d2028e14ed79c9a0f14b1ac7845379a3eb301))
* unauthenticated external elasticsearch no longer forces password… ([#1990](https://github.com/camunda/camunda-platform-helm/issues/1990)) ([485ecb7](https://github.com/camunda/camunda-platform-helm/commit/485ecb7e575aa6c702e119d6ced97a0f9246e2b1))


### Refactors

* remove the global image tag value and use it from the components ([#2069](https://github.com/camunda/camunda-platform-helm/issues/2069)) ([0c34cd5](https://github.com/camunda/camunda-platform-helm/commit/0c34cd56d12fe257e0feca3fcf52fca3ea4c3fb5))
