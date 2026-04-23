package resource_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccServiceConfig_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "appmixer_service_config" "google" {
  service_id = "appmixer:google-basic"
  fields = {
    client_id = "id-123"
  }
  sensitive_fields = {
    client_secret = "secret-456"
  }
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("appmixer_service_config.google", "service_id", "appmixer:google-basic"),
					resource.TestCheckResourceAttr("appmixer_service_config.google", "fields.client_id", "id-123"),
					resource.TestCheckResourceAttr("appmixer_service_config.google", "sensitive_fields.client_secret", "secret-456"),
					resource.TestCheckResourceAttr("appmixer_service_config.google", "id", "appmixer:google-basic"),
				),
			},
		},
	})
}
