# Contributing

Thanks for taking the time. This doc covers the layout, conventions, and workflow for changing the provider.

## Requirements

- Go **1.25+** (matches `go.mod`'s `go` directive; any newer toolchain works too)
- Terraform **1.8+** on `PATH` (only needed to run the e2e suite)

No Appmixer tenant or other external services are required — every test runs against the in-process mock at `internal/mockserver`.

## Repository layout

```
cmd/mockserver/            # standalone binary that runs the mock on a random port
docs/                      # tfplugindocs output — DO NOT edit by hand; run `make docs`
templates/                 # hand-written doc pages + overrides fed to tfplugindocs
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
5. **Example + doc.** Add `examples/resources/appmixer_<name>/resource.tf` (and `import.sh` if the resource supports import), then run `make docs` to regenerate `docs/resources/<name>.md`. Without an example the generated page has no Example Usage section, so this is not optional.
6. **Full stack.** If the resource is central, add it to `examples/stack/main.tf` so the e2e run exercises it.

## Coding conventions

- **404 on Read → `resp.State.RemoveResource(ctx)`**; 404 on Delete → return without error. Use `client.IsNotFound` for both.
- **Diagnostics** always go through `diagDetail(err)` (package-local alias for `client.DiagDetail`) to avoid leaking response bodies. The error's `SafeMessage()` form is what reaches Terraform.
- **Post-apply state comes from the plan, not the API response.** In `Create` and `Update`, Required and Optional attributes must be written back from `plan` verbatim; only Computed attributes may take a value from the API, and only when the plan left them unknown. Terraform requires the post-apply value of a non-computed attribute to equal its planned value, so any server-side normalization that leaks into state raises `Provider produced inconsistent result after apply`. Report server/config divergence as ordinary drift from `Read` instead. See `applyServerOwned` in `internal/resource/flow.go`.
- **Mock server handlers must not echo request bodies verbatim** where the real API rewrites them. A permissive mock hides the bug class above. `POST`/`PUT /flows` sets every component's version on purpose (`upgradeComponentVersions` in `internal/mockserver/routes_flows.go`), overwriting a pinned one *and* filling in a missing one — a mock that only rewrote existing values would never exercise the field the client omitted. Mirror any comparable rewrite you learn about from the real API.
- **A write that succeeds must leave the resource's id in state**, even when the read-back after it fails. Returning an error without saving the id orphans the object on the server: nothing in state can update or destroy it, and the next apply creates a duplicate. See `Create` in `internal/resource/flow.go`, and `TestAccFlow_createReadBackFailureDoesNotOrphan` for the fault-injection pattern (`Store.FailNextFlowGet`).
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

## Regenerating docs

All of `docs/` is generated from the provider schemas, `examples/`, and `templates/`:

```bash
make docs
```

Always go through `make`, never `go tool tfplugindocs generate` on its own — the target passes `--provider-name` / `--rendered-provider-name`, and without them every page title is rewritten from the repository directory name.

**Regeneration deletes anything that exists only under `docs/`.** Hand-written pages therefore live in `templates/`: `templates/guides/*.md` are copied through verbatim, and a `templates/<page>.md.tmpl` overrides the default layout for that page. Never add a page directly to `docs/` — the next regeneration removes it.

If you change a schema's `MarkdownDescription`, an example's `.tf`, or a guide, regenerate and commit the result in the same PR. `make docs-check` regenerates and fails when the committed output is stale; it runs in CI on every PR, and locally it wants a clean tree (commit first, then check).

## Commit / PR hygiene

- Conventional commit subjects (`feat:`, `fix:`, `chore:`, `refactor:`, `docs:`, `test:`). Existing history follows this.
- One logical change per PR. A resource + its tests + its docs count as one change.
- Update `CHANGELOG.md` under the `## [Unreleased]` section.
- Run the full suite (including e2e) before opening the PR.
