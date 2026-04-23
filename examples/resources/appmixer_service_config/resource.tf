resource "appmixer_service_config" "google" {
  service_id = "appmixer:google"
  fields = {
    client_id = var.google_client_id
  }
  sensitive_fields = {
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
