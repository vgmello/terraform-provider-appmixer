---
name: Bug Report
about: Report a bug or unexpected behavior in the provider
title: '[Bug] '
labels: bug
assignees: ''
---

## Terraform and Provider Version

- Terraform version: <!-- e.g. 1.9.0 -->
- Provider version: <!-- e.g. 0.0.1 -->
- Go version (if building from source): <!-- e.g. 1.22 -->

## Affected Resource / Data Source

<!-- e.g. appmixer_acl, appmixer_user -->

## Expected Behavior

<!-- What did you expect to happen? -->

## Actual Behavior

<!-- What actually happened? Include any error messages. -->

## Reproduction Steps

<!-- Minimal Terraform configuration that reproduces the issue -->

```terraform
# Provider configuration
provider "appmixer" {
  base_url = "https://api.example.appmixer.cloud"
  # credentials via env vars
}

# Resource that triggers the bug
resource "appmixer_xxx" "example" {
  # ...
}
```

```
# terraform plan / apply output or error
```

## Debug Output

<!-- Run with TF_LOG=DEBUG and include relevant log lines. Remove any sensitive values. -->

## Additional Context

<!-- Any other context, screenshots, or information that might be relevant -->
