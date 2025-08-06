# Changelog

## [10.10.0](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.5-10.9.0...camunda-platform-8.5-10.10.0) (2025-08-06)


### Features

* enable values to be source from this repo rather than push from the QA repo ([#3885](https://github.com/camunda/camunda-platform-helm/issues/3885)) ([5bcae1a](https://github.com/camunda/camunda-platform-helm/commit/5bcae1a788a86f7e11b1cf23922e1cf55c4450fe))
* introduce e2e tests for helm charts ([#3730](https://github.com/camunda/camunda-platform-helm/issues/3730)) ([e99089e](https://github.com/camunda/camunda-platform-helm/commit/e99089e96592e4e4e5d6d95c141701ed02979000))


### Bug Fixes

* add digest to web-modeler components ([#3873](https://github.com/camunda/camunda-platform-helm/issues/3873)) ([aa86501](https://github.com/camunda/camunda-platform-helm/commit/aa86501c72aee6367940f51f8eb5767955b0901b))
* sysctl images were not updated with the bitnamilegacy repo name ([#3900](https://github.com/camunda/camunda-platform-helm/issues/3900)) ([cd3573e](https://github.com/camunda/camunda-platform-helm/commit/cd3573ec0db17daf60ce17f3b9249234af4934f0))


### Dependencies

* update camunda-platform-8.5 (patch) ([#3882](https://github.com/camunda/camunda-platform-helm/issues/3882)) ([f719abb](https://github.com/camunda/camunda-platform-helm/commit/f719abb534e35f0823a7ac80492a5ab744e960bb))
* update camunda-platform-8.5 to v8.5.118 (patch) ([#3836](https://github.com/camunda/camunda-platform-helm/issues/3836)) ([ad67166](https://github.com/camunda/camunda-platform-helm/commit/ad67166d206365baf9634ea692f438e810caaf5a))
* update camunda-platform-8.5 to v8.5.19 (patch) ([#3875](https://github.com/camunda/camunda-platform-helm/issues/3875)) ([bcdd74d](https://github.com/camunda/camunda-platform-helm/commit/bcdd74d5e3946e4c843f0fd13b1bc8743e2d3b9b))
* update camunda/operate docker tag to v8.5.18 ([#3893](https://github.com/camunda/camunda-platform-helm/issues/3893)) ([fcff68a](https://github.com/camunda/camunda-platform-helm/commit/fcff68a41fc258cca5e085b41fc0c2f28593e6a0))
* update camunda/tasklist docker tag to v8.5.20 ([#3896](https://github.com/camunda/camunda-platform-helm/issues/3896)) ([04579c4](https://github.com/camunda/camunda-platform-helm/commit/04579c4696ab2d7c48f1364296a28a865554b7d2))
* update camunda/web-modeler docker tag to v8.5.21 ([#3868](https://github.com/camunda/camunda-platform-helm/issues/3868)) ([09bb523](https://github.com/camunda/camunda-platform-helm/commit/09bb52397ddb6b09584be30d463448483be9076f))
* update camunda/zeebe docker tag to v8.5.21 ([#3878](https://github.com/camunda/camunda-platform-helm/issues/3878)) ([52bc1e1](https://github.com/camunda/camunda-platform-helm/commit/52bc1e181ba1d91462fcdcc734ec4600296dac47))
* update dependency go to v1.24.5 ([#3624](https://github.com/camunda/camunda-platform-helm/issues/3624)) ([f83452a](https://github.com/camunda/camunda-platform-helm/commit/f83452ae727fbd8e1492f6875af24468a044cfba))


### Refactors

* change bitnami docker images to bitnamilegacy ([#3890](https://github.com/camunda/camunda-platform-helm/issues/3890)) ([f96858c](https://github.com/camunda/camunda-platform-helm/commit/f96858c3b8a2fc3340892ba82db1111cec35344d))
