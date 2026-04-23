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
