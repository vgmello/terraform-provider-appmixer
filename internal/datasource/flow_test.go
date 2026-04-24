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
  custom_fields = { env = "test" }
}

data "appmixer_flow" "lookup" {
  flow_id = appmixer_flow.target.flow_id
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"data.appmixer_flow.lookup", "flow_id",
						"appmixer_flow.target", "flow_id",
					),
					resource.TestCheckResourceAttr("data.appmixer_flow.lookup", "name", "Data Source Test Flow"),
					resource.TestCheckResourceAttr("data.appmixer_flow.lookup", "stage", "stopped"),
					resource.TestCheckResourceAttr("data.appmixer_flow.lookup", "custom_fields.env", "test"),
				),
			},
		},
	})
}
