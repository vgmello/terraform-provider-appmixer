# Changelog

All notable changes to this project are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and
this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.0.1-rc.1] — 2026-04-24

### Added

- `appmixer_acl` gained a `mode` attribute:
  - `authoritative` (default, existing behavior) — the resource owns the entire list; apply replaces, destroy empties.
  - `merge` — the resource owns only the rules it declares; externally-configured rules are preserved across apply and destroy; rules dropped from HCL between applies are removed from the server.
- In-place token / `profile_info` rotation for `appmixer_account` via a `POST /accounts` upsert keyed by `(service, display_name)`; the server-assigned `id` is preserved, so credential rotation no longer destroys the account.
- `appmixer_quota` resource for managing custom quota rules (`PUT /quota/{name}`, list-based refresh, `DELETE /quota/{name}`).
- End-to-end test suite under `e2e/` (build tag `e2e`) that drives the real `terraform` CLI against the in-process mock.
- Standalone `cmd/mockserver` binary for manual stack runs.
- Full example stack at `examples/stack/` exercising every resource and both data sources.
- `CONTRIBUTING.md` with layout, conventions, and resource-addition workflow.
- `client.DiagDetail` and `client.IsNotFound` helpers to centralize error handling.
- In-place password rotation for `appmixer_user` via `POST /user/reset-password`; the user is no longer destroyed on password change.

### Changed

- **Breaking — `appmixer_service_config` attribute rename.** `fields` → `items` and `sensitive_fields` → `sensitive_items`. Existing HCL and state must be updated. The old names are no longer recognized.
- `appmixer_service_config` now defaults **unknown keys to `sensitive_items`** on `Read` (previously defaulted to `items`). This keeps imports safe: after `terraform import`, every key lands in `sensitive_items` (redacted in plan output), and the operator promotes non-secrets into `items` in HCL. The first apply shows the partition as drift, which the apply reconciles.
- `appmixer_service_config` now writes `MapNull` for empty `items` / `sensitive_items`, eliminating a perpetual diff against a `null` config.
- `client.New` uses a dedicated `*http.Client` with a 60-second timeout instead of `http.DefaultClient`, and trims trailing slashes on `base_url`.
- Mock server's `POST /service-config` now upserts by `serviceId` (matches real API semantics).
- Mock server's `POST /accounts` now upserts when `(service, displayName)` matches an existing row, preserving its `accountId`.

### Fixed

- Every resource's `Delete` now treats a 404 as success, so `terraform destroy` no longer dangles state after an out-of-band deletion.
- `appmixer_acl` `Read` now removes the resource from state on 404 instead of hard-erroring.
- `appmixer_user` data source now returns an explicit "User not found" diagnostic on 404, matching the flow data source.
- `appmixer_flow` data source's `custom_fields` is now `null` when empty, matching the resource's representation (prevents null-vs-empty-map drift when piping between them).
- `appmixer_service_config` `Read` no longer silently drops `ElementsAs` diagnostics from the prior `sensitive_items` state — corrupt state surfaces as an error instead of demoting secrets to `items`.

## [0.1.0] — Initial development

Foundation work: provider wiring, HTTP client + admin auth, in-process Fiber mock
server, acceptance test harness, and the first batch of resources
(`appmixer_system_config`, `appmixer_service_config`, `appmixer_acl`,
`appmixer_modifiers`, `appmixer_flow`, `appmixer_account`, `appmixer_user`) plus
the `appmixer_user` and `appmixer_flow` data sources.

[Unreleased]: https://github.com/vgmello/terraform-provider-appmixer/compare/v0.0.1-rc.1...HEAD
[0.0.1-rc.1]: https://github.com/vgmello/terraform-provider-appmixer/compare/v0.1.0...v0.0.1-rc.1
[0.1.0]: https://github.com/vgmello/terraform-provider-appmixer/releases/tag/v0.1.0
