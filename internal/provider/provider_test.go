package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func newTestProviderServer(t *testing.T, handler http.HandlerFunc) (map[string]func() (tfprotov6.ProviderServer, error), func()) {
	t.Helper()
	srv := httptest.NewServer(handler)

	t.Setenv("APPMIXER_BASE_URL", srv.URL)
	t.Setenv("APPMIXER_USERNAME", "u")
	t.Setenv("APPMIXER_PASSWORD", "p")

	factories := map[string]func() (tfprotov6.ProviderServer, error){
		"appmixer": providerserver.NewProtocol6WithError(New()()),
	}
	return factories, func() { srv.Close() }
}

func loginSuccessHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost && r.URL.Path == "/user/auth" {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["username"] == "" || body["password"] == "" {
			w.WriteHeader(401)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"t"}`))
		return
	}
	http.NotFound(w, r)
}

func TestProvider_ConfigureReadsEnvFallbacks(t *testing.T) {
	var loginCalled int64

	wrappedHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/user/auth" {
			atomic.AddInt64(&loginCalled, 1)
		}
		loginSuccessHandler(w, r)
	}

	factories, cleanup := newTestProviderServer(t, wrappedHandler)
	defer cleanup()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "appmixer" {}
resource "appmixer_config" "x" {
  key   = "k"
  value = "v"
}
`,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})

	if atomic.LoadInt64(&loginCalled) == 0 {
		t.Fatal("Configure was not called — env-fallback path not exercised")
	}
}

func TestProvider_MissingConfigProducesDiagnostic(t *testing.T) {
	t.Setenv("APPMIXER_BASE_URL", "")
	t.Setenv("APPMIXER_USERNAME", "")
	t.Setenv("APPMIXER_PASSWORD", "")

	factories := map[string]func() (tfprotov6.ProviderServer, error){
		"appmixer": providerserver.NewProtocol6WithError(New()()),
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "appmixer" {}
resource "appmixer_config" "x" {
  key   = "k"
  value = "v"
}
`,
				ExpectError: regexp.MustCompile("Missing provider configuration"),
				PlanOnly:    true,
			},
		},
	})
}

