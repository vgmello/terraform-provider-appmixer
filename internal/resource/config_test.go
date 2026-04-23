package resource_test

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/ellosoft/terraform-provider-appmixer/internal/acctest"
	appprovider "github.com/ellosoft/terraform-provider-appmixer/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

var protoV6Factories = map[string]func() (tfprotov6.ProviderServer, error){
	"appmixer": providerserver.NewProtocol6WithError(appprovider.New()()),
}

func TestMain(m *testing.M) {
	cleanup := acctest.SpawnMockPackageLevel()
	code := m.Run()
	cleanup()
	os.Exit(code)
}

func TestAccConfig_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "appmixer_config" "x" {
  key   = "SAMPLE_KEY"
  value = "v1"
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("appmixer_config.x", "key", "SAMPLE_KEY"),
					resource.TestCheckResourceAttr("appmixer_config.x", "value", "v1"),
				),
			},
		},
	})
}

func TestAccConfig_replaceOnValueChange(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "appmixer_config" "x" {
  key   = "REPLACE_ME"
  value = "first"
}
`,
				Check: resource.TestCheckResourceAttr("appmixer_config.x", "value", "first"),
			},
			{
				Config: `
resource "appmixer_config" "x" {
  key   = "REPLACE_ME"
  value = "second"
}
`,
				Check: resource.TestCheckResourceAttr("appmixer_config.x", "value", "second"),
				// Framework detects the implicit destroy+create in the plan; the
				// test step's default drift check validates state matches config
				// after the replacement.
			},
		},
	})
}

func TestAccConfig_import(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "appmixer_config" "x" {
  key   = "IMPORT_KEY"
  value = "to-be-imported"
}
`,
			},
			{
				ResourceName:      "appmixer_config.x",
				ImportState:       true,
				ImportStateId:     "IMPORT_KEY",
				ImportStateVerify: true,
				// `value` is sensitive and not round-tripped reliably from a fresh import;
				// accept that it may not match until the next apply.
				ImportStateVerifyIgnore: []string{"value"},
			},
		},
	})
}
