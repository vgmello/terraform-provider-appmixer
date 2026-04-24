resource "appmixer_service_config" "google" {
  service_id = "appmixer:google"
  items = {
    client_id = var.google_client_id
  }
  sensitive_items = {
    client_secret = var.google_client_secret
  }
}

variable "google_client_id" {
  type = string
}

variable "google_client_secret" {
  type      = string
  sensitive = true
}
