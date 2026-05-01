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

// readServiceConfigServer returns the raw server-side config for serviceID, or
// nil if absent. Used to assert that merge mode preserves out-of-band keys.
func readServiceConfigServer(t *testing.T, serviceID string) map[string]any {
	t.Helper()
	baseURL := os.Getenv("APPMIXER_BASE_URL")
	req, _ := http.NewRequest("GET", baseURL+"/service-config/"+serviceID, nil)
	req.Header.Set("Authorization", "Bearer mock-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("read service-config %q: %v", serviceID, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return nil
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode service-config %q: %v", serviceID, err)
	}
	return out
}

func TestAccServiceConfig_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "appmixer_service_config" "google" {
  service_id = "appmixer:google-basic"
  items = {
    client_id = "id-123"
  }
  sensitive_items = {
    client_secret = "secret-456"
  }
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("appmixer_service_config.google", "service_id", "appmixer:google-basic"),
					resource.TestCheckResourceAttr("appmixer_service_config.google", "items.client_id", "id-123"),
					resource.TestCheckResourceAttr("appmixer_service_config.google", "sensitive_items.client_secret", "secret-456"),
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
  items = {
    shared = "plain"
  }
  sensitive_items = {
    shared = "secret"
  }
}
`,
				ExpectError: regexp.MustCompile(`appears in both items and sensitive_items`),
			},
		},
	})
}

func TestAccServiceConfig_updatesItemsViaPUT(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "appmixer_service_config" "u" {
  service_id = "appmixer:update-test"
  items = {
    client_id = "first"
  }
  sensitive_items = {
    client_secret = "secret-one"
  }
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("appmixer_service_config.u", "items.client_id", "first"),
					resource.TestCheckResourceAttr("appmixer_service_config.u", "sensitive_items.client_secret", "secret-one"),
				),
			},
			{
				Config: `
resource "appmixer_service_config" "u" {
  service_id = "appmixer:update-test"
  items = {
    client_id = "second"
  }
  sensitive_items = {
    client_secret = "secret-one"
  }
}
`,
				Check: resource.TestCheckResourceAttr("appmixer_service_config.u", "items.client_id", "second"),
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
  items = {
    client_id = "imported"
  }
  sensitive_items = {
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
				// in `sensitive_items` (the safe default — secrets stay
				// redacted). The first plan after import shows the partition
				// as drift, which the operator resolves by moving non-secrets
				// into `items` in HCL.
				ImportStateVerifyIgnore: []string{"items", "sensitive_items"},
			},
		},
	})
}

// TestAccServiceConfig_unknownMapValues reproduces the "Value Conversion Error:
// Received unknown value, however the target type cannot handle unknown values"
// bug. When sensitive_items (or items) references a computed attribute that is
// unknown at plan time, the noDuplicateKeysValidator used to call
// ElementsAs into map[string]string, which returns a Value Conversion Error
// diagnostic on unknown element values. The fix switches the target type to
// map[string]types.String, which can hold unknown values without error.
func TestAccServiceConfig_unknownMapValues(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				// terraform_data.secret.output is unknown during the first
				// plan (the resource hasn't been created yet). This triggers
				// the validator with an unknown map-element value.
				Config: `
resource "terraform_data" "secret" {
  input = "my-webhook-secret"
}

resource "appmixer_service_config" "main" {
  service_id = "appmixer:unknown-val-test"
  sensitive_items = {
    webhookSigningSecretPrimary = terraform_data.secret.output
  }
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("appmixer_service_config.main", "service_id", "appmixer:unknown-val-test"),
					resource.TestCheckResourceAttr("appmixer_service_config.main", "sensitive_items.webhookSigningSecretPrimary", "my-webhook-secret"),
				),
			},
		},
	})
}

// TestAccServiceConfig_importDefaultsToSensitive seeds a pre-existing
// service config directly on the mock (bypassing Terraform), then imports
// it and asserts the Read bucketing defaults unknown keys to
// `sensitive_items`, with `items` null. This is the safe-by-default
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
  sensitive_items = {
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
					// All three seeded keys must be in sensitive_items.
					wantSensitive := map[string]string{
						"sensitive_items.client_id":     "preseed-id",
						"sensitive_items.client_secret": "preseed-secret",
						"sensitive_items.region":        "us-east-1",
						"sensitive_items.%":             "3",
					}
					for k, want := range wantSensitive {
						if got := attrs[k]; got != want {
							return fmt.Errorf("attr %q: want %q, got %q (all attrs: %v)", k, want, got, attrs)
						}
					}
					// items must be null/empty. Terraform represents a null
					// map attribute by the absence of the `items.%` count
					// attribute (or it being "0"). Any non-zero count means
					// a key leaked into items — regression.
					if c := attrs["items.%"]; c != "" && c != "0" {
						return fmt.Errorf("items should be null after import, got count=%s attrs=%v", c, attrs)
					}
					return nil
				},
			},
		},
	})
}

