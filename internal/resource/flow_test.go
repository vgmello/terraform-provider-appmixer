package resource_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccFlow_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "appmixer_flow" "test" {
  name      = "Test Flow Basic"
  flow_json = jsonencode({ nodes = [] })
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("appmixer_flow.test", "id"),
					resource.TestCheckResourceAttr("appmixer_flow.test", "name", "Test Flow Basic"),
					resource.TestCheckResourceAttrSet("appmixer_flow.test", "stage"),
				),
			},
		},
	})
}

func TestAccFlow_update(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "appmixer_flow" "u" {
  name      = "Flow Before Update"
  flow_json = jsonencode({ nodes = [] })
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("appmixer_flow.u", "name", "Flow Before Update"),
					resource.TestCheckResourceAttrSet("appmixer_flow.u", "id"),
					resource.TestCheckResourceAttrSet("appmixer_flow.u", "stage"),
				),
			},
			{
				Config: `
resource "appmixer_flow" "u" {
  name      = "Flow After Update"
  flow_json = jsonencode({ nodes = [], version = "2" })
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("appmixer_flow.u", "name", "Flow After Update"),
					resource.TestCheckResourceAttr("appmixer_flow.u", "flow_json", `{"nodes":[],"version":"2"}`),
					resource.TestCheckResourceAttrSet("appmixer_flow.u", "stage"),
				),
			},
		},
	})
}

func TestAccFlow_import(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "appmixer_flow" "i" {
  name      = "Flow Import Test"
  flow_json = jsonencode({ nodes = [] })
}
`,
			},
			{
				ResourceName:      "appmixer_flow.i",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs := s.RootModule().Resources["appmixer_flow.i"]
					return rs.Primary.ID, nil
				},
				ImportStateVerifyIgnore: []string{"flow_json", "stage"},
			},
		},
	})
}

func TestAccFlow_customFieldsMixedTypes(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "appmixer_flow" "cf" {
  name      = "Flow Custom Fields"
  flow_json = jsonencode({ nodes = [] })
  custom_fields = {
    category = "customer-ops"
    active   = true
    priority = 1
  }
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("appmixer_flow.cf", "id"),
					resource.TestCheckResourceAttr("appmixer_flow.cf", "name", "Flow Custom Fields"),
				),
			},
		},
	})
}

func TestAccFlow_sharedWith(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "appmixer_flow" "sw" {
  name      = "Flow SharedWith"
  flow_json = jsonencode({ nodes = [] })
  shared_with = [
    {
      permissions = ["read"]
      scope       = "template"
    },
    {
      permissions = ["read", "start", "stop"]
      email       = "user@example.com"
    },
  ]
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("appmixer_flow.sw", "id"),
					resource.TestCheckResourceAttr("appmixer_flow.sw", "shared_with.#", "2"),
					resource.TestCheckResourceAttr("appmixer_flow.sw", "shared_with.0.scope", "template"),
					resource.TestCheckResourceAttr("appmixer_flow.sw", "shared_with.0.permissions.0", "read"),
					resource.TestCheckResourceAttr("appmixer_flow.sw", "shared_with.1.email", "user@example.com"),
				),
			},
		},
	})
}
