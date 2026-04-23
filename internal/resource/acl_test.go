package resource_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccACL_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "appmixer_acl" "test" {
  type = "user"
  rules = [
    {
      role       = "admin"
      resource   = "*"
      action     = ["read", "write"]
      attributes = ["*"]
    }
  ]
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("appmixer_acl.test", "id", "user"),
					resource.TestCheckResourceAttr("appmixer_acl.test", "type", "user"),
					resource.TestCheckResourceAttr("appmixer_acl.test", "rules.#", "1"),
					// Also check rule fields are round-tripped:
					resource.TestCheckTypeSetElemNestedAttrs("appmixer_acl.test", "rules.*", map[string]string{
						"role":         "admin",
						"resource":     "*",
						"attributes.#": "1",
					}),
				),
			},
		},
	})
}

func TestAccACL_update(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "appmixer_acl" "u" {
  type = "group"
  rules = [
    {
      role       = "viewer"
      resource   = "flow"
      action     = ["read"]
      attributes = []
    }
  ]
}
`,
				Check: resource.TestCheckResourceAttr("appmixer_acl.u", "rules.#", "1"),
			},
			{
				Config: `
resource "appmixer_acl" "u" {
  type = "group"
  rules = [
    {
      role       = "viewer"
      resource   = "flow"
      action     = ["read"]
      attributes = []
    },
    {
      role       = "editor"
      resource   = "flow"
      action     = ["read", "write"]
      attributes = []
    }
  ]
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("appmixer_acl.u", "rules.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs("appmixer_acl.u", "rules.*", map[string]string{
						"role":     "viewer",
						"resource": "flow",
					}),
					resource.TestCheckTypeSetElemNestedAttrs("appmixer_acl.u", "rules.*", map[string]string{
						"role":     "editor",
						"resource": "flow",
					}),
				),
			},
		},
	})
}

func TestAccACL_import(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "appmixer_acl" "i" {
  type = "team"
  rules = [
    {
      role       = "member"
      resource   = "workspace"
      action     = ["read"]
      attributes = []
    }
  ]
}
`,
			},
			{
				ResourceName:      "appmixer_acl.i",
				ImportState:       true,
				ImportStateId:     "team",
				ImportStateVerify: true,
			},
		},
	})
}
