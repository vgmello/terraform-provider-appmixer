package resource_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccModifiers_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "appmixer_modifiers" "m" {
  document = jsonencode({
    timeout = 30000
    retries = 3
  })
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("appmixer_modifiers.m", "id", "default"),
					resource.TestCheckResourceAttr("appmixer_modifiers.m", "document", `{"retries":3,"timeout":30000}`),
				),
			},
		},
	})
}

func TestAccModifiers_update(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "appmixer_modifiers" "u" {
  document = jsonencode({ timeout = 5000 })
}
`,
				Check: resource.TestCheckResourceAttr("appmixer_modifiers.u", "id", "default"),
			},
			{
				Config: `
resource "appmixer_modifiers" "u" {
  document = jsonencode({ timeout = 10000 })
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("appmixer_modifiers.u", "id", "default"),
					resource.TestCheckResourceAttr("appmixer_modifiers.u", "document", `{"timeout":10000}`),
				),
			},
		},
	})
}

func TestAccModifiers_import(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "appmixer_modifiers" "i" {
  document = jsonencode({ retries = 5 })
}
`,
			},
			{
				ResourceName:      "appmixer_modifiers.i",
				ImportState:       true,
				ImportStateId:     "default",
				// ImportStateVerify works because Read re-marshals via json.Marshal(map[string]any),
				// which sorts keys alphabetically — matching what jsonencode({...}) produces in HCL.
				ImportStateVerify: true,
			},
		},
	})
}
