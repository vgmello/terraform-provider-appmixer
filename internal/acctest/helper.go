package acctest

import (
	"os"
	"testing"

	"github.com/ellosoft/terraform-provider-appmixer/internal/mockserver"
)

// SpawnMock starts the in-process mock server and registers cleanup with t.
func SpawnMock(t *testing.T) {
	t.Helper()
	addr, stop := mockserver.Start()
	t.Setenv("APPMIXER_BASE_URL", addr)
	t.Setenv("APPMIXER_USERNAME", "admin@test.com")
	t.Setenv("APPMIXER_PASSWORD", "test123")
	t.Setenv("TF_ACC", "1")
	t.Cleanup(stop)
}

// Store is the in-memory state of the mock server started by
// SpawnMockPackageLevel, so tests can simulate out-of-band changes.
var Store *mockserver.Store

// SpawnMockPackageLevel is for use in TestMain. Returns a cleanup function.
func SpawnMockPackageLevel() func() {
	addr, store, stop := mockserver.StartWithStore()
	Store = store
	_ = os.Setenv("APPMIXER_BASE_URL", addr)
	_ = os.Setenv("APPMIXER_USERNAME", "admin@test.com")
	_ = os.Setenv("APPMIXER_PASSWORD", "test123")
	_ = os.Setenv("TF_ACC", "1")
	return stop
}
