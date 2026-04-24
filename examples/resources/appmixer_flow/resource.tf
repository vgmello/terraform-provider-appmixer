resource "appmixer_flow" "example" {
  name      = "Customer Onboarding"
  flow_json = file("${path.module}/onboarding.json")
  custom_fields = {
    category = "customer-ops"
  }
}
