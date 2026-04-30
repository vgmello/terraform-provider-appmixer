# Changelog

All notable changes to this project are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and
this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- `appmixer_account`: renamed `display_name` to `name` — the stable identity key (alongside `service`) that forces replacement when changed. `display_name` is now a separate **optional** attribute for the human-readable label shown in the Appmixer UI, updatable in-place without recreation.
- Mock server: upsert key for `POST /accounts` changed from `(service, displayName)` to `(service, name)`. `displayName` is now a separately updatable label field.

## [0.0.1] — 2026-04-24

First release candidate for the Appmixer Terraform Provider.

Provider wiring, HTTP client with admin auth, in-process Fiber mock server,
acceptance test harness, and the first batch of resources plus data sources.

### Added

- `appmixer_system_config`, `appmixer_service_config`, `appmixer_acl`, `appmixer_modifiers`, `appmixer_flow`, `appmixer_account`, `appmixer_user` resources.
- `appmixer_user` and `appmixer_flow` data sources.
- `appmixer_quota` resource for managing custom quota rules (`PUT /quota/{name}`, list-based refresh, `DELETE /quota/{name}`).
- `appmixer_acl` `mode` attribute: `authoritative` (default) and `merge`.
- In-place token / `profile_info` rotation for `appmixer_account` via upsert keyed on `(service, name)`.
- In-place password rotation for `appmixer_user` via `POST /user/reset-password`.
- End-to-end test suite under `e2e/` (build tag `e2e`) that drives the real `terraform` CLI against the in-process mock.
- Standalone `cmd/mockserver` binary for manual stack runs.
- Full example stack at `examples/stack/` exercising every resource and both data sources.
- `CONTRIBUTING.md` with layout, conventions, and resource-addition workflow.
- `client.DiagDetail` and `client.IsNotFound` helpers to centralize error handling.

[Unreleased]: https://github.com/vgmello/terraform-provider-appmixer/compare/v0.0.1...HEAD
[0.0.1]: https://github.com/vgmello/terraform-provider-appmixer/releases/tag/v0.0.1
