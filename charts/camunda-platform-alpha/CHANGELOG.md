# Changelog

## [8.8.0-alpha2](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-alpha-v8.7.0-alpha2...camunda-platform-alpha-8.8.0-alpha2) (2025-01-14)


### Features

* adding schema.json file to alpha (8.7) version ([#2537](https://github.com/camunda/camunda-platform-helm/issues/2537)) ([d7b4530](https://github.com/camunda/camunda-platform-helm/commit/d7b453030c533d6bfb2fd7508e444a48de99789c))
* **alpha:** add Connectors to release items ([#2659](https://github.com/camunda/camunda-platform-helm/issues/2659)) ([50a8392](https://github.com/camunda/camunda-platform-helm/commit/50a839276a485b33421b8e624a255fb3adfb7482))


### Bug Fixes

* **alpha:** add connectors init secret to identity ([245a28e](https://github.com/camunda/camunda-platform-helm/commit/245a28e1f18c3742607ff4884ee603d307259382))
* **alpha:** add connectors init secret to identity ([0a4441e](https://github.com/camunda/camunda-platform-helm/commit/0a4441e59cf23f10a699fd5df333fe611a029a23))
* **alpha:** move core sts command to correct location ([#2712](https://github.com/camunda/camunda-platform-helm/issues/2712)) ([0b88c6f](https://github.com/camunda/camunda-platform-helm/commit/0b88c6fb0eb215feb7951bba74a6a2e3c6141b22))
* **alpha:** move identity admin client to presets ([#2714](https://github.com/camunda/camunda-platform-helm/issues/2714)) ([c671090](https://github.com/camunda/camunda-platform-helm/commit/c6710909bcb41259520de87a23947c4c8b52bb5e))
* **alpha:** show error on usage of core.ingress.rest not core.ingress ([9318266](https://github.com/camunda/camunda-platform-helm/commit/93182668ade9cd99ce51423fca6869ea09504e82))
* changing `existingSecret.name` to comply with schema ([#2726](https://github.com/camunda/camunda-platform-helm/issues/2726)) ([c399f09](https://github.com/camunda/camunda-platform-helm/commit/c399f09e82d21cf11cbbcfd6ae9c61cd09d7b965))
* client-secret should only be present when string literal provided for oidc ([#2733](https://github.com/camunda/camunda-platform-helm/issues/2733)) ([6cd9313](https://github.com/camunda/camunda-platform-helm/commit/6cd9313aed1474d2c92143e7ea8b33ae3bd3a634))
* **deps:** update camunda-platform-alpha (patch) ([#2701](https://github.com/camunda/camunda-platform-helm/issues/2701)) ([a2661e2](https://github.com/camunda/camunda-platform-helm/commit/a2661e2767a6aaf1ff75bc485db152133f2a8116))
* empty commit for releasable unit (release-please) ([#2766](https://github.com/camunda/camunda-platform-helm/issues/2766)) ([7c81e3d](https://github.com/camunda/camunda-platform-helm/commit/7c81e3db92a47be163a8bb7a4efe26cdfab10551))
* set identity client secret env var in alpha ([#2703](https://github.com/camunda/camunda-platform-helm/issues/2703)) ([d48086c](https://github.com/camunda/camunda-platform-helm/commit/d48086cb9f3a0d9b8b2a5fa3ff47b8bf12c478c6))
* **zeebe-grpc-ingress:** class check for openshift was not checked ([#2678](https://github.com/camunda/camunda-platform-helm/issues/2678)) ([873dbd0](https://github.com/camunda/camunda-platform-helm/commit/873dbd08ca63292312e5965b2d5d43daeaa7da4f))


### Refactors

* **alpha:** adjust resources for core sts ([#2702](https://github.com/camunda/camunda-platform-helm/issues/2702)) ([b9102ee](https://github.com/camunda/camunda-platform-helm/commit/b9102ee8ff3ddf78378d4fb5b776ee7a31476749))
* **alpha:** enable connectors readinessProbe again ([cc74f31](https://github.com/camunda/camunda-platform-helm/commit/cc74f31c5d41a17e6636714cf39dfcabf8b06948))
* remove unused web-modeler config map entries ([#2708](https://github.com/camunda/camunda-platform-helm/issues/2708)) ([bd74490](https://github.com/camunda/camunda-platform-helm/commit/bd744904ded9f3f308d07c2b3e62755ef6429cdc))
* remove/rename deprecated helm values file keys ([#2615](https://github.com/camunda/camunda-platform-helm/issues/2615)) ([9a7f111](https://github.com/camunda/camunda-platform-helm/commit/9a7f111f3615fff9c2c9a41bdef12daa0276fedf))
