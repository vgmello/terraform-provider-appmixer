resource "appmixer_user" "admin_ops" {
  email    = "ops@example.com"
  password = var.ops_initial_password
  scope    = ["admin"]
  metadata = { team = "platform" }
}
