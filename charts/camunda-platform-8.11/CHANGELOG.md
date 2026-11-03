# Changelog

## [16.0.0-alpha1](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.11-16.0.0-alpha0...camunda-platform-8.11-16.0.0-alpha1) (2026-11-03)


### ⚠ BREAKING CHANGES

* render no 8.10 or 8.11 Elasticsearch or OpenSearch URL when none is configured ([#7527](https://github.com/camunda/camunda-platform-helm/issues/7527))

### Features

* add quick install values with embedded H2 [ready] ([#7641](https://github.com/camunda/camunda-platform-helm/issues/7641)) ([6329edc](https://github.com/camunda/camunda-platform-helm/commit/6329edceaf52c000f4f4a5c3ee4a339cc1a823df))
* require Helm CLI v4 for the 8.11 chart and fail early on Helm v3 ([#7657](https://github.com/camunda/camunda-platform-helm/issues/7657)) ([314107e](https://github.com/camunda/camunda-platform-helm/commit/314107e9d60296005646a45ea5e4137b4e873908))


### Bug Fixes

* **8.11:** keep Gateway API redirects on HTTPS for bare Web Modeler, Orchestration and Optimize paths ([#7571](https://github.com/camunda/camunda-platform-helm/issues/7571)) ([f2e2e2b](https://github.com/camunda/camunda-platform-helm/commit/f2e2e2bef933c7016d7d602c452ef1d25ccd2dec))
* **8.9,8.10,8.11:** omit empty elasticsearch username from optimize environment-config [ready] ([#7824](https://github.com/camunda/camunda-platform-helm/issues/7824)) ([75bc0aa](https://github.com/camunda/camunda-platform-helm/commit/75bc0aa0b2fffd597f3594686aec97af1b82de0d))
* align 8.10 and 8.11 deprecation removal targets and policy docs with the Camunda 8 non-public API policy ([#7545](https://github.com/camunda/camunda-platform-helm/issues/7545)) ([e45e611](https://github.com/camunda/camunda-platform-helm/commit/e45e61195082b15e189a04d9184a29ba5376abc5))
* align nginx compatibility shim removal target with the deprecation policy ([#7616](https://github.com/camunda/camunda-platform-helm/issues/7616)) ([8353eb1](https://github.com/camunda/camunda-platform-helm/commit/8353eb106afe8ab1402fa0d34b9bce6377ea78f6))
* deprecate global.image tag and repository in the 8.11 chart ([#7573](https://github.com/camunda/camunda-platform-helm/issues/7573)) ([48e5673](https://github.com/camunda/camunda-platform-helm/commit/48e56739fe9211b791ad661ebabbad5047be33fc))
* honor the env replication factor override in the 8.11 dual-region warning ([#7572](https://github.com/camunda/camunda-platform-helm/issues/7572)) ([481f270](https://github.com/camunda/camunda-platform-helm/commit/481f270e3f06145c8468991bbd38a98f9d763b06))
* name chart v17 for Camunda 8.12 as the strict-values default in the unknown-key warnings [ready] ([#7617](https://github.com/camunda/camunda-platform-helm/issues/7617)) ([5b18b04](https://github.com/camunda/camunda-platform-helm/commit/5b18b04a4a4e54528947536f8d1aa1d5bc1c6cd8))
* reject empty Camunda Hub image tags in the 8.10 and 8.11 schemas ([#7534](https://github.com/camunda/camunda-platform-helm/issues/7534)) ([c0103f4](https://github.com/camunda/camunda-platform-helm/commit/c0103f44e9489ec4ab6396772f4a8ebc78c98c6e))
* render 8.10 and 8.11 RDBMS connection from storage type and warn without a JDBC URL ([#7524](https://github.com/camunda/camunda-platform-helm/issues/7524)) ([a0558dc](https://github.com/camunda/camunda-platform-helm/commit/a0558dc98e462e68e79ea59f156c4a266cb44bf8))
* render no 8.10 or 8.11 Elasticsearch or OpenSearch URL when none is configured ([#7527](https://github.com/camunda/camunda-platform-helm/issues/7527)) ([87d1bdf](https://github.com/camunda/camunda-platform-helm/commit/87d1bdf76af291f7585cd52c85f85c5992bb4151))
* warn when 8.10 and 8.11 Management Identity uses Keycloak without a Keycloak URL ([#7525](https://github.com/camunda/camunda-platform-helm/issues/7525)) ([c8f01b5](https://github.com/camunda/camunda-platform-helm/commit/c8f01b5e4358246c6d350d7496666bdff62bc5bb))
* warn when global.noSecondaryStorage conflicts with orchestration secondary storage type ([#7512](https://github.com/camunda/camunda-platform-helm/issues/7512)) ([8de458a](https://github.com/camunda/camunda-platform-helm/commit/8de458a8fe69fe7560aac5a60fbb26c7864a77e1))


### Documentation

* clarify Camunda Hub auth override descriptions in 8.10 and 8.11 ([#7746](https://github.com/camunda/camunda-platform-helm/issues/7746)) ([dfb2e26](https://github.com/camunda/camunda-platform-helm/commit/dfb2e26de593d4a0aab298e712a4c1d0888ebb2a))


### Dependencies

* update camunda-platform-digests ([#7570](https://github.com/camunda/camunda-platform-helm/issues/7570)) ([62cdc25](https://github.com/camunda/camunda-platform-helm/commit/62cdc25009bac1abf670e260d81ff0303af6d83d))
* update camunda-platform-digests ([#7575](https://github.com/camunda/camunda-platform-helm/issues/7575)) ([3a47d79](https://github.com/camunda/camunda-platform-helm/commit/3a47d7913759247f5acd249d763b8ec1dfc6f261))
* update camunda-platform-digests ([#7576](https://github.com/camunda/camunda-platform-helm/issues/7576)) ([f67ae78](https://github.com/camunda/camunda-platform-helm/commit/f67ae7849238d839c789fa3e7c24a4d2dbf40775))
* update camunda-platform-digests ([#7577](https://github.com/camunda/camunda-platform-helm/issues/7577)) ([740835e](https://github.com/camunda/camunda-platform-helm/commit/740835e6698e2e8ccfc9f1d75028ac0a660b8f9c))
* update camunda-platform-digests ([#7590](https://github.com/camunda/camunda-platform-helm/issues/7590)) ([fa353f2](https://github.com/camunda/camunda-platform-helm/commit/fa353f23defc15e5b1cd6e54530535b720b93f4a))
* update camunda-platform-digests ([#7593](https://github.com/camunda/camunda-platform-helm/issues/7593)) ([e680ec7](https://github.com/camunda/camunda-platform-helm/commit/e680ec731e16d9f05da105c78b2e76e5fd4439de))
* update camunda-platform-digests ([#7594](https://github.com/camunda/camunda-platform-helm/issues/7594)) ([87a90bc](https://github.com/camunda/camunda-platform-helm/commit/87a90bc4079e2968ec20234593d5f510ba19c6e8))
* update camunda-platform-digests ([#7603](https://github.com/camunda/camunda-platform-helm/issues/7603)) ([7c91a0c](https://github.com/camunda/camunda-platform-helm/commit/7c91a0c8d3d92e197792df141dcd85d9b504635e))
* update camunda-platform-digests ([#7604](https://github.com/camunda/camunda-platform-helm/issues/7604)) ([4ecf279](https://github.com/camunda/camunda-platform-helm/commit/4ecf2794214e5006f4d2d4fa3576b3752a6cdda1))
* update camunda-platform-digests ([#7747](https://github.com/camunda/camunda-platform-helm/issues/7747)) ([66b8f3c](https://github.com/camunda/camunda-platform-helm/commit/66b8f3ceac5975d8e9f93c11682e2c346724c643))
* update camunda-platform-digests ([#7776](https://github.com/camunda/camunda-platform-helm/issues/7776)) ([a15f03a](https://github.com/camunda/camunda-platform-helm/commit/a15f03a5570c5050e27d737d544b36823a93bd2b))
* update camunda-platform-digests ([#7777](https://github.com/camunda/camunda-platform-helm/issues/7777)) ([c327b4e](https://github.com/camunda/camunda-platform-helm/commit/c327b4e6887a179d1d4ae518d58b292863bf4f9e))
* update camunda-platform-digests ([#7778](https://github.com/camunda/camunda-platform-helm/issues/7778)) ([4b099f1](https://github.com/camunda/camunda-platform-helm/commit/4b099f14ec6e5a6c494ee03ac40cc52e115cffc4))
* update camunda-platform-digests ([#7780](https://github.com/camunda/camunda-platform-helm/issues/7780)) ([a66aa99](https://github.com/camunda/camunda-platform-helm/commit/a66aa99984d33a894d7bce445130e5424ed9c4a8))
* update camunda-platform-digests ([#7781](https://github.com/camunda/camunda-platform-helm/issues/7781)) ([d3353e4](https://github.com/camunda/camunda-platform-helm/commit/d3353e4ca180260ec1813cd21414c8a2757aeab3))
* update camunda-platform-digests ([#7782](https://github.com/camunda/camunda-platform-helm/issues/7782)) ([169f68a](https://github.com/camunda/camunda-platform-helm/commit/169f68af98ce25b3d0005c65d6a007f1edf11b42))
* update camunda-platform-digests ([#7783](https://github.com/camunda/camunda-platform-helm/issues/7783)) ([643adc0](https://github.com/camunda/camunda-platform-helm/commit/643adc0c18864cc7465e863ff06a3c195d589085))
* update camunda-platform-digests ([#7799](https://github.com/camunda/camunda-platform-helm/issues/7799)) ([8232205](https://github.com/camunda/camunda-platform-helm/commit/82322059768e53cca864cfe86b656eb857d75205))
* update camunda-platform-digests ([#7802](https://github.com/camunda/camunda-platform-helm/issues/7802)) ([7b336b7](https://github.com/camunda/camunda-platform-helm/commit/7b336b7561cae2bf77e89fcefa9733adf0bba75e))
* update camunda-platform-digests ([#7803](https://github.com/camunda/camunda-platform-helm/issues/7803)) ([3ac7243](https://github.com/camunda/camunda-platform-helm/commit/3ac72431f6f24a11340ea683e2107e6a3df8be6b))
* update camunda-platform-digests ([#7807](https://github.com/camunda/camunda-platform-helm/issues/7807)) ([a6d978a](https://github.com/camunda/camunda-platform-helm/commit/a6d978ad4c197662f9283ea795d0bcebc829f87d))
* update camunda-platform-digests ([#7809](https://github.com/camunda/camunda-platform-helm/issues/7809)) ([863192f](https://github.com/camunda/camunda-platform-helm/commit/863192f17a89ec625c92907dba4756359c94f2d9))
* update camunda-platform-digests ([#7810](https://github.com/camunda/camunda-platform-helm/issues/7810)) ([cc8827e](https://github.com/camunda/camunda-platform-helm/commit/cc8827e5160e980c7b7d07a0759027f3cee069a4))
* update camunda-platform-digests ([#7811](https://github.com/camunda/camunda-platform-helm/issues/7811)) ([3d712ee](https://github.com/camunda/camunda-platform-helm/commit/3d712ee4db59a6a32ccc32a4de622e509a9f2ea9))
* update camunda-platform-digests ([#7813](https://github.com/camunda/camunda-platform-helm/issues/7813)) ([91ea336](https://github.com/camunda/camunda-platform-helm/commit/91ea336429472b0d71d32ba172c58c51c300a8e0))
* update camunda-platform-digests ([#7816](https://github.com/camunda/camunda-platform-helm/issues/7816)) ([6e17da9](https://github.com/camunda/camunda-platform-helm/commit/6e17da905ac31db48eb6adcdaf3497ea7e5e5c79))
* update camunda-platform-digests ([#7819](https://github.com/camunda/camunda-platform-helm/issues/7819)) ([4f74c81](https://github.com/camunda/camunda-platform-helm/commit/4f74c81889034a292ea3df464b262d55ece884cb))
* update camunda-platform-digests ([#7822](https://github.com/camunda/camunda-platform-helm/issues/7822)) ([6ac596f](https://github.com/camunda/camunda-platform-helm/commit/6ac596f05920a57705927615e2ca99e7b268f095))
* update camunda-platform-digests ([#7826](https://github.com/camunda/camunda-platform-helm/issues/7826)) ([27c03b7](https://github.com/camunda/camunda-platform-helm/commit/27c03b7b01a40aec0a176eb9512671c8526be45a))
* update camunda-platform-digests ([#7828](https://github.com/camunda/camunda-platform-helm/issues/7828)) ([78db731](https://github.com/camunda/camunda-platform-helm/commit/78db7319c1bd8486f21d28750928863cd70ed702))
* update camunda-platform-digests ([#7829](https://github.com/camunda/camunda-platform-helm/issues/7829)) ([2c70483](https://github.com/camunda/camunda-platform-helm/commit/2c7048315049944347a6a07ccd2d4514a57a0196))
* update camunda/camunda:8.11-snapshot docker digest to e13a61f ([#7792](https://github.com/camunda/camunda-platform-helm/issues/7792)) ([e38d2cb](https://github.com/camunda/camunda-platform-helm/commit/e38d2cb392856378bcebd5d7e134d68719bfa539))
* update camunda/connectors-bundle:8.11-snapshot docker digest to 5259da2 ([#7801](https://github.com/camunda/camunda-platform-helm/issues/7801)) ([3c8fbb5](https://github.com/camunda/camunda-platform-helm/commit/3c8fbb5a87588785b6b8840654fcdfff73b9fed8))
* update camunda/connectors-bundle:8.11-snapshot docker digest to 6dcbd3a ([#7595](https://github.com/camunda/camunda-platform-helm/issues/7595)) ([8121426](https://github.com/camunda/camunda-platform-helm/commit/8121426d9af9fd45cd6687cfdd9614a922f046b4))
* update camunda/hub-websockets:snapshot docker digest to 19d97bc ([#7794](https://github.com/camunda/camunda-platform-helm/issues/7794)) ([66b6c26](https://github.com/camunda/camunda-platform-helm/commit/66b6c260b43cc160435508c353894ab2036a143e))
* update camunda/hub-websockets:snapshot docker digest to 36759ca ([#7785](https://github.com/camunda/camunda-platform-helm/issues/7785)) ([fec6ad8](https://github.com/camunda/camunda-platform-helm/commit/fec6ad808bc78203986d841eb05949faa6cde1f5))
* update camunda/hub:snapshot docker digest to 14ab7ed ([#7808](https://github.com/camunda/camunda-platform-helm/issues/7808)) ([58dffa0](https://github.com/camunda/camunda-platform-helm/commit/58dffa0304b6ee5835ea5f7ac2a82a8c75642227))
* update camunda/hub:snapshot docker digest to 241aeae ([#7798](https://github.com/camunda/camunda-platform-helm/issues/7798)) ([ac0cbdd](https://github.com/camunda/camunda-platform-helm/commit/ac0cbddb59593a85d1593443914fa3d583ce1828))
* update camunda/hub:snapshot docker digest to 4f38cd2 ([#7797](https://github.com/camunda/camunda-platform-helm/issues/7797)) ([1fb807f](https://github.com/camunda/camunda-platform-helm/commit/1fb807f75fd4fce60c916b5b1d32234e67a9ac7f))
* update camunda/hub:snapshot docker digest to 7a385d8 ([#7796](https://github.com/camunda/camunda-platform-helm/issues/7796)) ([db64b39](https://github.com/camunda/camunda-platform-helm/commit/db64b39b0ad6ac445c20831adcc1986059c4f224))
* update camunda/hub:snapshot docker digest to f7d7a98 ([#7805](https://github.com/camunda/camunda-platform-helm/issues/7805)) ([117e8e5](https://github.com/camunda/camunda-platform-helm/commit/117e8e5efadef2f78d7e5da0c62217394fd5ff40))
* update camunda/identity:snapshot docker digest to da03664 ([#7804](https://github.com/camunda/camunda-platform-helm/issues/7804)) ([1a20107](https://github.com/camunda/camunda-platform-helm/commit/1a201073b247a130fa8a3ee495654df88769f8a8))
* update camunda/identity:snapshot docker digest to da24344 ([#7580](https://github.com/camunda/camunda-platform-helm/issues/7580)) ([b711486](https://github.com/camunda/camunda-platform-helm/commit/b7114860bb7afdead6b61a620dfc4e291ed01011))
* update camunda/identity:snapshot docker digest to db32906 ([#7788](https://github.com/camunda/camunda-platform-helm/issues/7788)) ([0e65297](https://github.com/camunda/camunda-platform-helm/commit/0e652971c88edd940e5fd5cc19963170817dcd6e))
* update camunda/optimize:8.11-snapshot docker digest to 6dd01e2 ([#7793](https://github.com/camunda/camunda-platform-helm/issues/7793)) ([7b7f095](https://github.com/camunda/camunda-platform-helm/commit/7b7f09528f6344f3c14efa261ddb820f13f6213d))
* update camunda/optimize:8.11-snapshot docker digest to 7cd07eb ([#7812](https://github.com/camunda/camunda-platform-helm/issues/7812)) ([9132ab0](https://github.com/camunda/camunda-platform-helm/commit/9132ab0255a69426e660778ae99ee1544e6c2687))
* update camunda/optimize:8.11-snapshot docker digest to 811a2cd ([#7806](https://github.com/camunda/camunda-platform-helm/issues/7806)) ([b1a7c93](https://github.com/camunda/camunda-platform-helm/commit/b1a7c93407d16142e9f1f043c95f5e1f8360bfb0))
* update patch-updates (patch) ([#7618](https://github.com/camunda/camunda-platform-helm/issues/7618)) ([c565042](https://github.com/camunda/camunda-platform-helm/commit/c56504296275003045f3a448bde080eedcde2d3d))
* upgrade golang version to 1.27.3 ([#7675](https://github.com/camunda/camunda-platform-helm/issues/7675)) ([89ecc30](https://github.com/camunda/camunda-platform-helm/commit/89ecc30da8aed16f5ea7813308dc3f698d6ef35d))


### Refactors

* bound 8.11 warning helpers and reuse TLS scanning ([#7756](https://github.com/camunda/camunda-platform-helm/issues/7756)) ([848fb2f](https://github.com/camunda/camunda-platform-helm/commit/848fb2f5206b8b7fc449ca1a485611b3724d84b1))

## Changelog
