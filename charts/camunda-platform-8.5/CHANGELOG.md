# Changelog

## [10.7.0](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.5-10.6.0...camunda-platform-8.5-10.7.0) (2025-05-09)


### Features

* dedicated opensearch.prefix support ([#3379](https://github.com/camunda/camunda-platform-helm/issues/3379)) ([672886b](https://github.com/camunda/camunda-platform-helm/commit/672886bf8e308f56a0455f97fc62433d94aa6b1b))


### Bug Fixes

* add apiVersion and kind to PersistentVolumeClaims ([#2321](https://github.com/camunda/camunda-platform-helm/issues/2321)) ([b7c6092](https://github.com/camunda/camunda-platform-helm/commit/b7c6092654001387834c40e15b8adfffa7896b50))
* add elasticsearch golden files ([#3334](https://github.com/camunda/camunda-platform-helm/issues/3334)) ([1faf57e](https://github.com/camunda/camunda-platform-helm/commit/1faf57e08452710cfaeca42165498c680c407278))
* backport 8.6 refactoring on test/unit/camunda to 8.5 ([#3301](https://github.com/camunda/camunda-platform-helm/issues/3301)) ([6d1276c](https://github.com/camunda/camunda-platform-helm/commit/6d1276c54c1ec133c9b86c48ae4ed92b610ba5a7))
* grpc port of zeebe gateway for venom tests ([#3364](https://github.com/camunda/camunda-platform-helm/issues/3364)) ([9228eb5](https://github.com/camunda/camunda-platform-helm/commit/9228eb542eaebb263c8fef88087291a4d82daf93))
* refactor unit tests in connectors of 8.5 to new styles ([#3304](https://github.com/camunda/camunda-platform-helm/issues/3304)) ([3a58144](https://github.com/camunda/camunda-platform-helm/commit/3a58144d5db1ed01461e0327e674c4d49f019ef2))
* refactored unit tests to new style in console ([#3335](https://github.com/camunda/camunda-platform-helm/issues/3335)) ([30a34f6](https://github.com/camunda/camunda-platform-helm/commit/30a34f65932f4c0788e29f4f97069aa209e8452f))
* refactored unit tests to new style in identity 8.5 ([#3340](https://github.com/camunda/camunda-platform-helm/issues/3340)) ([bb22084](https://github.com/camunda/camunda-platform-helm/commit/bb22084686acb7398792850bf3a7f4cda0f8c37a))
* refactored unit tests to new style in operate on 8.5 ([#3339](https://github.com/camunda/camunda-platform-helm/issues/3339)) ([05dc621](https://github.com/camunda/camunda-platform-helm/commit/05dc621114090a8dd1e3522ad139050517c3098f))
* refactored unit tests to new style in optimize on 8.5 ([#3338](https://github.com/camunda/camunda-platform-helm/issues/3338)) ([c36da13](https://github.com/camunda/camunda-platform-helm/commit/c36da133c3d4e6652a64c80e4e5f724f7ba62145))
* refactored unit tests to new style in tasklist ([#3337](https://github.com/camunda/camunda-platform-helm/issues/3337)) ([07feb13](https://github.com/camunda/camunda-platform-helm/commit/07feb131badba333b1c056c2a4b2be9c208ab965))
* refactored unit tests to new style in web modeler ([#3336](https://github.com/camunda/camunda-platform-helm/issues/3336)) ([bf4bd5c](https://github.com/camunda/camunda-platform-helm/commit/bf4bd5c8cd7ce03b99111fe2fc4901f51e5bf3a4))
* refactored unit tests to new style in zeebe 8.5 ([#3342](https://github.com/camunda/camunda-platform-helm/issues/3342)) ([6cdf419](https://github.com/camunda/camunda-platform-helm/commit/6cdf4196e1572e593ef852ea1e7fb9c63a323157))
* refactored unit tests to new style in zeebe-gateway 8.5 ([#3341](https://github.com/camunda/camunda-platform-helm/issues/3341)) ([c61ff9a](https://github.com/camunda/camunda-platform-helm/commit/c61ff9abcd712c449057c63723b3a696bec73ad5))


### Dependencies

* update bitnami/postgresql docker tag to v14.17.0-debian-12-r15 ([#3312](https://github.com/camunda/camunda-platform-helm/issues/3312)) ([9fb4e35](https://github.com/camunda/camunda-platform-helm/commit/9fb4e3525e0b5dc82290db984e90397457a07c7f))
* update bitnami/postgresql docker tag to v14.17.0-debian-12-r16 ([#3400](https://github.com/camunda/camunda-platform-helm/issues/3400)) ([cd27e73](https://github.com/camunda/camunda-platform-helm/commit/cd27e73c723473dfe8ebd9435737c9fe21ee464a))
* update bitnami/postgresql docker tag to v14.17.0-debian-12-r17 ([#3402](https://github.com/camunda/camunda-platform-helm/issues/3402)) ([f0b5316](https://github.com/camunda/camunda-platform-helm/commit/f0b531643c93f5608224227da3ac11af14633c41))
* update camunda-platform-8.5 (patch) ([#3439](https://github.com/camunda/camunda-platform-helm/issues/3439)) ([8c22cfd](https://github.com/camunda/camunda-platform-helm/commit/8c22cfdf9ed8e2a00d48ad4a2731077560b8cf27))
* update camunda-platform-8.5 (patch) ([#3443](https://github.com/camunda/camunda-platform-helm/issues/3443)) ([847079a](https://github.com/camunda/camunda-platform-helm/commit/847079ab4504716915af78233228905e127e985c))
* update camunda-platform-8.5 (patch) ([#3458](https://github.com/camunda/camunda-platform-helm/issues/3458)) ([4ff7795](https://github.com/camunda/camunda-platform-helm/commit/4ff7795ff22480fd52a76f38bb06ec678f907d25))
* update camunda-platform-8.5 to v8.5.115 (patch) ([#3453](https://github.com/camunda/camunda-platform-helm/issues/3453)) ([d84465f](https://github.com/camunda/camunda-platform-helm/commit/d84465f5c10be7592c5d07a5e00632ce1875d405))
* update camunda-platform-8.5 to v8.5.16 (patch) ([#3435](https://github.com/camunda/camunda-platform-helm/issues/3435)) ([1492a42](https://github.com/camunda/camunda-platform-helm/commit/1492a422c4a6b85174fc42ff7db9f53e0e3d65fd))
* update camunda/connectors-bundle docker tag to v8.5.16 ([#3416](https://github.com/camunda/camunda-platform-helm/issues/3416)) ([f637777](https://github.com/camunda/camunda-platform-helm/commit/f637777109491f7de6e3e3d1174df5d8b395ec86))
* update module gopkg.in/yaml.v2 to v3 ([#3398](https://github.com/camunda/camunda-platform-helm/issues/3398)) ([4e8231c](https://github.com/camunda/camunda-platform-helm/commit/4e8231c4faacae58570136cf64bd58e3449944fe))
