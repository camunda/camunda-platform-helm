# Changelog

## [13.1.2](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.8-13.1.1...camunda-platform-8.8-13.1.2) (2025-11-11)


### Bug Fixes

* `optmimize.persistence.enabled` crashes optimize ([#4682](https://github.com/camunda/camunda-platform-helm/issues/4682)) ([f7e7835](https://github.com/camunda/camunda-platform-helm/commit/f7e78354f76b6d77fb58e1ad691a4ebe99a11083))
* configmap for orchestration renders as valid yaml and not giant blob ([#4591](https://github.com/camunda/camunda-platform-helm/issues/4591)) ([6634917](https://github.com/camunda/camunda-platform-helm/commit/66349172d5d661e68a206da1066842e079e9415f))
* correct retention configuration in orchestration configMap ([#4620](https://github.com/camunda/camunda-platform-helm/issues/4620)) ([5436098](https://github.com/camunda/camunda-platform-helm/commit/543609870228a9b8f430b58aaf72520bf80ca5ff))
* disable version check restrict on qa upgrades ([#4651](https://github.com/camunda/camunda-platform-helm/issues/4651)) ([352e4dc](https://github.com/camunda/camunda-platform-helm/commit/352e4dc3abf89f76997f170936cea413cbebfd22))
* include exported password in importer deployment ([#4602](https://github.com/camunda/camunda-platform-helm/issues/4602)) ([300ef27](https://github.com/camunda/camunda-platform-helm/commit/300ef27013702516e1b53cae1dc0b8d36e762886))
* remove console secret since console is a public OIDC client ([#4482](https://github.com/camunda/camunda-platform-helm/issues/4482)) ([8e37948](https://github.com/camunda/camunda-platform-helm/commit/8e379489816c8ee1b7d317e8e2af784d662606e2))
* zbctl 8.5.6 was deleted, long live 8.6.0 ([#4650](https://github.com/camunda/camunda-platform-helm/issues/4650)) ([6336915](https://github.com/camunda/camunda-platform-helm/commit/6336915a8b0bcb51048508fe302c4caa71ff079a))


### Dependencies

* update camunda-platform-digests ([#4592](https://github.com/camunda/camunda-platform-helm/issues/4592)) ([715f133](https://github.com/camunda/camunda-platform-helm/commit/715f133d35dc814459f6c85226d05885776f56d2))
* update camunda-platform-digests ([#4600](https://github.com/camunda/camunda-platform-helm/issues/4600)) ([f2de333](https://github.com/camunda/camunda-platform-helm/commit/f2de33367b2cb2e62aba6a2f0fed5278f55aeeda))
* update camunda-platform-digests ([#4610](https://github.com/camunda/camunda-platform-helm/issues/4610)) ([5244e63](https://github.com/camunda/camunda-platform-helm/commit/5244e635e8719302f813000e5c5f31ec464156ce))
* update camunda-platform-digests ([#4612](https://github.com/camunda/camunda-platform-helm/issues/4612)) ([19d9c4f](https://github.com/camunda/camunda-platform-helm/commit/19d9c4f0ad41154d1547648d09fe0a4d87b328fa))
* update camunda-platform-digests ([#4616](https://github.com/camunda/camunda-platform-helm/issues/4616)) ([94ef8ba](https://github.com/camunda/camunda-platform-helm/commit/94ef8bad79bfe59d208cc022b2af7f67167188d5))
* update camunda-platform-digests ([#4623](https://github.com/camunda/camunda-platform-helm/issues/4623)) ([254f7a0](https://github.com/camunda/camunda-platform-helm/commit/254f7a041807c693cfd88d03eadc2df35da6b3d0))
* update camunda-platform-digests ([#4625](https://github.com/camunda/camunda-platform-helm/issues/4625)) ([45958d6](https://github.com/camunda/camunda-platform-helm/commit/45958d6e881c6a91ff42864d38dd1f4a9e021e75))
* update camunda-platform-digests ([#4631](https://github.com/camunda/camunda-platform-helm/issues/4631)) ([ff6c5c2](https://github.com/camunda/camunda-platform-helm/commit/ff6c5c24fc2073cf126c2b72d98e919569aa4dcd))
* update camunda-platform-digests ([#4634](https://github.com/camunda/camunda-platform-helm/issues/4634)) ([8b5f95b](https://github.com/camunda/camunda-platform-helm/commit/8b5f95b89732e90965e5b4eb57de1edf03580d12))
* update camunda-platform-digests ([#4644](https://github.com/camunda/camunda-platform-helm/issues/4644)) ([8d56404](https://github.com/camunda/camunda-platform-helm/commit/8d564046281effac0bc7a22c56a3558eed7f7754))
* update camunda-platform-digests ([#4658](https://github.com/camunda/camunda-platform-helm/issues/4658)) ([9547e5f](https://github.com/camunda/camunda-platform-helm/commit/9547e5f3009f93c0bf39e7b4fee2a9c9de4e5e35))
* update camunda-platform-digests ([#4677](https://github.com/camunda/camunda-platform-helm/issues/4677)) ([f0d8177](https://github.com/camunda/camunda-platform-helm/commit/f0d81774985088322ee6fd9a56a52dc2e1a0337e))
* update camunda-platform-images ([#4627](https://github.com/camunda/camunda-platform-helm/issues/4627)) ([6cba63c](https://github.com/camunda/camunda-platform-helm/commit/6cba63c3b79044961a0f189af71126a5b64cfe18))
* update camunda-platform-images (patch) ([#4637](https://github.com/camunda/camunda-platform-helm/issues/4637)) ([6e69edc](https://github.com/camunda/camunda-platform-helm/commit/6e69edc0c10bbca442a40f9882c49dd63a03d277))
* update camunda-platform-images (patch) ([#4659](https://github.com/camunda/camunda-platform-helm/issues/4659)) ([6a4e5d8](https://github.com/camunda/camunda-platform-helm/commit/6a4e5d8584747b067c87f364b1322b4c2c018e36))
* update minor-updates (minor) ([#4639](https://github.com/camunda/camunda-platform-helm/issues/4639)) ([6994d28](https://github.com/camunda/camunda-platform-helm/commit/6994d2872232be8a0e8f0d54fde2613ac976a5af))
* update patch-updates (patch) ([#4638](https://github.com/camunda/camunda-platform-helm/issues/4638)) ([9f0fa16](https://github.com/camunda/camunda-platform-helm/commit/9f0fa160ac513aa282d668abbafc7fa4c8c5fe53))
* update registry.camunda.cloud/keycloak-ee/keycloak docker tag to v26.4.4 ([#4664](https://github.com/camunda/camunda-platform-helm/issues/4664)) ([6c0b4b9](https://github.com/camunda/camunda-platform-helm/commit/6c0b4b9478e5deb72e43cf04ed24a4ec0bb3f584))
