# Changelog

## [12.1.0](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.7-12.0.2...camunda-platform-8.7-12.1.0) (2025-06-10)


### Features

* core test suite inital migration to playwright ([#3521](https://github.com/camunda/camunda-platform-helm/issues/3521)) ([62ff130](https://github.com/camunda/camunda-platform-helm/commit/62ff1300ebb17e468ba6d36d9701c0d5c1e5038e))
* support image.digest for all components ([#3446](https://github.com/camunda/camunda-platform-helm/issues/3446)) ([d4e3510](https://github.com/camunda/camunda-platform-helm/commit/d4e351038059e032e6cdf8ffffbee7fc34634cfa))
* support ingress external hostname ([#3547](https://github.com/camunda/camunda-platform-helm/issues/3547)) ([2d23e04](https://github.com/camunda/camunda-platform-helm/commit/2d23e0497f74b6030359933fcff2ec06cdacdf28))


### Bug Fixes

* add main env to migration init container ([#3586](https://github.com/camunda/camunda-platform-helm/issues/3586)) ([60f0122](https://github.com/camunda/camunda-platform-helm/commit/60f012241ff24efdb0b983c5b7b84180d9627a9b))
* remove installationType for 8.7 ([#3529](https://github.com/camunda/camunda-platform-helm/issues/3529)) ([e665027](https://github.com/camunda/camunda-platform-helm/commit/e66502766e13e7a15e10332b1bfc685fd0b7dae3))
* restore the original Zeebe startup script behaviour for backup operations ([#3589](https://github.com/camunda/camunda-platform-helm/issues/3589)) ([35dfe00](https://github.com/camunda/camunda-platform-helm/commit/35dfe0066bc7cc39e9e3a2466f2451135312fa75))


### Dependencies

* update asdf-vm/actions action to v4 ([#3552](https://github.com/camunda/camunda-platform-helm/issues/3552)) ([c5bd7bb](https://github.com/camunda/camunda-platform-helm/commit/c5bd7bbe4c7ea934a974856b9b381b99660e6c31))
* update camunda-platform-8.7 (patch) ([#3537](https://github.com/camunda/camunda-platform-helm/issues/3537)) ([ba5bba4](https://github.com/camunda/camunda-platform-helm/commit/ba5bba40bdce5f7ea7cb73461accd46b134c1a6b))
* update camunda-platform-8.7 (patch) ([#3561](https://github.com/camunda/camunda-platform-helm/issues/3561)) ([0b43895](https://github.com/camunda/camunda-platform-helm/commit/0b43895dfb592cf0115183b3b8fce82e37c8dd00))
* update camunda-platform-8.7 (patch) ([#3593](https://github.com/camunda/camunda-platform-helm/issues/3593)) ([d4835cf](https://github.com/camunda/camunda-platform-helm/commit/d4835cf217390830ba5740e5de4f478c40987024))
* update camunda-platform-8.7 (patch) ([#3601](https://github.com/camunda/camunda-platform-helm/issues/3601)) ([9172e35](https://github.com/camunda/camunda-platform-helm/commit/9172e35bf3730bdf2da727d5592b0f43da5a69c6))
* update camunda-platform-8.7 (patch) ([#3617](https://github.com/camunda/camunda-platform-helm/issues/3617)) ([2dac5a4](https://github.com/camunda/camunda-platform-helm/commit/2dac5a491e699f48ca0d65028449c347783f2ba6))
* update camunda-platform-8.7 to v8.7.3 (patch) ([#3528](https://github.com/camunda/camunda-platform-helm/issues/3528)) ([6c0f848](https://github.com/camunda/camunda-platform-helm/commit/6c0f848d18e51260cd280aa73ea8052424055402))
* update camunda/console docker tag to v8.7.18 ([#3484](https://github.com/camunda/camunda-platform-helm/issues/3484)) ([4a004d6](https://github.com/camunda/camunda-platform-helm/commit/4a004d620b688cfab82a21eb347a84f591fe108c))
* update camunda/console docker tag to v8.7.19 ([#3496](https://github.com/camunda/camunda-platform-helm/issues/3496)) ([bd8026c](https://github.com/camunda/camunda-platform-helm/commit/bd8026c4a3431b3bcae601c9686a18820cf85312))
* update camunda/console docker tag to v8.7.21 ([#3502](https://github.com/camunda/camunda-platform-helm/issues/3502)) ([51d25cd](https://github.com/camunda/camunda-platform-helm/commit/51d25cd7279616a753efb5b3019bf85d43b148f0))
* update camunda/console docker tag to v8.7.22 ([#3508](https://github.com/camunda/camunda-platform-helm/issues/3508)) ([f3bf7f0](https://github.com/camunda/camunda-platform-helm/commit/f3bf7f0b5ad025438a875fd9326f2ab61950e315))
* update camunda/console docker tag to v8.7.25 ([#3557](https://github.com/camunda/camunda-platform-helm/issues/3557)) ([c04c946](https://github.com/camunda/camunda-platform-helm/commit/c04c946d9256356a0388b1bc328f3d25f9f9e5d3))
* update camunda/web-modeler-restapi docker tag to v8.7.2 ([#3534](https://github.com/camunda/camunda-platform-helm/issues/3534)) ([54b2790](https://github.com/camunda/camunda-platform-helm/commit/54b2790bdc6e069f5e0425e1a2ad5994d819d3a5))
* update dependency go to v1.24.3 ([#3479](https://github.com/camunda/camunda-platform-helm/issues/3479)) ([69b0516](https://github.com/camunda/camunda-platform-helm/commit/69b05161d60e771e666c2c685ce556c3392e7e23))
* update keycloak docker tag to v24.6.7 ([#3486](https://github.com/camunda/camunda-platform-helm/issues/3486)) ([70ad13a](https://github.com/camunda/camunda-platform-helm/commit/70ad13af43fac990a7aaa7f063d7f9d259dd1fbe))
* update keycloak docker tag to v24.7.0 ([#3523](https://github.com/camunda/camunda-platform-helm/issues/3523)) ([ea08442](https://github.com/camunda/camunda-platform-helm/commit/ea084421192bd40c0d0dfb516835b4521e24f14a))
* update keycloak docker tag to v24.7.3 ([#3568](https://github.com/camunda/camunda-platform-helm/issues/3568)) ([8d157c5](https://github.com/camunda/camunda-platform-helm/commit/8d157c5feca4664277edb3dc31645facc7b67fd3))
* update module github.com/fatih/color to v1.18.0 ([#3480](https://github.com/camunda/camunda-platform-helm/issues/3480)) ([193d53b](https://github.com/camunda/camunda-platform-helm/commit/193d53b9a19a00eb5090aa17691c8667bd11be3b))
* update module github.com/gruntwork-io/terratest to v0.49.0 ([#3466](https://github.com/camunda/camunda-platform-helm/issues/3466)) ([6110203](https://github.com/camunda/camunda-platform-helm/commit/61102039e8cbe6825d2134b7bd58480cd3cb6912))
* update module github.com/spf13/cobra to v1.9.1 ([#3481](https://github.com/camunda/camunda-platform-helm/issues/3481)) ([07a378b](https://github.com/camunda/camunda-platform-helm/commit/07a378bd305acb951ac8073847e85e6bdc3edf49))
