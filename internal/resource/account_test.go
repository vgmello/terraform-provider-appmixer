package resource_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// accountIDSnapshot captures the id of an appmixer_account resource on one step so
// later steps can assert the id is unchanged (i.e. the resource was not recreated).
func accountIDSnapshot(resourceName string, dst *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %s not found in state", resourceName)
		}
		*dst = rs.Primary.ID
		return nil
	}
}

// accountIDSame asserts the live id matches a previously-captured id.
func accountIDSame(resourceName string, prior *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %s not found in state", resourceName)
		}
		if rs.Primary.ID != *prior {
			return fmt.Errorf("expected id %q to be stable, got %q (resource was recreated)", *prior, rs.Primary.ID)
		}
		return nil
	}
}

// accountIDChanged asserts the live id differs from a previously-captured id (i.e. recreation happened).
func accountIDChanged(resourceName string, prior *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %s not found in state", resourceName)
		}
		if rs.Primary.ID == *prior {
			return fmt.Errorf("expected id to change after recreation, still %q", rs.Primary.ID)
		}
		return nil
	}
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
