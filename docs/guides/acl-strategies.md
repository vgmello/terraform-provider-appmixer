---
page_title: "ACL Strategies - appmixer Provider"
description: |-
  Understand the two ACL ownership modes — authoritative and merge — and learn
  when to use each one.
---

# ACL Strategies

The `appmixer_acl` resource manages ACL rules for a given `type` (e.g. `components`, `flows`).
A key design decision is choosing the `mode`: **authoritative** or **merge**.

## The Two Modes

### Authoritative (default)

```terraform
resource "appmixer_acl" "components" {
  type = "components"
  mode = "authoritative"   # or omit — this is the default

  rules = [
    {
      role       = "admin"
      resource   = "*"
      action     = ["read", "write", "delete"]
      attributes = ["*"]
    },
  ]
}
```

**Behavior:**
- `apply` — replaces the entire server-side rule list with the rules declared here
- `destroy` — resets the list to empty (does **not** restore tenant defaults)
- Any rules added out-of-band (via the Appmixer UI or API) are deleted on the next `apply`

**Use authoritative when** you want Terraform to be the single source of truth for an entire ACL type. This is the safest choice for greenfield deployments or when the full list is small and known.

### Merge

```terraform
resource "appmixer_acl" "extra_rules" {
  type = "components"
  mode = "merge"

  rules = [
    {
      role       = "partner"
      resource   = "appmixer.services.partner.*"
      action     = ["read"]
      attributes = ["*"]
    },
  ]
}
```

**Behavior:**
- `apply` — adds or updates only the rules declared here; other rules on the server are left untouched
- `destroy` — removes only the rules declared here; other rules on the server are left untouched
- Rules removed from `rules` between applies are deleted; externally-managed rules are preserved

**Use merge when:**
- Rules are partly managed by another system (e.g. the Appmixer admin UI)
- Multiple Terraform modules or workspaces each own a subset of the rules for the same type
- You want to layer rules on top of tenant defaults without overwriting them

## Mixing Both Modes on the Same Type

You can have both an authoritative resource and one or more merge resources targeting the same `type`, as long as the rule sets do not overlap. However, the authoritative resource will wipe any rules not in its own list on `apply`, which will conflict with rules managed by merge resources.

**Recommended practice:** use one mode per `type`. If you need the flexibility of merge, use merge throughout.

## Multiple Merge Resources on the Same Type

This is the recommended pattern when different teams or modules manage different rule subsets:

```terraform
# Module: team-platform owns platform component rules
resource "appmixer_acl" "platform_rules" {
  type = "components"
  mode = "merge"

  rules = [
    {
      role       = "platform-admin"
      resource   = "*"
      action     = ["read", "write", "delete"]
      attributes = ["*"]
    },
  ]
}

# Module: team-partners owns partner-facing rules
resource "appmixer_acl" "partner_rules" {
  type = "components"
  mode = "merge"

  rules = [
    {
      role       = "partner"
      resource   = "appmixer.services.partner.*"
      action     = ["read"]
      attributes = ["*"]
    },
  ]
}
```

Each resource only touches the rules it declares. Destroying `partner_rules` leaves `platform_rules` intact.

## Wildcard Syntax Reference

| Pattern | Matches |
|---------|---------|
| `*` | All resources or all actions |
| `appmixer.services.*` | All services under the `appmixer.services` namespace |
| `non-private` | All non-private resources (special keyword) |
| `["*"]` | All actions (used in the `action` list) |

## Quick Decision Guide

| Scenario | Recommended mode |
|----------|-----------------|
| Single team, full Terraform control | `authoritative` |
| Shared with manual admin UI changes | `merge` |
| Multiple Terraform modules / workspaces | `merge` |
| Layering rules on top of tenant defaults | `merge` |
| Greenfield, complete list is known | `authoritative` |