// TestAccServiceConfig_mergePreservesExternalOnCreate seeds an out-of-band
// service-config payload and asserts that creating a `mode = "merge"` resource
// keeps the externally-managed keys alongside the Terraform-declared ones.
func TestAccServiceConfig_mergePreservesExternalOnCreate(t *testing.T) {
	const serviceID = "appmixer:merge-create"
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					seedServiceConfig(t, map[string]any{
						"serviceId":  serviceID,
						"externalA":  "keep-me",
						"externalB":  "also-keep",
					})
				},
				Config: fmt.Sprintf(`
resource "appmixer_service_config" "m" {
  service_id = %q
  mode       = "merge"
  items = {
    client_id = "tf-id"
  }
  sensitive_items = {
    client_secret = "tf-secret"
  }
}
`, serviceID),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("appmixer_service_config.m", "mode", "merge"),
					resource.TestCheckResourceAttr("appmixer_service_config.m", "items.client_id", "tf-id"),
					resource.TestCheckResourceAttr("appmixer_service_config.m", "sensitive_items.client_secret", "tf-secret"),
					// Externally-managed keys should NOT appear in TF state in merge mode.
					resource.TestCheckNoResourceAttr("appmixer_service_config.m", "items.externalA"),
					resource.TestCheckNoResourceAttr("appmixer_service_config.m", "sensitive_items.externalA"),
					func(s *terraform.State) error {
						got := readServiceConfigServer(t, serviceID)
						if got == nil {
							return fmt.Errorf("service-config %q missing on server", serviceID)
						}
						for _, k := range []string{"externalA", "externalB", "client_id", "client_secret"} {
							if _, ok := got[k]; !ok {
								return fmt.Errorf("server missing key %q (got %v)", k, got)
							}
						}
						if got["externalA"] != "keep-me" {
							return fmt.Errorf("externalA: want %q, got %v", "keep-me", got["externalA"])
						}
						return nil
					},
				),
			},
		},
	})
}

// TestAccServiceConfig_mergeUpdateAddRemoveKeys verifies the merge-mode
// Update path: keys removed from config get deleted server-side, but
// out-of-band keys still survive across the apply.
func TestAccServiceConfig_mergeUpdateAddRemoveKeys(t *testing.T) {
	const serviceID = "appmixer:merge-update"
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					seedServiceConfig(t, map[string]any{
						"serviceId": serviceID,
						"external":  "stays",
					})
				},
				Config: fmt.Sprintf(`
resource "appmixer_service_config" "u" {
  service_id = %q
  mode       = "merge"
  items = {
    a = "1"
    b = "2"
  }
}
`, serviceID),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("appmixer_service_config.u", "items.a", "1"),
					resource.TestCheckResourceAttr("appmixer_service_config.u", "items.b", "2"),
				),
			},
			{
				// Drop `b`, change `a`, add `c`.
				Config: fmt.Sprintf(`
resource "appmixer_service_config" "u" {
  service_id = %q
  mode       = "merge"
  items = {
    a = "one"
    c = "3"
  }
}
`, serviceID),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("appmixer_service_config.u", "items.a", "one"),
					resource.TestCheckResourceAttr("appmixer_service_config.u", "items.c", "3"),
					resource.TestCheckNoResourceAttr("appmixer_service_config.u", "items.b"),
					func(s *terraform.State) error {
						got := readServiceConfigServer(t, serviceID)
						if got == nil {
							return fmt.Errorf("service-config %q missing on server", serviceID)
						}
						if _, ok := got["b"]; ok {
							return fmt.Errorf("expected key %q to be removed server-side, got %v", "b", got)
						}
						if got["external"] != "stays" {
							return fmt.Errorf("external key clobbered: got %v", got["external"])
						}
						if got["a"] != "one" || got["c"] != "3" {
							return fmt.Errorf("merged keys wrong: got %v", got)
						}
						return nil
					},
				),
			},
		},
	})
}

// TestAccServiceConfig_mergeDeletePreservesExternal verifies that destroying
// a merge-mode resource removes only the keys it declared, leaving any
// externally-managed keys behind.
func TestAccServiceConfig_mergeDeletePreservesExternal(t *testing.T) {
	const serviceID = "appmixer:merge-delete"
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					seedServiceConfig(t, map[string]any{
						"serviceId": serviceID,
						"keepme":    "yes",
					})
				},
				Config: fmt.Sprintf(`
resource "appmixer_service_config" "d" {
  service_id = %q
  mode       = "merge"
  items = {
    transient = "bye"
  }
}
`, serviceID),
				Check: resource.TestCheckResourceAttr("appmixer_service_config.d", "items.transient", "bye"),
			},
			{
				// Remove the resource from config — triggers Delete.
				Config: ` `,
				Check: func(s *terraform.State) error {
					got := readServiceConfigServer(t, serviceID)
					if got == nil {
						return fmt.Errorf("service-config %q was wiped — merge delete should preserve external keys", serviceID)
					}
					if _, ok := got["transient"]; ok {
						return fmt.Errorf("managed key %q should have been removed, got %v", "transient", got)
					}
					if got["keepme"] != "yes" {
						return fmt.Errorf("external key %q lost, got %v", "keepme", got)
					}
					return nil
				},
			},
		},
	})
}

// TestAccServiceConfig_mergeReadIgnoresExternal verifies that Read in merge
// mode does not surface server-side keys outside the resource's declared
// set — i.e., out-of-band additions don't show up as drift.
func TestAccServiceConfig_mergeReadIgnoresExternal(t *testing.T) {
	const serviceID = "appmixer:merge-read"
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "appmixer_service_config" "r" {
  service_id = %q
  mode       = "merge"
  items = {
    mine = "1"
  }
}
`, serviceID),
			},
			{
				// Inject an out-of-band key, then refresh and re-plan.
				PreConfig: func() {
					seedServiceConfig(t, map[string]any{
						"serviceId": serviceID,
						"mine":      "1",
						"alien":     "from-outside",
					})
				},
				RefreshState: true,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("appmixer_service_config.r", "items.mine", "1"),
					resource.TestCheckNoResourceAttr("appmixer_service_config.r", "items.alien"),
					resource.TestCheckNoResourceAttr("appmixer_service_config.r", "sensitive_items.alien"),
				),
			},
			{
				// Re-running the same config should produce no plan drift in merge mode.
				Config: fmt.Sprintf(`
resource "appmixer_service_config" "r" {
  service_id = %q
  mode       = "merge"
  items = {
    mine = "1"
  }
}
`, serviceID),
				PlanOnly: true,
			},
		},
	})
}
