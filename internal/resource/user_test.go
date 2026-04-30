package resource_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccUser_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "appmixer_user" "test" {
  email    = "test@example.com"
  password = "secret123"
  scope    = ["admin"]
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("appmixer_user.test", "id"),
					resource.TestCheckResourceAttr("appmixer_user.test", "email", "test@example.com"),
					resource.TestCheckResourceAttr("appmixer_user.test", "scope.#", "1"),
					resource.TestCheckResourceAttr("appmixer_user.test", "scope.0", "admin"),
				),
			},
			{
				Config: `
resource "appmixer_user" "test" {
  email    = "test@example.com"
  password = "secret123"
  scope    = ["admin", "viewer"]
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("appmixer_user.test", "id"),
					resource.TestCheckResourceAttr("appmixer_user.test", "scope.#", "2"),
					resource.TestCheckResourceAttr("appmixer_user.test", "scope.0", "admin"),
					resource.TestCheckResourceAttr("appmixer_user.test", "scope.1", "viewer"),
				),
			},
		},
	})
}

func TestAccUser_passwordRotation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "appmixer_user" "rot" {
  email    = "rotate@example.com"
  password = "first-pw"
  scope    = ["user"]
}
`,
				Check: resource.TestCheckResourceAttr("appmixer_user.rot", "password", "first-pw"),
			},
			{
				// Same user, new password — should rotate in-place, id unchanged.
				Config: `
resource "appmixer_user" "rot" {
  email    = "rotate@example.com"
  password = "second-pw"
  scope    = ["user"]
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("appmixer_user.rot", "password", "second-pw"),
					resource.TestCheckResourceAttr("appmixer_user.rot", "email", "rotate@example.com"),
				),
			},
		},
	})
}

func TestAccUser_import(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "appmixer_user" "imp" {
  email    = "import@example.com"
  password = "importpass"
  scope    = ["user"]
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("appmixer_user.imp", "id"),
				),
			},
			{
				ResourceName:            "appmixer_user.imp",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"password"},
			},
		},
	})
}
