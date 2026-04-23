resource "appmixer_account" "example" {
  service      = "appmixer:slack"
  display_name = "Platform Slack Bot"
  token = jsonencode({
    accessToken = var.slack_token
  })
}

variable "slack_token" {
  type      = string
  sensitive = true
}
