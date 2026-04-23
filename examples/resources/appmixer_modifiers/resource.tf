resource "appmixer_modifiers" "example" {
  document = jsonencode({
    timeout = 30000
    retries = 3
  })
}
