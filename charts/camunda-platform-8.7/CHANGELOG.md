# Changelog

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
