# Contributing

Thanks for taking the time. This doc covers the layout, conventions, and workflow for changing the provider.

## Requirements

- Go **1.25+** (matches `go.mod`'s `go` directive; any newer toolchain works too)
- Terraform **1.8+** on `PATH` (only needed to run the e2e suite)

No Appmixer tenant or other external services are required — every test runs against the in-process mock at `internal/mockserver`.

## Repository layout

```
cmd/mockserver/            # standalone binary that runs the mock on a random port
docs/                      # tfplugindocs output — DO NOT edit by hand; regenerate
e2e/                       # end-to-end test (build tag `e2e`) driving terraform CLI
examples/                  # HCL examples; per-resource + full `stack` walkthrough
internal/acctest/          # acceptance-test helper (spawns the mock, sets env)
internal/client/           # HTTP client, auth, error shaping
internal/datasource/       # data-source implementations
internal/mockserver/       # Fiber-based mock of the Appmixer API
internal/provider/         # provider wiring (Configure, Resources, DataSources)
internal/resource/         # resource implementations + acc tests
main.go                    # provider server entrypoint
```

## Workflow: adding a resource

1. **Mock first.** Add routes under `internal/mockserver/routes_<name>.go` and a typed bucket on `Store` in `internal/mockserver/server.go`. Include a small seed in `newStore()` so the API has something to return before the first create.
2. **Implement the resource** in `internal/resource/<name>.go` — CRUD, `ImportState`, and a proper schema. Use `client.IsNotFound` for 404 handling on Read/Delete; surface errors via `diagDetail(err)`.
3. **Register it** in `internal/provider/provider.go` under `Resources()`.
4. **Write acceptance tests** in `internal/resource/<name>_test.go`. Cover: basic create, update-in-place (or replace), import.
5. **Example + doc.** Add `examples/resources/appmixer_<name>/resource.tf` and run `make docs` (see below) to regenerate `docs/resources/<name>.md`.
6. **Full stack.** If the resource is central, add it to `examples/stack/main.tf` so the e2e run exercises it.

## Coding conventions

- **404 on Read → `resp.State.RemoveResource(ctx)`**; 404 on Delete → return without error. Use `client.IsNotFound` for both.
- **Diagnostics** always go through `diagDetail(err)` (package-local alias for `client.DiagDetail`) to avoid leaking response bodies. The error's `SafeMessage()` form is what reaches Terraform.
- **Null vs empty maps:** Optional-not-Computed map attributes must write `types.MapNull(...)` when the server returns an empty map — an empty `MapValue` causes a perpetual diff against a `null` config. See `internal/resource/service_config.go` for the pattern.
- **No `*APIError` plumbing outside `client`**. Consumers call `client.IsNotFound(err)` or `client.DiagDetail(err)` instead of type-asserting directly.
- **RequiresReplace for fields the API cannot update.** Don't silently drop changes.
- **Comments:** only when the _why_ is non-obvious. Don't restate the code.

## Running tests

```bash
# Fast unit + acceptance suite (mock-backed, ~25s)
go test ./...

# End-to-end (builds the provider, drives terraform CLI, ~5s after build)
go test -tags e2e -v ./e2e/...
```

The acceptance tests auto-set `TF_ACC=1`; no environment prep needed.

## Regenerating per-resource docs

`docs/` is generated from schemas + `examples/`. Use the Go tool pinned in `go.mod`:

```bash
go tool tfplugindocs generate
```

If you change a schema's `MarkdownDescription` or an example's `.tf`, regenerate and commit the diff in the same PR.

## Commit / PR hygiene

- Conventional commit subjects (`feat:`, `fix:`, `chore:`, `refactor:`, `docs:`, `test:`). Existing history follows this.
- One logical change per PR. A resource + its tests + its docs count as one change.
- Update `CHANGELOG.md` under the `## [Unreleased]` section.
- Run the full suite (including e2e) before opening the PR.
