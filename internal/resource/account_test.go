package resource_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// accountIDSnapshot, accountIDSame, accountIDChanged are thin wrappers around
// the generic resourceID* helpers with account-specific names kept for
// backwards-compat with existing test step references.
func accountIDSnapshot(resourceName string, dst *string) resource.TestCheckFunc {
	return resourceIDSnapshot(resourceName, dst)
}

func accountIDSame(resourceName string, prior *string) resource.TestCheckFunc {
	return resourceIDSame(resourceName, prior)
}

func accountIDChanged(resourceName string, prior *string) resource.TestCheckFunc {
	return resourceIDChanged(resourceName, prior)
}

const accountBaseConfig = `
resource "appmixer_account" "test" {
  service = "appmixer:slack"
  name    = "test-slack-bot"
  token   = "{\"accessToken\": \"test-token\"}"
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
					resource.TestCheckResourceAttr("appmixer_account.test", "service", "appmixer:slack"),
					resource.TestCheckResourceAttr("appmixer_account.test", "name", "test-slack-bot"),
				),
			},
		},
	})
}

func TestAccAccount_replaceOnNameChange(t *testing.T) {
	var priorID string
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: accountBaseConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("appmixer_account.test", "name", "test-slack-bot"),
					accountIDSnapshot("appmixer_account.test", &priorID),
				),
			},
			{
				Config: `
resource "appmixer_account" "test" {
  service = "appmixer:slack"
  name    = "updated-slack-bot"
  token   = "{\"accessToken\": \"test-token\"}"
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("appmixer_account.test", "name", "updated-slack-bot"),
					accountIDChanged("appmixer_account.test", &priorID),
				),
			},
		},
	})
}

func TestAccAccount_updateDisplayName(t *testing.T) {
	var priorID string
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "appmixer_account" "test" {
  service      = "appmixer:slack"
  name         = "test-slack-bot"
  display_name = "Test Slack Bot"
  token        = "{\"accessToken\": \"test-token\"}"
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("appmixer_account.test", "display_name", "Test Slack Bot"),
					accountIDSnapshot("appmixer_account.test", &priorID),
				),
			},
			{
				Config: `
resource "appmixer_account" "test" {
  service      = "appmixer:slack"
  name         = "test-slack-bot"
  display_name = "Updated Slack Bot"
  token        = "{\"accessToken\": \"test-token\"}"
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("appmixer_account.test", "display_name", "Updated Slack Bot"),
					accountIDSame("appmixer_account.test", &priorID),
				),
			},
		},
	})
}

func TestAccAccount_updateToken(t *testing.T) {
	var priorID string
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: accountBaseConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("appmixer_account.test", "id"),
					resource.TestCheckResourceAttr("appmixer_account.test", "token", `{"accessToken": "test-token"}`),
					accountIDSnapshot("appmixer_account.test", &priorID),
				),
			},
			{
				Config: `
resource "appmixer_account" "test" {
  service = "appmixer:slack"
  name    = "test-slack-bot"
  token   = "{\"accessToken\": \"rotated-token\"}"
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("appmixer_account.test", "token", `{"accessToken": "rotated-token"}`),
					accountIDSame("appmixer_account.test", &priorID),
				),
			},
		},
	})
}

func TestAccAccount_updateProfileInfo(t *testing.T) {
	var priorID string
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "appmixer_account" "test" {
  service      = "appmixer:slack"
  name         = "profile-slack-bot"
  token        = "{\"accessToken\": \"test-token\"}"
  profile_info = "{\"email\": \"a@test.com\"}"
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("appmixer_account.test", "profile_info", `{"email": "a@test.com"}`),
					accountIDSnapshot("appmixer_account.test", &priorID),
				),
			},
			{
				Config: `
resource "appmixer_account" "test" {
  service      = "appmixer:slack"
  name         = "profile-slack-bot"
  token        = "{\"accessToken\": \"test-token\"}"
  profile_info = "{\"email\": \"b@test.com\"}"
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("appmixer_account.test", "profile_info", `{"email": "b@test.com"}`),
					accountIDSame("appmixer_account.test", &priorID),
				),
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
