# Authoritative (default): this resource owns the entire service-config object.
# Any keys configured out-of-band (e.g. via the Appmixer UI) are wiped on apply;
# destroy removes the whole object.
resource "appmixer_service_config" "google" {
  service_id = "appmixer:google"
  items = {
    client_id = var.google_client_id
  }
  sensitive_items = {
    client_secret = var.google_client_secret
  }
}

# Merge: this resource owns only the keys it declares. Keys added in the
# Appmixer UI (or by other automation) are preserved across apply and destroy.
# Keys removed from `items`/`sensitive_items` between applies are deleted from
# the server; externally-added keys are left in place.
resource "appmixer_service_config" "slack" {
  service_id = "appmixer:slack"
  mode       = "merge"
  items = {
    client_id = var.slack_client_id
  }
  sensitive_items = {
    client_secret = var.slack_client_secret
  }
}

variable "google_client_id" {
  type = string
}

variable "google_client_secret" {
  type      = string
  sensitive = true
}

variable "slack_client_id" {
  type = string
}

variable "slack_client_secret" {
  type      = string
  sensitive = true
}
