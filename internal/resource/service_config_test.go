package resource_test

import (
	"regexp"
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

func TestAccServiceConfig_rejectsKeyCollision(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "appmixer_service_config" "dup" {
  service_id = "appmixer:collision-test"
  fields = {
    shared = "plain"
  }
  sensitive_fields = {
    shared = "secret"
  }
}
`,
				ExpectError: regexp.MustCompile(`appears in both fields and sensitive_fields`),
			},
		},
	})
}

func TestAccServiceConfig_updatesFieldsViaPUT(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "appmixer_service_config" "u" {
  service_id = "appmixer:update-test"
  fields = {
    client_id = "first"
  }
  sensitive_fields = {
    client_secret = "secret-one"
  }
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("appmixer_service_config.u", "fields.client_id", "first"),
					resource.TestCheckResourceAttr("appmixer_service_config.u", "sensitive_fields.client_secret", "secret-one"),
				),
			},
			{
				Config: `
resource "appmixer_service_config" "u" {
  service_id = "appmixer:update-test"
  fields = {
    client_id = "second"
  }
  sensitive_fields = {
    client_secret = "secret-one"
  }
}
`,
				Check: resource.TestCheckResourceAttr("appmixer_service_config.u", "fields.client_id", "second"),
			},
		},
	})
}
