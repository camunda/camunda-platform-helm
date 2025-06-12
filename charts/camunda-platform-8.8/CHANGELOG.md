# Changelog

## [13.0.0-alpha5](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.8-13.0.0-alpha4.2...camunda-platform-8.8-13.0.0-alpha5) (2025-06-12)


### Features

* support core identity oidc ([#3493](https://github.com/camunda/camunda-platform-helm/issues/3493)) ([00bd285](https://github.com/camunda/camunda-platform-helm/commit/00bd28545ad5ec0e7bc3bfbc00ba9d27cc21881e))
* support image.digest for all components ([#3446](https://github.com/camunda/camunda-platform-helm/issues/3446)) ([d4e3510](https://github.com/camunda/camunda-platform-helm/commit/d4e351038059e032e6cdf8ffffbee7fc34634cfa))
* support ingress external hostname ([#3547](https://github.com/camunda/camunda-platform-helm/issues/3547)) ([2d23e04](https://github.com/camunda/camunda-platform-helm/commit/2d23e0497f74b6030359933fcff2ec06cdacdf28))


### Bug Fixes

* add main env to migration init container ([#3586](https://github.com/camunda/camunda-platform-helm/issues/3586)) ([60f0122](https://github.com/camunda/camunda-platform-helm/commit/60f012241ff24efdb0b983c5b7b84180d9627a9b))
* bump version in webModeler alpha to alpha5.1 ([#3646](https://github.com/camunda/camunda-platform-helm/issues/3646)) ([43844a9](https://github.com/camunda/camunda-platform-helm/commit/43844a9a13d5f3d21e98ae0134e37e7ed34cf78f))
* connectors default mapping for the core component ([#3650](https://github.com/camunda/camunda-platform-helm/issues/3650)) ([767d886](https://github.com/camunda/camunda-platform-helm/commit/767d8862e5f7ceccd4435cbdce9a1d31ecc7d792))
* correct identity 8.8 image tag ([#3566](https://github.com/camunda/camunda-platform-helm/issues/3566)) ([3ab47a4](https://github.com/camunda/camunda-platform-helm/commit/3ab47a4f9e40cb4bf6bce5dcc6848713d036bcb1))
* readd Zeebe NodeId logic from multi-region for 8.8 ([#3610](https://github.com/camunda/camunda-platform-helm/issues/3610)) ([6d902a3](https://github.com/camunda/camunda-platform-helm/commit/6d902a3308273ae468d4d6e4af7a10da467e3395))
* restore the original Zeebe startup script behaviour for backup operations ([#3589](https://github.com/camunda/camunda-platform-helm/issues/3589)) ([35dfe00](https://github.com/camunda/camunda-platform-helm/commit/35dfe0066bc7cc39e9e3a2466f2451135312fa75))


### Dependencies

* manually update connectors and core to alpha5 ([8b8f28b](https://github.com/camunda/camunda-platform-helm/commit/8b8f28b8fcde979d0c5d48351faceef880b60f96))
* update asdf-vm/actions action to v4 ([#3552](https://github.com/camunda/camunda-platform-helm/issues/3552)) ([c5bd7bb](https://github.com/camunda/camunda-platform-helm/commit/c5bd7bbe4c7ea934a974856b9b381b99660e6c31))
* update camunda-platform-8.8 (patch) ([#3625](https://github.com/camunda/camunda-platform-helm/issues/3625)) ([ffcfa08](https://github.com/camunda/camunda-platform-helm/commit/ffcfa083bc4beace9a2ed9c015381f14937f3354))
* update camunda/console docker tag to v8.8.0-alpha5 ([#3587](https://github.com/camunda/camunda-platform-helm/issues/3587)) ([c6777b6](https://github.com/camunda/camunda-platform-helm/commit/c6777b634320415a032dde2ab8847924b13909b9))
* update camunda/identity docker tag to v8.8.0-alpha5 ([#3602](https://github.com/camunda/camunda-platform-helm/issues/3602)) ([441b08d](https://github.com/camunda/camunda-platform-helm/commit/441b08d75efac90c0bddd8c20c6e164653ccdecf))
* update camunda/web-modeler-restapi docker tag to v8.8.0-alpha5 ([#3596](https://github.com/camunda/camunda-platform-helm/issues/3596)) ([3b4a180](https://github.com/camunda/camunda-platform-helm/commit/3b4a1807003560ded3393e9bb4fa438b82e75b3b))
* update dependency go to v1.24.3 ([#3479](https://github.com/camunda/camunda-platform-helm/issues/3479)) ([69b0516](https://github.com/camunda/camunda-platform-helm/commit/69b05161d60e771e666c2c685ce556c3392e7e23))
* update keycloak docker tag to v24.7.0 ([#3524](https://github.com/camunda/camunda-platform-helm/issues/3524)) ([dfd7aa6](https://github.com/camunda/camunda-platform-helm/commit/dfd7aa6271498515a40ff4b2f36d429768fcdb90))
* update keycloak docker tag to v24.7.1 ([#3538](https://github.com/camunda/camunda-platform-helm/issues/3538)) ([750e632](https://github.com/camunda/camunda-platform-helm/commit/750e632857eebbc085ca58a76a063d35032240c7))
* update keycloak docker tag to v24.7.3 ([#3564](https://github.com/camunda/camunda-platform-helm/issues/3564)) ([ed80d01](https://github.com/camunda/camunda-platform-helm/commit/ed80d013290dd2d7e1357c655e8127765f8a0741))
* update keycloak docker tag to v24.7.4 ([#3645](https://github.com/camunda/camunda-platform-helm/issues/3645)) ([baff77a](https://github.com/camunda/camunda-platform-helm/commit/baff77ad7bab201c68091b42c989976ea541b869))
* update module github.com/fatih/color to v1.18.0 ([#3480](https://github.com/camunda/camunda-platform-helm/issues/3480)) ([193d53b](https://github.com/camunda/camunda-platform-helm/commit/193d53b9a19a00eb5090aa17691c8667bd11be3b))
* update module github.com/spf13/cobra to v1.9.1 ([#3481](https://github.com/camunda/camunda-platform-helm/issues/3481)) ([07a378b](https://github.com/camunda/camunda-platform-helm/commit/07a378bd305acb951ac8073847e85e6bdc3edf49))
