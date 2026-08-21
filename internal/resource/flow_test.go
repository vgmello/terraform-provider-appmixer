package resource_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/ellosoft/terraform-provider-appmixer/internal/acctest"

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
					resource.TestCheckResourceAttr("appmixer_flow.cf", "custom_fields.category", "customer-ops"),
					resource.TestCheckResourceAttr("appmixer_flow.cf", "custom_fields.active", "true"),
					resource.TestCheckResourceAttr("appmixer_flow.cf", "custom_fields.priority", "1"),
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

// TestAccFlow_serverUpgradesComponentVersion reproduces issue #24: Appmixer
// upgrades every component in a flow to the newest installed version on write,
// so the descriptor read back differs from the one in the config. The provider
// used to write that server copy into flow_json, which Terraform rejected with
// "Provider produced inconsistent result after apply". The mock server performs
// the same upgrade, so an apply here fails if that regresses. A non-empty plan
// after either step means the upgrade leaked into state as perpetual drift.
func TestAccFlow_serverUpgradesComponentVersion(t *testing.T) {
	const pinned = `{"c1":{"type":"appmixer.utils.controls.Each","version":"1.4.5","x":1312,"y":96}}`
	const repinned = `{"c1":{"type":"appmixer.utils.controls.Each","version":"1.4.6","x":1312,"y":96}}`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "appmixer_flow" "v" {
  name      = "Flow Component Version"
  flow_json = jsonencode({
    c1 = { type = "appmixer.utils.controls.Each", version = "1.4.5", x = 1312, y = 96 }
  })
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("appmixer_flow.v", "id"),
					resource.TestCheckResourceAttr("appmixer_flow.v", "flow_json", pinned),
				),
			},
			{
				// Re-pinning a version is still a real change and must be applied.
				Config: `
resource "appmixer_flow" "v" {
  name      = "Flow Component Version"
  flow_json = jsonencode({
    c1 = { type = "appmixer.utils.controls.Each", version = "1.4.6", x = 1312, y = 96 }
  })
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("appmixer_flow.v", "flow_json", repinned),
				),
			},
		},
	})
}

// TestAccFlow_serverUpgradeIsNotDrift asserts the refresh path: after the mock
// server has upgraded the stored component version, a plan that refreshes state
// must still come back empty rather than proposing to rewrite flow_json.
func TestAccFlow_serverUpgradeIsNotDrift(t *testing.T) {
	config := `
resource "appmixer_flow" "d" {
  name      = "Flow Version Drift"
  flow_json = jsonencode({
    c1 = { type = "appmixer.utils.controls.Each", version = "1.4.5" }
  })
}
`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				RefreshState:       true,
				ExpectNonEmptyPlan: false,
				Check: resource.TestCheckResourceAttr("appmixer_flow.d", "flow_json",
					`{"c1":{"type":"appmixer.utils.controls.Each","version":"1.4.5"}}`),
			},
		},
	})
}

// TestAccFlow_realDriftIsDetected is the counterweight to the two tests above:
// reconciliation must hide the component version and nothing else. A field
// changed outside Terraform still has to show up as drift.
func TestAccFlow_realDriftIsDetected(t *testing.T) {
	config := `
resource "appmixer_flow" "rd" {
  name      = "Flow Real Drift"
  flow_json = jsonencode({
    c1 = { type = "appmixer.utils.controls.Each", version = "1.4.5", x = 10 }
  })
}
`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					mutateStoredFlow(t, "Flow Real Drift", func(flow map[string]any) {
						flow["c1"].(map[string]any)["x"] = float64(999)
					})
				},
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TestAccFlow_unpinnedComponentVersion covers the descriptor that pins no
// component version at all. Appmixer supplies one on write, and keeping it in
// state would propose removing a field the next apply cannot remove — the same
// never-converging loop as #24, only silent. The mock server fills in a version
// for every component, pinned or not, so this fails if reconciliation stops
// treating a server-supplied version as server-owned.
func TestAccFlow_unpinnedComponentVersion(t *testing.T) {
	config := `
resource "appmixer_flow" "np" {
  name      = "Flow Unpinned Version"
  flow_json = jsonencode({
    c1 = { type = "appmixer.utils.controls.Each", x = 1312, y = 96 }
  })
}
`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.TestCheckResourceAttr("appmixer_flow.np", "flow_json",
					`{"c1":{"type":"appmixer.utils.controls.Each","x":1312,"y":96}}`),
			},
			{
				RefreshState:       true,
				ExpectNonEmptyPlan: false,
				Check: resource.TestCheckResourceAttr("appmixer_flow.np", "flow_json",
					`{"c1":{"type":"appmixer.utils.controls.Each","x":1312,"y":96}}`),
			},
		},
	})
}

// TestAccFlow_createReadBackFailureDoesNotOrphan asserts that when the POST
// succeeds but the read-back that follows it fails, the flow's id still reaches
// state. Without that, the flow is left running on the tenant with nothing in
// state to update or destroy it, and the next apply creates a second copy.
func TestAccFlow_createReadBackFailureDoesNotOrphan(t *testing.T) {
	const name = "Flow Orphan Check"
	config := `
resource "appmixer_flow" "orph" {
  name      = "Flow Orphan Check"
  flow_json = jsonencode({ nodes = [] })
}
`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				PreConfig:   func() { acctest.Store.FailNextFlowGet() },
				Config:      config,
				ExpectError: regexp.MustCompile("Read /flows after create failed"),
			},
			{
				// The retry must adopt the flow created by the failed step
				// rather than create a second one.
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("appmixer_flow.orph", "id"),
					func(*terraform.State) error {
						if n := acctest.Store.CountFlowsByName(name); n != 1 {
							return fmt.Errorf("expected exactly 1 flow named %q on the server, found %d (create was orphaned)", name, n)
						}
						return nil
					},
				),
			},
		},
	})
}
