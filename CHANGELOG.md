# Changelog

## [0.5.0](https://github.com/praxis-os/praxis-forge/compare/v0.3.0...v0.5.0) (2026-04-20)


### Added

* **build:** stamp NormalizedHash and Capabilities onto Manifest ([d4edfaa](https://github.com/praxis-os/praxis-forge/commit/d4edfaa5efadf9f0fb6dd32e358451233c5a2fa5))
* **manifest:** NormalizedHash + Capabilities fields ([1e097c9](https://github.com/praxis-os/praxis-forge/commit/1e097c960e5516d5fbc073f680a6c056e63dade5))
* **phase-2b:** canonical JSON, stable hash, capabilities ([d38f214](https://github.com/praxis-os/praxis-forge/commit/d38f214245c4b7e2c359174eaf9688efc111db53))
* **spec:** add JSON tags to AgentSpec for canonical serialization ([2a1b505](https://github.com/praxis-os/praxis-forge/commit/2a1b505adb35c53b096f092dad30b58c171334a7))
* **spec:** canonical JSON encoder for NormalizedSpec ([353bcdd](https://github.com/praxis-os/praxis-forge/commit/353bcdd24c757ee2fe6a87feafe7d6a7bc585b8d))
* **spec:** stable SHA-256 hash accessor on NormalizedSpec ([e8fe7a2](https://github.com/praxis-os/praxis-forge/commit/e8fe7a2bfc4d3839d37384010850ac254b6c0c8a))


### Documentation

* Phase 2b design spec and forge-overview amendment ([9f692e6](https://github.com/praxis-os/praxis-forge/commit/9f692e6b1d7de7a371610a4e3419329f2d755275))


### Testing

* **spec:** determinism fixture with permuted map order ([384bd76](https://github.com/praxis-os/praxis-forge/commit/384bd76ac3ea3ea19f7179c7f22992d0d393cac9))

## [0.3.0](https://github.com/praxis-os/praxis-forge/compare/v0.1.0...v0.3.0) (2026-04-19)


### Added

* **build,manifest:** resolver, chain adapters, tool router, Build assembler ([2668984](https://github.com/praxis-os/praxis-forge/commit/26689844a4ee0088c2a621828adfbf9169ab0351))
* **factories:** 11 concrete factories for Phase 1 vertical slice ([366599b](https://github.com/praxis-os/praxis-forge/commit/366599b24742f9c39bc7de7e6bedfc7099a5033a))
* forge facade, fakeprovider, integration test, demo, CI ([8b2802d](https://github.com/praxis-os/praxis-forge/commit/8b2802dd2af380126236695cef242d0ca72d61ee))
* **forge:** NormalizedSpec accessor + LoadOverlays helper + integration test for extends + overlays ([4b3806e](https://github.com/praxis-os/praxis-forge/commit/4b3806e08dd0f4c1c8aeedbf9785179e58fa27a4))
* **forge:** WithOverlays + WithSpecStore options, wire Normalize ([d927ad8](https://github.com/praxis-os/praxis-forge/commit/d927ad888b40382b5e0a13324935ad77c3fc8855))
* **manifest:** ExtendsChain + Overlays attribution ([61c5e95](https://github.com/praxis-os/praxis-forge/commit/61c5e95e94c6658dea85b5dcc1acd7b0e7ace80a))
* **Phase 2a:** composition depth — extends, overlays, provenance, locked fields ([8d7b2e3](https://github.com/praxis-os/praxis-forge/commit/8d7b2e305ed373b7193da9d193e95418814c2bf8))
* **registry:** ComponentRegistry with 11 typed factory kinds ([9143786](https://github.com/praxis-os/praxis-forge/commit/91437864738c805895650acfc4a3817e7e747048))
* **spec:** AgentOverlay + RefList tri-state + LoadOverlay ([3ca3f8f](https://github.com/praxis-os/praxis-forge/commit/3ca3f8fbb57fae44e839c6adbae93f0843b17eea))
* **spec:** extends chain resolver ([3b90b37](https://github.com/praxis-os/praxis-forge/commit/3b90b3779212a6dd806127f27e50f30137d0106a))
* **spec:** overlay merger + locked-field validation + Normalize entry point ([8150d74](https://github.com/praxis-os/praxis-forge/commit/8150d7412d66cac3666377b27a06459a4dfb3020))
* **spec:** provenance type + NormalizedSpec wrapper ([fd77c8c](https://github.com/praxis-os/praxis-forge/commit/fd77c8c34d3f09d069f066954cb009264211ae54))
* **spec:** SpecStore interface + filesystem and map impls ([6193364](https://github.com/praxis-os/praxis-forge/commit/6193364bfad65ac3823af5e07676217f01c91eb1))
* **spec:** types, strict YAML loader, validator, and test fixtures ([b03564a](https://github.com/praxis-os/praxis-forge/commit/b03564a8d5e8dbba829fb98bc7ad95a15642d39a))


### Fixed

* **ci:** drop include scope from dependabot commit-message config ([99a327d](https://github.com/praxis-os/praxis-forge/commit/99a327da5e133ca6909de5b311be561d003a7619))
* **ci:** drop include scope from dependabot commit-message config ([2152ade](https://github.com/praxis-os/praxis-forge/commit/2152ade0c6ea62262de20cc99ba1cd370adafc34))
* **ci:** drop parens from dependabot prefix to avoid double-scope ([3d0e362](https://github.com/praxis-os/praxis-forge/commit/3d0e36210a51801700895d6e2c32d625ae20b8a1))
* **ci:** drop parens from dependabot prefix to avoid double-scope ([2dbe1a8](https://github.com/praxis-os/praxis-forge/commit/2dbe1a882a19473dc5cd888ebb30d09385b8cb7e))
* **spec:** extract role string constants for goconst compliance ([2871f06](https://github.com/praxis-os/praxis-forge/commit/2871f0619c5d50163d8c1e1da3e7019bde1d5b21))


### Documentation

* ADR 0003 — forge owns no memory across short/medium/long ([f7d65aa](https://github.com/praxis-os/praxis-forge/commit/f7d65aa9f2569635086edca41b21cf2b39c1577d))
* amend Phase 0 designs — drop ConfigSchema, add prompt_asset, defer overlays ([897ee2b](https://github.com/praxis-os/praxis-forge/commit/897ee2b9785091c8dbd8db666300d99ad833381b))
* amend Phase 0/1 design docs for Phase 2a ([e0689b6](https://github.com/praxis-os/praxis-forge/commit/e0689b61e72a2465d273a5a1148f5d9f6f469b8b))
* Phase 0 design — default components + external registries ([cd07d8d](https://github.com/praxis-os/praxis-forge/commit/cd07d8d6ad074397d35bf0b6d22dd2d2ba8948e9))
* Phase 1 implementation plan (40+ TDD tasks) ([e7eee2a](https://github.com/praxis-os/praxis-forge/commit/e7eee2aeeef09a77c559abe25c196fdede4733ae))
* Phase 2a (composition depth) design spec ([eec6dab](https://github.com/praxis-os/praxis-forge/commit/eec6dabb0bbef353c93f8815488c8b8a0741172d))
* Phase 2a implementation plan (9 task groups, ~4500 lines) ([32ea7bd](https://github.com/praxis-os/praxis-forge/commit/32ea7bd326db45a7e6cf2d0037a47a9ec7f2e2df))
* split Phase 1 implementation plan into per-task-group files ([47956f9](https://github.com/praxis-os/praxis-forge/commit/47956f94f87da856dff3da8e48dbdb79403c3293))

## Changelog

All notable changes to praxis-forge are documented here. This file is
maintained automatically by
[release-please](https://github.com/googleapis/release-please) from
Conventional Commits on `main`. Do not hand-edit the release sections.

## Unreleased
