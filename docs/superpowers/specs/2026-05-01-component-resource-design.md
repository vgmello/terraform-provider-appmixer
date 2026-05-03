# Component Resource Design

## Overview

Add an `appmixer_component` resource that publishes, tracks, and deletes Appmixer components/modules/services from a local zip archive.

## HCL Usage

```hcl
resource "appmixer_component" "my_service" {
  selector    = "appmixer.myservice"
  source      = "path/to/appmixer.myservice.zip"
  replace_all = false  # optional, default false
}
```

## Resource Schema

| Attribute      | Type   | Required | Description                                                                 |
|----------------|--------|----------|-----------------------------------------------------------------------------|
| `selector`     | string | yes      | Dot-separated identifier (e.g. `appmixer.myservice`). ForceNew.            |
| `source`       | string | yes      | Path to the zip file on disk.                                               |
| `replace_all`  | bool   | optional | Maps to `?replaceAll=true` query param. Default `false`.                    |
| `file_hash`    | string | computed | SHA256 of the zip file, recalculated at plan time via a custom plan modifier. A change triggers an update. |
| `published_at` | string | computed | Timestamp from the `finished` field of the upload ticket response.          |

The `selector` doubles as the resource `id` — no server-generated ID is needed.

## API Endpoints

| Operation         | Method   | Endpoint                              | Notes                                    |
|-------------------|----------|---------------------------------------|------------------------------------------|
| Publish           | POST     | `/components?replaceAll={replace_all}` | Body: zip bytes, `application/octet-stream`. Returns `{"ticket":"..."}`. |
| Poll progress     | GET      | `/components/uploader/:ticket`         | Poll until `finished` is present.        |
| Read (existence)  | GET      | `/apps/components?app={selector}`      | Confirms component exists on server.     |
| Delete            | DELETE   | `/components/:selector`                | Removes component/module/service.        |

## Client Layer

### New method: `PostBinary`

The existing `client.do()` always JSON-encodes the body. A new `PostBinary` method sends raw bytes with `Content-Type: application/octet-stream`.

### New types

```go
type PublishResponse struct {
    Ticket string `json:"ticket"`
}

type UploadStatus struct {
    Finished string `json:"finished"`
    Err      string `json:"err"`
    Data     []any  `json:"data"`
}
```

## Resource Lifecycle

### Create & Update

1. Read the zip file from `source`, compute SHA256 for `file_hash`.
2. `POST /components?replaceAll={replace_all}` with zip bytes as `application/octet-stream`.
3. Receive `{"ticket":"..."}`.
4. Poll `GET /components/uploader/:ticket` every 2 seconds until `finished` is present or 5-minute timeout is reached.
5. If the response contains `err`, fail with a diagnostic showing the error and validation details.
6. On success, store `file_hash`, `published_at`, and `selector` as `id` in state.

### Read

1. `GET /apps/components?app={selector}`.
2. If 404 or empty → remove from state (deleted out-of-band).
3. Otherwise, confirm existence. `file_hash` and `published_at` are tracked locally, not refreshed from server.

### Delete

1. `DELETE /components/{selector}`.
2. Remove from state.

### Import

- `terraform import appmixer_component.x "appmixer.myservice"` — import ID is the selector.
- Read confirms existence, sets `selector` and `id`.
- `file_hash`, `published_at`, and `source` are unknown after import — first plan shows a diff requiring `source`, triggering a re-publish.

## Plan Modifier

A custom `planmodifier.String` on `file_hash`:

- During plan, reads the file at `source` and computes SHA256.
- Compares to state — if different, marks `file_hash` as changed (triggers update).
- If `source` changed, also recomputes.
- If the file doesn't exist at plan time, produces a diagnostic error.

## Mock Server

New file: `internal/mockserver/routes_components.go`.

New store fields:

```go
Components map[string]map[string]any  // keyed by selector
Tickets    map[string]map[string]any  // keyed by ticket ID
```

Routes:

- `POST /components` — stores upload, generates ticket, marks finished immediately.
- `GET /components/uploader/:ticket` — returns ticket status.
- `GET /apps/components` — returns components matching `app` query param.
- `DELETE /components/:selector` — removes from store.

Publishes succeed instantly in the mock (no async delay). Error-path tests can seed a ticket with an `err` field.

## Polling

- Interval: 2 seconds.
- Timeout: 5 minutes, hardcoded. No user-configurable attribute for now.
- On timeout: fail with a diagnostic.
- On `err` in response: fail with the error message and validation data.

## Files to Create/Modify

| File                                          | Action  | Purpose                                    |
|-----------------------------------------------|---------|---------------------------------------------|
| `internal/client/client.go`                   | Modify  | Add `PostBinary` method                     |
| `internal/resource/component.go`              | Create  | Resource implementation + plan modifier     |
| `internal/resource/component_test.go`         | Create  | Acceptance tests                            |
| `internal/mockserver/routes_components.go`    | Create  | Mock API routes                             |
| `internal/mockserver/server.go`               | Modify  | Add store fields + register routes          |
| `internal/provider/provider.go`               | Modify  | Register `NewComponentResource`             |
| `docs/resources/component.md`                 | Create  | Resource documentation                      |
| `examples/resources/appmixer_component/`      | Create  | Example HCL                                 |
