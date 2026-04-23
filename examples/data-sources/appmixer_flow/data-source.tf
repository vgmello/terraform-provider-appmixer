data "appmixer_flow" "onboarding" {
  flow_id = "flow-abc123"
}

output "onboarding_stage" {
  value = data.appmixer_flow.onboarding.stage
}
