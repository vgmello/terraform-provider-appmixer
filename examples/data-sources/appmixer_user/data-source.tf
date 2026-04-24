data "appmixer_user" "ops" {
  user_id = "user-abc123"
}

output "ops_email" {
  value = data.appmixer_user.ops.email
}
