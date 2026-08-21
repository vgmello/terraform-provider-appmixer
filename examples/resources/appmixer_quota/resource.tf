resource "appmixer_quota" "hubspot" {
  name   = "appmixer:hubspot"
  source = file("${path.module}/hubspot-quota.js")
}

# Inline-defined quota for a tenant-custom rule.
resource "appmixer_quota" "tenant_custom" {
  name   = "mews:bookings"
  source = <<-EOT
    'use strict';

    module.exports = {
      rules: [
        {
          limit: 100,
          window: 1000 * 60,
          throttling: 'window-sliding',
          queueing: 'fifo',
          resource: 'requests',
          scope: 'userId'
        }
      ]
    };
  EOT
}
