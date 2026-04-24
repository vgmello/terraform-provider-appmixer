package resource_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// seedServiceConfig POSTs a service-config payload directly to the mock,
// bypassing Terraform. Used to simulate a pre-existing server-side config
// that we then import.
func seedServiceConfig(t *testing.T, payload map[string]any) {
	t.Helper()
	baseURL := os.Getenv("APPMIXER_BASE_URL")
	if baseURL == "" {
		t.Fatal("APPMIXER_BASE_URL not set — mock server not running")
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", baseURL+"/service-config", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer mock-token")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("seed service-config: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("seed service-config: want 200, got %d", resp.StatusCode)
	}
}

func TestAccServiceConfig_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "appmixer_service_config" "google" {
  service_id = "appmixer:google-basic"
  fields = {
    client_id = "id-123"
  }
  sensitive_fields = {
    client_secret = "secret-456"
  }
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("appmixer_service_config.google", "service_id", "appmixer:google-basic"),
					resource.TestCheckResourceAttr("appmixer_service_config.google", "fields.client_id", "id-123"),
					resource.TestCheckResourceAttr("appmixer_service_config.google", "sensitive_fields.client_secret", "secret-456"),
					resource.TestCheckResourceAttr("appmixer_service_config.google", "id", "appmixer:google-basic"),
				),
			},
		},
	})
}

func TestAccServiceConfig_rejectsKeyCollision(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "appmixer_service_config" "dup" {
  service_id = "appmixer:collision-test"
  fields = {
    shared = "plain"
  }
  sensitive_fields = {
    shared = "secret"
  }
}
`,
				ExpectError: regexp.MustCompile(`appears in both fields and sensitive_fields`),
			},
		},
	})
}

func TestAccServiceConfig_updatesFieldsViaPUT(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "appmixer_service_config" "u" {
  service_id = "appmixer:update-test"
  fields = {
    client_id = "first"
  }
  sensitive_fields = {
    client_secret = "secret-one"
  }
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("appmixer_service_config.u", "fields.client_id", "first"),
					resource.TestCheckResourceAttr("appmixer_service_config.u", "sensitive_fields.client_secret", "secret-one"),
				),
			},
			{
				Config: `
resource "appmixer_service_config" "u" {
  service_id = "appmixer:update-test"
  fields = {
    client_id = "second"
  }
  sensitive_fields = {
    client_secret = "secret-one"
  }
}
`,
				Check: resource.TestCheckResourceAttr("appmixer_service_config.u", "fields.client_id", "second"),
			},
		},
	})
}

func TestAccServiceConfig_import(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "appmixer_service_config" "i" {
  service_id = "appmixer:import-test"
  fields = {
    client_id = "imported"
  }
  sensitive_fields = {
    client_secret = "imported-secret"
  }
}
`,
			},
			{
				ResourceName:      "appmixer_service_config.i",
				ImportState:       true,
				ImportStateId:     "appmixer:import-test",
				ImportStateVerify: true,
				// Per design spec: on a fresh import the provider cannot know
				// which keys were intended as sensitive, so everything lands
				// in `sensitive_fields` (the safe default — secrets stay
				// redacted). The first plan after import shows the partition
				// as drift, which the operator resolves by moving non-secrets
				// into `fields` in HCL.
				ImportStateVerifyIgnore: []string{"fields", "sensitive_fields"},
			},
		},
	})
}

// TestAccServiceConfig_importDefaultsToSensitive seeds a pre-existing
// service config directly on the mock (bypassing Terraform), then imports
// it and asserts the Read bucketing defaults unknown keys to
// `sensitive_fields`, with `fields` null. This is the safe-by-default
// behavior: on a fresh import the provider has no prior bucket partition,
// so it must assume every key could be a secret and keep them redacted.
func TestAccServiceConfig_importDefaultsToSensitive(t *testing.T) {
	const serviceID = "appmixer:import-defaults-sensitive"
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					seedServiceConfig(t, map[string]any{
						"serviceId":     serviceID,
						"client_id":     "preseed-id",
						"client_secret": "preseed-secret",
						"region":        "us-east-1",
					})
				},
				// Minimal config: service_id only. The resource is imported
				// in the next step; this config exists so the ImportState
				// step has a resource address to target.
				Config: fmt.Sprintf(`
resource "appmixer_service_config" "s" {
  service_id = %q
  sensitive_fields = {
    client_id     = "preseed-id"
    client_secret = "preseed-secret"
    region        = "us-east-1"
  }
}
`, serviceID),
				ResourceName:  "appmixer_service_config.s",
				ImportState:   true,
				ImportStateId: serviceID,
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					if len(states) != 1 {
						return fmt.Errorf("expected 1 imported state, got %d", len(states))
					}
					attrs := states[0].Attributes
					// All three seeded keys must be in sensitive_fields.
					wantSensitive := map[string]string{
						"sensitive_fields.client_id":     "preseed-id",
						"sensitive_fields.client_secret": "preseed-secret",
						"sensitive_fields.region":        "us-east-1",
						"sensitive_fields.%":             "3",
					}
					for k, want := range wantSensitive {
						if got := attrs[k]; got != want {
							return fmt.Errorf("attr %q: want %q, got %q (all attrs: %v)", k, want, got, attrs)
						}
					}
					// fields must be null/empty. Terraform represents a null
					// map attribute by the absence of the `fields.%` count
					// attribute (or it being "0"). Any non-zero count means
					// a key leaked into fields — regression.
					if c := attrs["fields.%"]; c != "" && c != "0" {
						return fmt.Errorf("fields should be null after import, got count=%s attrs=%v", c, attrs)
					}
					return nil
				},
			},
		},
	})
}
