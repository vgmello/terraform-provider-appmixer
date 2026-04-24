package resource_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// seedACLRules POSTs a rule list directly to the mock, bypassing Terraform.
// Used to simulate externally-configured rules that merge mode should preserve.
func seedACLRules(t *testing.T, aclType string, rules []map[string]any) {
	t.Helper()
	baseURL := os.Getenv("APPMIXER_BASE_URL")
	if baseURL == "" {
		t.Fatal("APPMIXER_BASE_URL not set — mock server not running")
	}
	body, _ := json.Marshal(rules)
	req, _ := http.NewRequest("POST", baseURL+"/acl/"+aclType, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer mock-token")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("seed %s: %v", aclType, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("seed %s: want 200, got %d", aclType, resp.StatusCode)
	}
}

// readACLServer returns the current rule list for aclType from the mock.
func readACLServer(t *testing.T, aclType string) []map[string]any {
	t.Helper()
	baseURL := os.Getenv("APPMIXER_BASE_URL")
	req, _ := http.NewRequest("GET", baseURL+"/acl/"+aclType, nil)
	req.Header.Set("Authorization", "Bearer mock-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("read %s: %v", aclType, err)
	}
	defer resp.Body.Close()
	var out []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode %s: %v", aclType, err)
	}
	return out
}

// serverACLHasRole returns a test check that asserts the server-side ACL for
// aclType contains a rule with the given role. This goes past Terraform state
// to verify merge-mode externals are preserved.
func serverACLHasRole(t *testing.T, aclType, role string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		server := readACLServer(t, aclType)
		for _, r := range server {
			if r["role"] == role {
				return nil
			}
		}
		return fmt.Errorf("expected role %q in server %s ACL; server had %v", role, aclType, server)
	}
}

// serverACLLacksRole asserts the server-side ACL has no rule with the given role.
func serverACLLacksRole(t *testing.T, aclType, role string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		server := readACLServer(t, aclType)
		for _, r := range server {
			if r["role"] == role {
				return fmt.Errorf("did not expect role %q in server %s ACL; server had %v", role, aclType, server)
			}
		}
		return nil
	}
}

// serverACLCount asserts the total number of rules on the server for aclType.
func serverACLCount(t *testing.T, aclType string, want int) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		got := len(readACLServer(t, aclType))
		if got != want {
			return fmt.Errorf("server %s ACL: want %d rules, got %d", aclType, want, got)
		}
		return nil
	}
}

func TestAccACL_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "appmixer_acl" "test" {
  type = "components"
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
					resource.TestCheckResourceAttr("appmixer_acl.test", "id", "components"),
					resource.TestCheckResourceAttr("appmixer_acl.test", "type", "components"),
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
  type = "routes"
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
  type = "routes"
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
  type = "routes"
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
				ImportStateId:     "routes",
				ImportStateVerify: true,
			},
		},
	})
}

// Externally-seeded rule used to prove merge mode preserves it.
var externalRule = map[string]any{
	"role":       "external-ops",
	"resource":   "dashboard",
	"action":     []any{"read"},
	"attributes": []any{"*"},
}

// TestAccACL_mergePreservesExternalOnCreate: pre-seed a rule outside of
// Terraform, then apply a merge-mode resource. The server should end up with
// both; destroy should keep the external.
func TestAccACL_mergePreservesExternalOnCreate(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					seedACLRules(t, "components", []map[string]any{externalRule})
				},
				Config: `
resource "appmixer_acl" "m" {
  type = "components"
  mode = "merge"
  rules = [
    {
      role       = "terraform-admin"
      resource   = "*"
      action     = ["read", "write"]
      attributes = ["*"]
    }
  ]
}
`,
				Check: resource.ComposeTestCheckFunc(
					// State only knows about our rule.
					resource.TestCheckResourceAttr("appmixer_acl.m", "mode", "merge"),
					resource.TestCheckResourceAttr("appmixer_acl.m", "rules.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs("appmixer_acl.m", "rules.*", map[string]string{
						"role": "terraform-admin",
					}),
					// Server has both the external and our rule.
					serverACLCount(t, "components", 2),
					serverACLHasRole(t, "components", "external-ops"),
					serverACLHasRole(t, "components", "terraform-admin"),
				),
			},
		},
		CheckDestroy: func(_ *terraform.State) error {
			// After destroy, external rule must remain; our rule must be gone.
			if err := serverACLHasRole(t, "components", "external-ops")(nil); err != nil {
				return err
			}
			if err := serverACLLacksRole(t, "components", "terraform-admin")(nil); err != nil {
				return err
			}
			// Clean up: remove the external so later tests start empty.
			seedACLRules(t, "components", []map[string]any{})
			return nil
		},
	})
}

// TestAccACL_mergeAddRemoveRules: with an external rule in place, add a
// Terraform-managed rule, then swap it for a different one. Externals stay,
// the previously managed rule is deleted.
func TestAccACL_mergeAddRemoveRules(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					seedACLRules(t, "components", []map[string]any{externalRule})
				},
				Config: `
resource "appmixer_acl" "swap" {
  type = "components"
  mode = "merge"
  rules = [
    {
      role       = "first-owner"
      resource   = "*"
      action     = ["read"]
      attributes = ["*"]
    }
  ]
}
`,
				Check: resource.ComposeTestCheckFunc(
					serverACLCount(t, "components", 2),
					serverACLHasRole(t, "components", "external-ops"),
					serverACLHasRole(t, "components", "first-owner"),
				),
			},
			{
				Config: `
resource "appmixer_acl" "swap" {
  type = "components"
  mode = "merge"
  rules = [
    {
      role       = "second-owner"
      resource   = "*"
      action     = ["read"]
      attributes = ["*"]
    }
  ]
}
`,
				Check: resource.ComposeTestCheckFunc(
					// Server has external + new rule; the first-owner is gone.
					serverACLCount(t, "components", 2),
					serverACLHasRole(t, "components", "external-ops"),
					serverACLHasRole(t, "components", "second-owner"),
					serverACLLacksRole(t, "components", "first-owner"),
				),
			},
		},
		CheckDestroy: func(_ *terraform.State) error {
			if err := serverACLHasRole(t, "components", "external-ops")(nil); err != nil {
				return err
			}
			seedACLRules(t, "components", []map[string]any{})
			return nil
		},
	})
}

// TestAccACL_authoritativeNukesExternal: the existing behavior — authoritative
// mode wipes externals. Guards against regression.
func TestAccACL_authoritativeNukesExternal(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					seedACLRules(t, "components", []map[string]any{externalRule})
				},
				Config: `
resource "appmixer_acl" "a" {
  type = "components"
  rules = [
    {
      role       = "only-admin"
      resource   = "*"
      action     = ["*"]
      attributes = ["*"]
    }
  ]
}
`,
				Check: resource.ComposeTestCheckFunc(
					// Authoritative mode replaced the list — only our rule remains.
					serverACLCount(t, "components", 1),
					serverACLHasRole(t, "components", "only-admin"),
					serverACLLacksRole(t, "components", "external-ops"),
				),
			},
		},
	})
}
