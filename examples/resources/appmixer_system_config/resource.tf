resource "appmixer_system_config" "jwt" {
  key   = "JWTSecret"
  value = var.jwt_secret
}

variable "jwt_secret" {
  type      = string
  sensitive = true
}
