package resource_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const accountBaseConfig = `
resource "appmixer_account" "test" {
  service      = "appmixer:slack"
  display_name = "Test Slack Bot"
  token        = "{\"accessToken\": \"test-token\"}"
}
`

func TestAccAccount_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: accountBaseConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("appmixer_account.test", "id"),
					resource.TestCheckResourceAttrSet("appmixer_account.test", "account_id"),
					resource.TestCheckResourceAttrPair("appmixer_account.test", "id", "appmixer_account.test", "account_id"),
					resource.TestCheckResourceAttr("appmixer_account.test", "service", "appmixer:slack"),
					resource.TestCheckResourceAttr("appmixer_account.test", "display_name", "Test Slack Bot"),
				),
			},
		},
	})
}

func TestAccAccount_update(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: accountBaseConfig,
				Check:  resource.TestCheckResourceAttr("appmixer_account.test", "display_name", "Test Slack Bot"),
			},
			{
				Config: `
resource "appmixer_account" "test" {
  service      = "appmixer:slack"
  display_name = "Updated Slack Bot"
  token        = "{\"accessToken\": \"test-token\"}"
}
`,
				Check: resource.TestCheckResourceAttr("appmixer_account.test", "display_name", "Updated Slack Bot"),
			},
		},
	})
}

func TestAccAccount_import(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: accountBaseConfig,
			},
			{
				ResourceName:            "appmixer_account.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"token", "profile_info"},
			},
		},
	})
}
