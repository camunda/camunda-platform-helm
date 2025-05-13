# Changelog

## [12.1.0](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.7-12.0.1...camunda-platform-8.7-12.1.0) (2025-05-13)


### Features

* dedicated opensearch.prefix support ([#3379](https://github.com/camunda/camunda-platform-helm/issues/3379)) ([672886b](https://github.com/camunda/camunda-platform-helm/commit/672886bf8e308f56a0455f97fc62433d94aa6b1b))


### Bug Fixes

* add apiVersion and kind to PersistentVolumeClaims ([#2321](https://github.com/camunda/camunda-platform-helm/issues/2321)) ([b7c6092](https://github.com/camunda/camunda-platform-helm/commit/b7c6092654001387834c40e15b8adfffa7896b50))
* corrected a misplaced end key ([#3473](https://github.com/camunda/camunda-platform-helm/issues/3473)) ([ebda08d](https://github.com/camunda/camunda-platform-helm/commit/ebda08d1cce57a1c3c9a96e21f5e26aaca94e83e))
* elasticsearch.extraConfig is a table ([#3353](https://github.com/camunda/camunda-platform-helm/issues/3353)) ([2db5f0b](https://github.com/camunda/camunda-platform-helm/commit/2db5f0b802a4cc5068ea0e553fec58933fbf7b38))
* grpc port of zeebe gateway for venom tests ([#3364](https://github.com/camunda/camunda-platform-helm/issues/3364)) ([9228eb5](https://github.com/camunda/camunda-platform-helm/commit/9228eb542eaebb263c8fef88087291a4d82daf93))
* openshift elasticsearch disabled error ([#3472](https://github.com/camunda/camunda-platform-helm/issues/3472)) ([1f7d2f3](https://github.com/camunda/camunda-platform-helm/commit/1f7d2f38b6f149c7ef5d8d1b37c5ba5e6256f998))
* revert "feat: dedicated opensearch.prefix support" ([#3482](https://github.com/camunda/camunda-platform-helm/issues/3482)) ([bcfe32a](https://github.com/camunda/camunda-platform-helm/commit/bcfe32a1b1e9fbe9ca516a70831080bf3f7bb7d0))
* skip elasticsearch.extraConfig ([#3397](https://github.com/camunda/camunda-platform-helm/issues/3397)) ([f25b1f0](https://github.com/camunda/camunda-platform-helm/commit/f25b1f068c1d5168659544739c3e959b4138ffc3))
* update param for operate ([ad97adb](https://github.com/camunda/camunda-platform-helm/commit/ad97adb222f04c80d80a04989bc0901c6a615e7d))


### Dependencies

* update bitnami/keycloak docker tag to v26.1.5 ([#3374](https://github.com/camunda/camunda-platform-helm/issues/3374)) ([7282631](https://github.com/camunda/camunda-platform-helm/commit/7282631e166248f38b00ebd6b05e8c7013ed1ad4))
* update bitnami/keycloak docker tag to v26.2.0 ([#3375](https://github.com/camunda/camunda-platform-helm/issues/3375)) ([cab2beb](https://github.com/camunda/camunda-platform-helm/commit/cab2beb7b2f9287de29c292fd460e30519dc2798))
* update bitnami/postgresql docker tag to v14.17.0-debian-12-r15 ([#3383](https://github.com/camunda/camunda-platform-helm/issues/3383)) ([3b5f929](https://github.com/camunda/camunda-platform-helm/commit/3b5f929e88718a75f288ba1c2450eeb43cf47e8e))
* update bitnami/postgresql docker tag to v14.18.0-debian-12-r0 ([#3464](https://github.com/camunda/camunda-platform-helm/issues/3464)) ([b21a6ba](https://github.com/camunda/camunda-platform-helm/commit/b21a6baab9ed62dfb122d78fa3950f8d63183dba))
* update camunda-platform-8.7 (patch) ([#3389](https://github.com/camunda/camunda-platform-helm/issues/3389)) ([2b8078e](https://github.com/camunda/camunda-platform-helm/commit/2b8078e24a6d7153db9a6c54bc5a06675b3f8592))
* update camunda-platform-8.7 (patch) ([#3410](https://github.com/camunda/camunda-platform-helm/issues/3410)) ([7039d83](https://github.com/camunda/camunda-platform-helm/commit/7039d835cb8cbb747cdda1a2e445d5dbf99a5554))
* update camunda-platform-8.7 (patch) ([#3418](https://github.com/camunda/camunda-platform-helm/issues/3418)) ([7181890](https://github.com/camunda/camunda-platform-helm/commit/71818900c38858ae04ac52302678fe5e039e8776))
* update camunda-platform-8.7 (patch) ([#3436](https://github.com/camunda/camunda-platform-helm/issues/3436)) ([38a338d](https://github.com/camunda/camunda-platform-helm/commit/38a338de8e8aee345d4f5de4912cf45ee03afb3c))
* update camunda-platform-8.7 (patch) ([#3454](https://github.com/camunda/camunda-platform-helm/issues/3454)) ([01289f2](https://github.com/camunda/camunda-platform-helm/commit/01289f2b01ade26ff99757f7690c958fa4f7a84a))
* update camunda-platform-8.7 (patch) ([#3460](https://github.com/camunda/camunda-platform-helm/issues/3460)) ([581eca0](https://github.com/camunda/camunda-platform-helm/commit/581eca0e145418a3da1edb9410ffb098c4d0cd04))
* update camunda/connectors-bundle docker tag to v8.7.1 ([#3450](https://github.com/camunda/camunda-platform-helm/issues/3450)) ([1424e23](https://github.com/camunda/camunda-platform-helm/commit/1424e23ddd252eff7b70ed21aa9f255642dcc2d5))
* update camunda/connectors-bundle docker tag to v8.7.2 ([#3474](https://github.com/camunda/camunda-platform-helm/issues/3474)) ([2c220cb](https://github.com/camunda/camunda-platform-helm/commit/2c220cb213fb611da4f8ee41489d375bf0c747a4))
* update camunda/console docker tag to v8.7.11 ([#3459](https://github.com/camunda/camunda-platform-helm/issues/3459)) ([766c603](https://github.com/camunda/camunda-platform-helm/commit/766c603b7958f5dd494ef99462388e323d633077))
* update camunda/console docker tag to v8.7.17 ([#3477](https://github.com/camunda/camunda-platform-helm/issues/3477)) ([b4872d1](https://github.com/camunda/camunda-platform-helm/commit/b4872d1215a74e05ff87a59384f35ae12354619a))
* update camunda/console docker tag to v8.7.8 ([#3387](https://github.com/camunda/camunda-platform-helm/issues/3387)) ([5f7f704](https://github.com/camunda/camunda-platform-helm/commit/5f7f7049233539abbc258aca01a367d74dd84d96))
* update elasticsearch docker tag to v21.6.2 ([#3390](https://github.com/camunda/camunda-platform-helm/issues/3390)) ([7499fd5](https://github.com/camunda/camunda-platform-helm/commit/7499fd5fc3769b065cde8ae6f7601e757967f248))
* update keycloak docker tag to v24.5.2 ([#3305](https://github.com/camunda/camunda-platform-helm/issues/3305)) ([a586205](https://github.com/camunda/camunda-platform-helm/commit/a5862053bb7c6f49287976321a3cd71f01305de8))
* update keycloak docker tag to v24.6.1 ([#3393](https://github.com/camunda/camunda-platform-helm/issues/3393)) ([be33bd1](https://github.com/camunda/camunda-platform-helm/commit/be33bd10dc772676d385a625ae5a36cd079048f3))
* update keycloak docker tag to v24.6.2 ([#3411](https://github.com/camunda/camunda-platform-helm/issues/3411)) ([4ff9d4a](https://github.com/camunda/camunda-platform-helm/commit/4ff9d4a7bf225c5d35f62b698f90746d5c36dbf5))
* update module gopkg.in/yaml.v2 to v3 ([#3398](https://github.com/camunda/camunda-platform-helm/issues/3398)) ([4e8231c](https://github.com/camunda/camunda-platform-helm/commit/4e8231c4faacae58570136cf64bd58e3449944fe))

## [12.0.1](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.7-12.0.0...camunda-platform-8.7-12.0.1) (2025-04-10)


### Bug Fixes

* add elasticsearch golden files ([#3334](https://github.com/camunda/camunda-platform-helm/issues/3334)) ([1faf57e](https://github.com/camunda/camunda-platform-helm/commit/1faf57e08452710cfaeca42165498c680c407278))
* add gateway enabled ([#3307](https://github.com/camunda/camunda-platform-helm/issues/3307)) ([112dcb4](https://github.com/camunda/camunda-platform-helm/commit/112dcb4512a6060f649bae74fc6898c2e10daeb9))
* port of new style to camunda unit tests in 8.7 ([#3324](https://github.com/camunda/camunda-platform-helm/issues/3324)) ([39ab328](https://github.com/camunda/camunda-platform-helm/commit/39ab328374eb3440fda7d587d06f527d6884932d))
* refactor of connector unit tests to new style in 8.7 ([#3325](https://github.com/camunda/camunda-platform-helm/issues/3325)) ([fb56a1d](https://github.com/camunda/camunda-platform-helm/commit/fb56a1d2639c8787ec1dbfac4b1d4a54a0f10284))
* refactor of identity unit tests in 8.7 ([#3327](https://github.com/camunda/camunda-platform-helm/issues/3327)) ([3dbb27a](https://github.com/camunda/camunda-platform-helm/commit/3dbb27ab49c7ef546d2a58c6e20464e546ffbd7d))
* refactored console unit tests in 8.7 ([#3326](https://github.com/camunda/camunda-platform-helm/issues/3326)) ([90d6e84](https://github.com/camunda/camunda-platform-helm/commit/90d6e845119fb7674348e55696e06b13e0cb8861))
* refactored optimize unit tests to new style ([#3332](https://github.com/camunda/camunda-platform-helm/issues/3332)) ([80ebe8d](https://github.com/camunda/camunda-platform-helm/commit/80ebe8df31d12005ec0da3759402406b274dbb92))
* refactored tasklist unit tests to new style ([#3331](https://github.com/camunda/camunda-platform-helm/issues/3331)) ([c4f1eff](https://github.com/camunda/camunda-platform-helm/commit/c4f1effd5911a442dbb7eb765cdb8ea8db8b1b53))
* refactored web-modeler unit tests to new style ([#3330](https://github.com/camunda/camunda-platform-helm/issues/3330)) ([b273c10](https://github.com/camunda/camunda-platform-helm/commit/b273c1054dade6e7c1fee214f579ebe49b78cde4))
* refactored zeebe gateway unit tests to new style ([#3328](https://github.com/camunda/camunda-platform-helm/issues/3328)) ([1ce4a58](https://github.com/camunda/camunda-platform-helm/commit/1ce4a589c3f64e785d47e079f970dd80635c4b5e))
* refactored zeebe unit tests to new style in 8.7 ([#3329](https://github.com/camunda/camunda-platform-helm/issues/3329)) ([e71259f](https://github.com/camunda/camunda-platform-helm/commit/e71259f5ff3aece620b64a30fcb827cefc4edd67))
* update existingSecret params for 8.6 8.7 and 8.8 ([#3299](https://github.com/camunda/camunda-platform-helm/issues/3299)) ([057f855](https://github.com/camunda/camunda-platform-helm/commit/057f855936311fc1a90fc261aca3179f9172163c))


### Dependencies

* update camunda-platform-8.7 (patch) ([#3285](https://github.com/camunda/camunda-platform-helm/issues/3285)) ([18470b4](https://github.com/camunda/camunda-platform-helm/commit/18470b44bb539b9d4e7f5135a865b6aa63052dc9))
* update camunda-platform-8.7 (patch) ([#3314](https://github.com/camunda/camunda-platform-helm/issues/3314)) ([da8f6bb](https://github.com/camunda/camunda-platform-helm/commit/da8f6bbd93dc9373e776bc9fb8bcfe803f20bdd0))

## [12.0.0](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.7-12.0.0-alpha5...camunda-platform-8.7-12.0.0) (2025-04-02)


### Bug Fixes

* empty commit for releasable unit (release-please) ([023dca3](https://github.com/camunda/camunda-platform-helm/commit/023dca334710faf63a57da8aec970379a446f3a6))
* empty commit for releasable unit (release-please) ([#2766](https://github.com/camunda/camunda-platform-helm/issues/2766)) ([7c81e3d](https://github.com/camunda/camunda-platform-helm/commit/7c81e3db92a47be163a8bb7a4efe26cdfab10551))
* manual version bumps renovate couldnt do ([#3284](https://github.com/camunda/camunda-platform-helm/issues/3284)) ([ff598d6](https://github.com/camunda/camunda-platform-helm/commit/ff598d67bec0c11cc65bc33dbc8bbf3818d23dae))


### Dependencies

* update camunda-platform-8.7 (patch) ([#3255](https://github.com/camunda/camunda-platform-helm/issues/3255)) ([ee40ba3](https://github.com/camunda/camunda-platform-helm/commit/ee40ba39ebcee0d24ca92eeb53952bfcb9bae0b6))
* update camunda-platform-8.7 (patch) ([#3270](https://github.com/camunda/camunda-platform-helm/issues/3270)) ([1f7e778](https://github.com/camunda/camunda-platform-helm/commit/1f7e7788371ccde618cafe1920f478e275eda5b1))
* update module gopkg.in/yaml.v2 to v3 ([#3256](https://github.com/camunda/camunda-platform-helm/issues/3256)) ([c138e99](https://github.com/camunda/camunda-platform-helm/commit/c138e99a7b8f1db43f9af621b57cc26a14d8b3d8))


### Refactors

* disable elasticsearch deprecation warnings ([#3257](https://github.com/camunda/camunda-platform-helm/issues/3257)) ([ddfa3d2](https://github.com/camunda/camunda-platform-helm/commit/ddfa3d23919d0f5d0d12838e280daf60fd40fa5c))
* update apps deps ([c554fc5](https://github.com/camunda/camunda-platform-helm/commit/c554fc5354c4807172f55a39d0d74a51bd9031b4))
