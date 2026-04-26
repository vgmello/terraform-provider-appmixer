package datasource_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccUserDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "appmixer_user" "target" {
  email    = "ds-user@example.com"
  password = "secret123"
  scope    = ["admin"]
  metadata = { team = "platform" }
}

data "appmixer_user" "lookup" {
  user_id = appmixer_user.target.id
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"data.appmixer_user.lookup", "user_id",
						"appmixer_user.target", "id",
					),
					resource.TestCheckResourceAttr("data.appmixer_user.lookup", "email", "ds-user@example.com"),
					resource.TestCheckResourceAttr("data.appmixer_user.lookup", "scope.#", "1"),
					resource.TestCheckResourceAttr("data.appmixer_user.lookup", "scope.0", "admin"),
					resource.TestCheckResourceAttr("data.appmixer_user.lookup", "metadata.team", "platform"),
				),
			},
		},
	})
}
