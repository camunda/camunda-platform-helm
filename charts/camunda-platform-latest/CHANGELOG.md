# Changelog

## [10.3.0](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-10.2.0...camunda-platform-10.3.0) (2024-07-03)


### Features

* add console auth vars ([#1782](https://github.com/camunda/camunda-platform-helm/issues/1782)) ([1bd65ca](https://github.com/camunda/camunda-platform-helm/commit/1bd65ca58c56a821710532ed8fc6e68d97d492ca))
* configurable update strategy ([#2036](https://github.com/camunda/camunda-platform-helm/issues/2036)) ([70f5232](https://github.com/camunda/camunda-platform-helm/commit/70f523223e5c39676471d3beb166083c1b0ad185))
* support dnsPolicy and dnsConfig for all components ([#2009](https://github.com/camunda/camunda-platform-helm/issues/2009)) ([31b7c4f](https://github.com/camunda/camunda-platform-helm/commit/31b7c4fee88361e441f820f9104c2192a1261965))


### Bug Fixes

* **deps:** update module github.com/gruntwork-io/terratest to v0.46.16 ([#2088](https://github.com/camunda/camunda-platform-helm/issues/2088)) ([8fe27b5](https://github.com/camunda/camunda-platform-helm/commit/8fe27b55966a4577e5f72c720bd85aac5bd63d63))
* identity base url not configured ([#2028](https://github.com/camunda/camunda-platform-helm/issues/2028)) ([d3d0012](https://github.com/camunda/camunda-platform-helm/commit/d3d001232b42dc8f94e139dc4a5fe29b32fae9aa))
* unauthenticated external elasticsearch no longer forces password… ([#1990](https://github.com/camunda/camunda-platform-helm/issues/1990)) ([fc79bdb](https://github.com/camunda/camunda-platform-helm/commit/fc79bdbc70475e19a22d9f7b24e11c036cea6be8))


### Refactors

* remove the global image tag value and use it from the components ([#2069](https://github.com/camunda/camunda-platform-helm/issues/2069)) ([3a672ea](https://github.com/camunda/camunda-platform-helm/commit/3a672eaabd6154baa88aa1f70777b850dfe5c9b9))
