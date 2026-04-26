terraform {
  required_providers {
    appmixer = {
      source = "ellosoft/appmixer"
    }
  }
}

# Credentials and base_url come from environment:
# APPMIXER_BASE_URL, APPMIXER_USERNAME, APPMIXER_PASSWORD.
provider "appmixer" {}

variable "user_password" {
  description = "Password for the demo user. Change between applies to exercise rotation."
  type        = string
  default     = "first-pass"
  sensitive   = true
}

variable "quota_source" {
  description = "Node.js quota source. Swap between applies to exercise update."
  type        = string
  default     = null
}

# ---- system configuration ----

resource "appmixer_system_config" "api_url" {
  key   = "API_URL_OVERRIDE"
  value = "https://api.example.com"
}

# ---- third-party service configuration ----

resource "appmixer_service_config" "google" {
  service_id = "appmixer:google"

  items = {
    client_id = "gcp-client-123"
  }

  sensitive_items = {
    client_secret = "shhh"
  }
}

# ---- modifiers (singleton) ----

resource "appmixer_modifiers" "default" {
  document = jsonencode({
    categories = {
      object = { label = "Object", index = 1 }
    }
    modifiers = {
      stringify = {
        name     = "stringify"
        label    = "Stringify"
        category = ["object"]
      }
    }
  })
}

# ---- access-control list ----

resource "appmixer_acl" "components" {
  type = "components"

  rules = [
    {
      role       = "admin"
      resource   = "*"
      action     = ["*"]
      attributes = ["*"]
    },
    {
      role       = "viewer"
      resource   = "*"
      action     = ["read"]
      attributes = ["non-private"]
    },
  ]
}

# ---- service account ----

resource "appmixer_account" "slack" {
  service      = "appmixer:slack"
  name         = "ci-slack"
  display_name = "CI-managed Slack account"
  token        = jsonencode({ accessToken = "xoxb-fake" })
  profile_info = jsonencode({ team = "QA" })
}

# ---- admin user ----

resource "appmixer_user" "demo" {
  email    = "demo@example.com"
  password = var.user_password
  scope    = ["admin"]
  metadata = {
    purpose = "stack-e2e"
  }
}

# ---- flow ----

resource "appmixer_flow" "hello" {
  name = "hello-world"
  flow_json = jsonencode({
    components = {
      a = { type = "appmixer.utils.controls.OnStart" }
    }
  })
  custom_fields = {
    category = "demo"
  }
}

# ---- quota ----

resource "appmixer_quota" "hubspot" {
  service_id = "appmixer:hubspot"
  source     = var.quota_source == null ? file("${path.module}/quota.js") : var.quota_source
}

# ---- data sources (round-trip the resources we just created) ----

data "appmixer_user" "readback" {
  user_id    = appmixer_user.demo.id
  depends_on = [appmixer_user.demo]
}

data "appmixer_flow" "readback" {
  flow_id    = appmixer_flow.hello.id
  depends_on = [appmixer_flow.hello]
}

# ---- outputs used by the e2e harness ----

output "user_id"          { value = appmixer_user.demo.id }
output "user_email"       { value = appmixer_user.demo.email }
output "flow_id"          { value = appmixer_flow.hello.id }
output "flow_name"        { value = appmixer_flow.hello.name }
output "account_id"       { value = appmixer_account.slack.id }
output "quota_id"         { value = appmixer_quota.hubspot.id }
output "quota_is_custom"  { value = appmixer_quota.hubspot.is_custom }
output "readback_user"    { value = data.appmixer_user.readback.user_id }
output "readback_flow"    { value = data.appmixer_flow.readback.flow_id }
output "readback_stage"   { value = data.appmixer_flow.readback.stage }
