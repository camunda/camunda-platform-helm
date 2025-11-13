# Changelog

## [14.0.0-alpha1](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.9-14.0.0-alpha0...camunda-platform-8.9-14.0.0-alpha1) (2025-11-12)


### Features

* create 8.9 helm chart version ([#4463](https://github.com/camunda/camunda-platform-helm/issues/4463)) ([0f46741](https://github.com/camunda/camunda-platform-helm/commit/0f46741641c35ec2824bd17c1c4269cbfece07ae))


### Bug Fixes

* add missing files from 8.9 ([#4469](https://github.com/camunda/camunda-platform-helm/issues/4469)) ([f3274e3](https://github.com/camunda/camunda-platform-helm/commit/f3274e3d434ae54a183f0d272959012361a1f6ef))
* allows cpu to be set as either string or number ([#4491](https://github.com/camunda/camunda-platform-helm/issues/4491)) ([a30c924](https://github.com/camunda/camunda-platform-helm/commit/a30c9249329346012427df2d814d61476b51ccc6))
* apply dual-region exclusion logic to opensearch exporter as well ([#4352](https://github.com/camunda/camunda-platform-helm/issues/4352)) ([8b0aff7](https://github.com/camunda/camunda-platform-helm/commit/8b0aff7ad1de25d3187497321707b9b278882fca))
* configmap for orchestration renders as valid yaml and not giant blob ([#4591](https://github.com/camunda/camunda-platform-helm/issues/4591)) ([6634917](https://github.com/camunda/camunda-platform-helm/commit/66349172d5d661e68a206da1066842e079e9415f))
* correct retention configuration in orchestration configMap ([#4620](https://github.com/camunda/camunda-platform-helm/issues/4620)) ([5436098](https://github.com/camunda/camunda-platform-helm/commit/543609870228a9b8f430b58aaf72520bf80ca5ff))
* delete the migrator from 8.9 ([#4503](https://github.com/camunda/camunda-platform-helm/issues/4503)) ([3ead5cc](https://github.com/camunda/camunda-platform-helm/commit/3ead5cc1a15b3070c611e0ec0fdbb3aed18d86f5))
* disable schema manager version check restriction for upgrades ([#4645](https://github.com/camunda/camunda-platform-helm/issues/4645)) ([da5d8c0](https://github.com/camunda/camunda-platform-helm/commit/da5d8c018270a27842d59bb82fd59e0d833e5cbd))
* disable version check restrict on qa upgrades ([#4624](https://github.com/camunda/camunda-platform-helm/issues/4624)) ([8a5fb84](https://github.com/camunda/camunda-platform-helm/commit/8a5fb841224e5672548944366894231bbdab6403))
* move config under camunda ([#4590](https://github.com/camunda/camunda-platform-helm/issues/4590)) ([96fd240](https://github.com/camunda/camunda-platform-helm/commit/96fd240cd71efbed80fa82e4f21d429861f61c49))
* remove console secret since console is a public OIDC client ([#4482](https://github.com/camunda/camunda-platform-helm/issues/4482)) ([8e37948](https://github.com/camunda/camunda-platform-helm/commit/8e379489816c8ee1b7d317e8e2af784d662606e2))
* use joinPath helper to fix double slash issues in console configmap ([#4562](https://github.com/camunda/camunda-platform-helm/issues/4562)) ([4e06bd5](https://github.com/camunda/camunda-platform-helm/commit/4e06bd57017a7116c607a97a1fce299dd93f4725))


### Documentation

* update architecture sections in readme 8.8+ ([#4688](https://github.com/camunda/camunda-platform-helm/issues/4688)) ([f4f62b9](https://github.com/camunda/camunda-platform-helm/commit/f4f62b9c74b18abe2a7d4934b129a34cf4d3ff2b))


### Dependencies

* update camunda-platform-8.9 (patch) ([#4468](https://github.com/camunda/camunda-platform-helm/issues/4468)) ([5284b1a](https://github.com/camunda/camunda-platform-helm/commit/5284b1a815f3d5e7a4b4c8f856f6899867918d8c))
* update camunda-platform-8.9-digest ([#4465](https://github.com/camunda/camunda-platform-helm/issues/4465)) ([9dd6b86](https://github.com/camunda/camunda-platform-helm/commit/9dd6b861b186b093a4e130094470d283ae808b49))
* update camunda-platform-8.9-digest ([#4499](https://github.com/camunda/camunda-platform-helm/issues/4499)) ([4334c76](https://github.com/camunda/camunda-platform-helm/commit/4334c7612278ced01619fbfa79b2dc3732cffe60))
* update camunda-platform-8.9-digest ([#4510](https://github.com/camunda/camunda-platform-helm/issues/4510)) ([13ba85e](https://github.com/camunda/camunda-platform-helm/commit/13ba85eb06b4901835e67a1388a36bf063b6ba13))
* update camunda-platform-8.9-digest ([#4515](https://github.com/camunda/camunda-platform-helm/issues/4515)) ([c22ac04](https://github.com/camunda/camunda-platform-helm/commit/c22ac041b805c7776397e4bebc3fc6b687edcd04))
* update camunda-platform-8.9-digest ([#4517](https://github.com/camunda/camunda-platform-helm/issues/4517)) ([a6240f8](https://github.com/camunda/camunda-platform-helm/commit/a6240f8598eec28f41d2d70ec3c1d0913f49fac0))
* update camunda-platform-8.9-digest ([#4519](https://github.com/camunda/camunda-platform-helm/issues/4519)) ([380001e](https://github.com/camunda/camunda-platform-helm/commit/380001e6bf0229e6474e83caa9150d2bc0bafaf9))
* update camunda-platform-8.9-digest ([#4522](https://github.com/camunda/camunda-platform-helm/issues/4522)) ([b33796f](https://github.com/camunda/camunda-platform-helm/commit/b33796fdc989184904fc7085a0601bf08166dc08))
* update camunda-platform-8.9-digest ([#4530](https://github.com/camunda/camunda-platform-helm/issues/4530)) ([eda038d](https://github.com/camunda/camunda-platform-helm/commit/eda038d3588df5f291c7f188e342fefdf6d75357))
* update camunda-platform-8.9-digest ([#4544](https://github.com/camunda/camunda-platform-helm/issues/4544)) ([038341c](https://github.com/camunda/camunda-platform-helm/commit/038341c6d68a63b3568c0ecfd8cfd49969dec2b5))
* update camunda-platform-8.9-digest ([#4546](https://github.com/camunda/camunda-platform-helm/issues/4546)) ([5ea1d14](https://github.com/camunda/camunda-platform-helm/commit/5ea1d1413dde4814711941e743c3f172a99929ee))
* update camunda-platform-8.9-digest ([#4548](https://github.com/camunda/camunda-platform-helm/issues/4548)) ([dcc7341](https://github.com/camunda/camunda-platform-helm/commit/dcc73415155467e51ac7de2326a2185116231ca0))
* update camunda-platform-8.9-digest ([#4551](https://github.com/camunda/camunda-platform-helm/issues/4551)) ([e28e413](https://github.com/camunda/camunda-platform-helm/commit/e28e413a2c06e505ef184c85c2cecf0df458ee7a))
* update camunda-platform-digests ([#4566](https://github.com/camunda/camunda-platform-helm/issues/4566)) ([2a0c9fc](https://github.com/camunda/camunda-platform-helm/commit/2a0c9fc7d1fdf050aac08f9ddd6224c24c018678))
* update camunda-platform-digests ([#4575](https://github.com/camunda/camunda-platform-helm/issues/4575)) ([9865a11](https://github.com/camunda/camunda-platform-helm/commit/9865a11fa75702e00a0ee8f8b6a79b452f54d5b6))
* update camunda-platform-digests ([#4579](https://github.com/camunda/camunda-platform-helm/issues/4579)) ([90db16a](https://github.com/camunda/camunda-platform-helm/commit/90db16a5a573d3f859359e3b46a07221506f22f1))
* update camunda-platform-digests ([#4585](https://github.com/camunda/camunda-platform-helm/issues/4585)) ([73376ee](https://github.com/camunda/camunda-platform-helm/commit/73376ee81a5d4f1937ed2674aeaebc5e16afd8b3))
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
* update camunda-platform-digests ([#4684](https://github.com/camunda/camunda-platform-helm/issues/4684)) ([1575866](https://github.com/camunda/camunda-platform-helm/commit/1575866bdbe871acf2970fd888a49977481e528b))
* update camunda-platform-digests ([#4692](https://github.com/camunda/camunda-platform-helm/issues/4692)) ([de97226](https://github.com/camunda/camunda-platform-helm/commit/de972264f55ffa9e9c9a6d3c44f766e04a5fed5e))
* update camunda-platform-images ([#4583](https://github.com/camunda/camunda-platform-helm/issues/4583)) ([df7cf00](https://github.com/camunda/camunda-platform-helm/commit/df7cf000041d432d25dc566a08b2e19ad1348ac6))
* update camunda-platform-images ([#4627](https://github.com/camunda/camunda-platform-helm/issues/4627)) ([6cba63c](https://github.com/camunda/camunda-platform-helm/commit/6cba63c3b79044961a0f189af71126a5b64cfe18))
* update camunda/camunda docker tag to v8.8.1 ([#4561](https://github.com/camunda/camunda-platform-helm/issues/4561)) ([ec0bf9c](https://github.com/camunda/camunda-platform-helm/commit/ec0bf9cebadb082580d22f181a78d61f81efdc53))
* update camunda/camunda:snapshot docker digest to 68302dc ([#4539](https://github.com/camunda/camunda-platform-helm/issues/4539)) ([4a70306](https://github.com/camunda/camunda-platform-helm/commit/4a703064777c34afd152c2be471ed3668ae7a72c))
* update camunda/camunda:snapshot docker digest to ae47645 ([#4513](https://github.com/camunda/camunda-platform-helm/issues/4513)) ([aa5c92f](https://github.com/camunda/camunda-platform-helm/commit/aa5c92f23f46188c4528bc6f895436419f1347c1))
* update camunda/console docker tag to v8.8.11 ([#4529](https://github.com/camunda/camunda-platform-helm/issues/4529)) ([631e314](https://github.com/camunda/camunda-platform-helm/commit/631e31402e5b45c40661db8ae554af82c29745fa))
* update camunda/console docker tag to v8.8.12 ([#4532](https://github.com/camunda/camunda-platform-helm/issues/4532)) ([84b9cde](https://github.com/camunda/camunda-platform-helm/commit/84b9cde1aec61ca233b97c732206e990eba75c97))
* update camunda/console docker tag to v8.8.13 ([#4559](https://github.com/camunda/camunda-platform-helm/issues/4559)) ([69d99f7](https://github.com/camunda/camunda-platform-helm/commit/69d99f77f1251a09c12c94e7fc7c550d0e60cc3c))
* update camunda/identity:snapshot docker digest to c243af6 ([#4556](https://github.com/camunda/camunda-platform-helm/issues/4556)) ([6d0c54f](https://github.com/camunda/camunda-platform-helm/commit/6d0c54f1e8b28d29b2d6dd91e0d2ebf3ec99a430))
* update camunda/web-modeler-restapi docker tag to v8.9.0-alpha1-rc3 ([#4632](https://github.com/camunda/camunda-platform-helm/issues/4632)) ([1645977](https://github.com/camunda/camunda-platform-helm/commit/1645977fe1ef4037d122512da8235bfea7d0479f))
* update minor-updates (minor) ([#4639](https://github.com/camunda/camunda-platform-helm/issues/4639)) ([6994d28](https://github.com/camunda/camunda-platform-helm/commit/6994d2872232be8a0e8f0d54fde2613ac976a5af))
* update patch-updates (patch) ([#4638](https://github.com/camunda/camunda-platform-helm/issues/4638)) ([9f0fa16](https://github.com/camunda/camunda-platform-helm/commit/9f0fa160ac513aa282d668abbafc7fa4c8c5fe53))
* update registry.camunda.cloud/keycloak-ee/keycloak docker tag to v26.4.1 ([#4541](https://github.com/camunda/camunda-platform-helm/issues/4541)) ([f0f0498](https://github.com/camunda/camunda-platform-helm/commit/f0f0498dcc0daa34287fa3574c7d0fb05e0bbc5c))
* update registry.camunda.cloud/keycloak-ee/keycloak docker tag to v26.4.4 ([#4664](https://github.com/camunda/camunda-platform-helm/issues/4664)) ([6c0b4b9](https://github.com/camunda/camunda-platform-helm/commit/6c0b4b9478e5deb72e43cf04ed24a4ec0bb3f584))
