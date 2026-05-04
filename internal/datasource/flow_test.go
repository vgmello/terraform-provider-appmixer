package datasource_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccFlowDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "appmixer_flow" "target" {
  name      = "Data Source Test Flow"
  flow_json = jsonencode({ nodes = [], edges = [] })
  custom_fields = { env = "test", enabled = true, version = 2 }
}

data "appmixer_flow" "lookup" {
  flow_id = appmixer_flow.target.id
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"data.appmixer_flow.lookup", "flow_id",
						"appmixer_flow.target", "id",
					),
					resource.TestCheckResourceAttr("data.appmixer_flow.lookup", "name", "Data Source Test Flow"),
					resource.TestCheckResourceAttr("data.appmixer_flow.lookup", "stage", "stopped"),
				),
			},
		},
	})
}

func TestAccFlowDataSource_sharedWith(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "appmixer_flow" "target2" {
  name      = "Data Source SharedWith Flow"
  flow_json = jsonencode({ nodes = [] })
  shared_with = [
    {
      permissions = ["read"]
      scope       = "template"
    },
  ]
}

data "appmixer_flow" "lookup2" {
  flow_id = appmixer_flow.target2.id
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"data.appmixer_flow.lookup2", "flow_id",
						"appmixer_flow.target2", "id",
					),
					resource.TestCheckResourceAttr("data.appmixer_flow.lookup2", "shared_with.#", "1"),
					resource.TestCheckResourceAttr("data.appmixer_flow.lookup2", "shared_with.0.scope", "template"),
				),
			},
		},
	})
}
