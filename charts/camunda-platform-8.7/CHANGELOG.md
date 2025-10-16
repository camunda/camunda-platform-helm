# Changelog

## [12.6.3](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.7-12.6.2...camunda-platform-8.7-12.6.3) (2025-10-16)


### Bug Fixes

* fix link to upgrade instructions ([#4450](https://github.com/camunda/camunda-platform-helm/issues/4450)) ([ba9093b](https://github.com/camunda/camunda-platform-helm/commit/ba9093bc4ad884b689bda9c4e2c51a23b63d6ee0))


### Dependencies

* update camunda-platform-8.7 (patch) ([#4487](https://github.com/camunda/camunda-platform-helm/issues/4487)) ([a3a99bc](https://github.com/camunda/camunda-platform-helm/commit/a3a99bcfbefd550274e97b2b84ea27cfec5d1ca4))

## [12.6.2](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.7-12.6.1...camunda-platform-8.7-12.6.2) (2025-10-10)


### Bug Fixes

* 2 issues.. the configmap for optimize was using the elasticsearch prefix value for opensearch and the QA scenarios where reusing the global prefix to shared OS failed CI ([#4238](https://github.com/camunda/camunda-platform-helm/issues/4238)) ([962cc2b](https://github.com/camunda/camunda-platform-helm/commit/962cc2bfa706c71d1becea6226c25650136ca31b))
* correct sysctlImage structure in values-enterprise.yaml ([#4339](https://github.com/camunda/camunda-platform-helm/issues/4339)) ([8dd44f2](https://github.com/camunda/camunda-platform-helm/commit/8dd44f23f750569253c4a8ea57b98d19bb365879))
* revert renovate bot version bump ([#4376](https://github.com/camunda/camunda-platform-helm/issues/4376)) ([801200d](https://github.com/camunda/camunda-platform-helm/commit/801200df1e14e58a8eab6663a663e597d54fbb30))
* this PR is enabling Entra for testing ([#4294](https://github.com/camunda/camunda-platform-helm/issues/4294)) ([3a6ca22](https://github.com/camunda/camunda-platform-helm/commit/3a6ca22db1aa1c28edbe0a2a4cd3790dff145493))
* web modeler extraConfiguration uses subcomponent subkey ([#4097](https://github.com/camunda/camunda-platform-helm/issues/4097)) ([72b7978](https://github.com/camunda/camunda-platform-helm/commit/72b7978b00ccdb29fcf61f8c636acc82103449a4))


### Dependencies

* update camunda-platform-8.7 (patch) ([#4177](https://github.com/camunda/camunda-platform-helm/issues/4177)) ([6897652](https://github.com/camunda/camunda-platform-helm/commit/6897652c25712f42787e3600fa66d71c8d0a2aea))
* update camunda-platform-8.7 (patch) ([#4378](https://github.com/camunda/camunda-platform-helm/issues/4378)) ([32bcc20](https://github.com/camunda/camunda-platform-helm/commit/32bcc2039f71e62a83c412129a8e0cb8c890f792))
* update module github.com/gruntwork-io/terratest to v0.51.0 ([#4268](https://github.com/camunda/camunda-platform-helm/issues/4268)) ([a47c21c](https://github.com/camunda/camunda-platform-helm/commit/a47c21ce6205cde4521840b7b1eb41294a5c005f))


### Refactors

* upgrade keycloak image from 26.3.1 to 26.3.3 ([#4386](https://github.com/camunda/camunda-platform-helm/issues/4386)) ([d5e0a0b](https://github.com/camunda/camunda-platform-helm/commit/d5e0a0b34b111c2c9ff83cdbbbb447e0543902fb))
