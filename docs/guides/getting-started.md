---
page_title: "Getting Started - appmixer Provider"
description: |-
  A step-by-step guide to configuring the Appmixer provider and managing your
  first resources with Terraform.
---

# Getting Started with the Appmixer Provider

This guide walks you through setting up the Appmixer Terraform provider and creating your first resources.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.8
- An Appmixer tenant with admin credentials
- The Appmixer API base URL for your tenant (e.g. `https://api.your-tenant.appmixer.cloud`)

## Provider Configuration

Add the provider to your `required_providers` block and configure it with your tenant credentials:

```terraform
terraform {
  required_providers {
    appmixer = {
      source  = "vgmello/appmixer"
      version = "~> 0.1"
    }
  }
}

provider "appmixer" {
  base_url = "https://api.your-tenant.appmixer.cloud"
  username = "admin@example.com"
  password = var.appmixer_password
}

variable "appmixer_password" {
  type      = string
  sensitive = true
}
```

### Using Environment Variables

All three provider attributes fall back to environment variables, which keeps credentials out of your Terraform files entirely:

```shell
export APPMIXER_BASE_URL="https://api.your-tenant.appmixer.cloud"
export APPMIXER_USERNAME="admin@example.com"
export APPMIXER_PASSWORD="your-password"
```

With environment variables set, the provider block can be left empty:

```terraform
provider "appmixer" {}
```

## Creating Your First Resources

### Step 1 — Create a Service Account

Service accounts are used by integrations and automations to authenticate with Appmixer without personal credentials:

```terraform
resource "appmixer_account" "ci_bot" {
  username    = "ci-bot"
  vendor      = "myorg"
  token       = var.ci_bot_token
  token_name  = "apiKey"
}
```

### Step 2 — Create a User

```terraform
resource "appmixer_user" "alice" {
  username = "alice@example.com"
  password = var.alice_password
  name     = "Alice"
  role     = "user"
}
```

### Step 3 — Apply ACL Rules

Restrict which components each role can access:

```terraform
resource "appmixer_acl" "components" {
  type = "components"

  rules = [
    {
      role       = "user"
      resource   = "appmixer.services.*"
      action     = ["read"]
      attributes = ["*"]
    },
    {
      role       = "admin"
      resource   = "*"
      action     = ["read", "write", "delete"]
      attributes = ["*"]
    },
  ]
}
```

## Importing Existing Resources

If you have an existing Appmixer tenant with resources that were configured outside Terraform, use `terraform import` to bring them under management:

```shell
# Import an existing user
terraform import appmixer_user.alice alice@example.com

# Import ACL rules for a type
terraform import appmixer_acl.components components
```

After importing, run `terraform plan` to verify the state matches your configuration before making any changes.

## Next Steps

- Read the [ACL Strategies guide](acl-strategies) to choose the right ownership mode for your ACL resources
- Browse the [resource documentation](../resources/acl) for full attribute references
- See the [full stack example](../../examples/stack/README.md) for a complete working configuration
