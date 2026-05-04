resource "appmixer_flow" "example" {
  name      = "Customer Onboarding"
  flow_json = file("${path.module}/onboarding.json")

  # custom_fields supports string, boolean, and number values.
  custom_fields = {
    category = "customer-ops"
    active   = true
    priority = 1
  }

  shared_with = [
    {
      permissions = ["read"]
      scope       = "template"
    },
    {
      permissions = ["read", "start", "stop"]
      email       = "partner@example.com"
    },
    {
      permissions = ["read"]
      domain      = "acme.com"
    },
  ]
}
