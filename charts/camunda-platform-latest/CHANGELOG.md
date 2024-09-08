# Changelog

## [10.4.0](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-latest-v10.3.2...camunda-platform-latest-10.4.0) (2024-09-08)


### Features

* adding aws config in apps for OpenSearch ([#2232](https://github.com/camunda/camunda-platform-helm/issues/2232)) ([9b77ec5](https://github.com/camunda/camunda-platform-helm/commit/9b77ec59cb71bd68438e503d8423c318777c03ed))
* adding play env var to web-modeler ([#2269](https://github.com/camunda/camunda-platform-helm/issues/2269)) ([fa99b6d](https://github.com/camunda/camunda-platform-helm/commit/fa99b6d7dee41857330307074ece21e7e78fd719))
* support optional chart secrets auto-generation ([#2257](https://github.com/camunda/camunda-platform-helm/issues/2257)) ([37085aa](https://github.com/camunda/camunda-platform-helm/commit/37085aa650208e20568553b72f813a7c6a1216eb))
* support optional chart secrets auto-generation ([#2288](https://github.com/camunda/camunda-platform-helm/issues/2288)) ([33c25ad](https://github.com/camunda/camunda-platform-helm/commit/33c25ada0eef27774444585fdd001f8c3e05c233))


### Bug Fixes

* add zeebe opensearch retention to app config file ([#2250](https://github.com/camunda/camunda-platform-helm/issues/2250)) ([62c9c31](https://github.com/camunda/camunda-platform-helm/commit/62c9c31e3cb9c9bd92208bf65c6cb82ca7715152))
* added helper function smtp auth for webmodeler ([#2245](https://github.com/camunda/camunda-platform-helm/issues/2245)) ([b54fa13](https://github.com/camunda/camunda-platform-helm/commit/b54fa13a1de20e2ae54c143449fcd11dbec85afa))
* correct ingress nginx annotation to activate proxy buffering by default ([#2304](https://github.com/camunda/camunda-platform-helm/issues/2304)) ([1e260e9](https://github.com/camunda/camunda-platform-helm/commit/1e260e9db34c349420237251156575f235d077f2))
* correctly intend operate migration envs ([#2238](https://github.com/camunda/camunda-platform-helm/issues/2238)) ([b795cfe](https://github.com/camunda/camunda-platform-helm/commit/b795cfea0c672b7598250b91621967acb161a0ff))
* enable secrets deprecation flag in alpha by default ([#2081](https://github.com/camunda/camunda-platform-helm/issues/2081)) ([b791f4c](https://github.com/camunda/camunda-platform-helm/commit/b791f4cd6ac3859112b07a89fa6bc89a46d08313))
* **follow-up:** correct existingSecretKey for connectors inbound auth ([712ea6a](https://github.com/camunda/camunda-platform-helm/commit/712ea6a6b387f063e67238321b8a59134d4b2d16))
* gives port-forward hostnames to external urls when no ingress is… ([#1897](https://github.com/camunda/camunda-platform-helm/issues/1897)) ([d28a790](https://github.com/camunda/camunda-platform-helm/commit/d28a7902237340350027fb4709daa3bc278c9d21))
* reload identity when its config changed ([#2234](https://github.com/camunda/camunda-platform-helm/issues/2234)) ([cb41059](https://github.com/camunda/camunda-platform-helm/commit/cb41059630597c4239886dff577c33b8488cb3f8))


### Documentation

* update of outdated url in the local kubernetes  section ([#2274](https://github.com/camunda/camunda-platform-helm/issues/2274)) ([83f8230](https://github.com/camunda/camunda-platform-helm/commit/83f8230d8f5b34d52294e6d3d1be449ffe6aee9c))
