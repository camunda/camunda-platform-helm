# Changelog

## [12.0.0-alpha4](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-alpha-v12.0.0-alpha3...camunda-platform-alpha-12.0.0-alpha4) (2025-02-11)


### Features

* configure web-modeler restapi JWK Set URI ([#2704](https://github.com/camunda/camunda-platform-helm/issues/2704)) ([0be3045](https://github.com/camunda/camunda-platform-helm/commit/0be304587c72c25644f08e3520089065eff55a8a))
* **web-modeler:** add support for configuring Zeebe clusters [backport] ([#2853](https://github.com/camunda/camunda-platform-helm/issues/2853)) ([b769947](https://github.com/camunda/camunda-platform-helm/commit/b76994737808f5a6ce32cdbb76a63c8d073b3004))


### Bug Fixes

* added support for optimize env vars for migration initcontainer ([#2710](https://github.com/camunda/camunda-platform-helm/issues/2710)) ([8fc1265](https://github.com/camunda/camunda-platform-helm/commit/8fc1265feba2e9ab5b2d386b53e54e6e0cea47b5))
* apply password key from subchart ([#2799](https://github.com/camunda/camunda-platform-helm/issues/2799)) ([a708991](https://github.com/camunda/camunda-platform-helm/commit/a70899107aafc9360aee09c47316ba3f19ec1262))
* **connectors:** correct config for new 8.7 ([#2855](https://github.com/camunda/camunda-platform-helm/issues/2855)) ([92fb1f4](https://github.com/camunda/camunda-platform-helm/commit/92fb1f42db69f401bd3999471aadb1cabe1dcc67))


### Documentation

* remove executionIdentity as it's still non-stable ([#2846](https://github.com/camunda/camunda-platform-helm/issues/2846)) ([53dcb07](https://github.com/camunda/camunda-platform-helm/commit/53dcb07f1ad234e3feede53752b9aab24b5312f1))


### Refactors

* switch 8.7 to 8.6 chart structure ([#2790](https://github.com/camunda/camunda-platform-helm/issues/2790)) ([24e9c1a](https://github.com/camunda/camunda-platform-helm/commit/24e9c1a2d57025dcd08a14fb2a324c4af4cdcbac))
* update apps deps ([c554fc5](https://github.com/camunda/camunda-platform-helm/commit/c554fc5354c4807172f55a39d0d74a51bd9031b4))
