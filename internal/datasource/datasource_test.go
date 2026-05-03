package datasource_test

import (
	"os"
	"testing"

	"github.com/ellosoft/terraform-provider-appmixer/internal/acctest"
	appprovider "github.com/ellosoft/terraform-provider-appmixer/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

var protoV6Factories = map[string]func() (tfprotov6.ProviderServer, error){
	"appmixer": providerserver.NewProtocol6WithError(appprovider.New("dev")()),
}

func TestMain(m *testing.M) {
	cleanup := acctest.SpawnMockPackageLevel()
	code := m.Run()
	cleanup()
	os.Exit(code)
}
