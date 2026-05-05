package resource

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestUpgradeV0CustomFields covers the conversion of the old map(string)
// custom_fields into a DynamicAttribute object. This is the core logic that
// fixes the "invalid key in dynamically-typed value" error from GitHub #15.
func TestUpgradeV0CustomFields(t *testing.T) {
	tests := []struct {
		name       string
		input      map[string]string
		wantNull   bool
		wantKeys   []string
		wantValues map[string]string
	}{
		{
			name:     "nil map becomes DynamicNull",
			input:    nil,
			wantNull: true,
		},
		{
			name:       "empty map becomes empty object",
			input:      map[string]string{},
			wantNull:   false,
			wantKeys:   []string{},
			wantValues: map[string]string{},
		},
		{
			name:       "string values preserved",
			input:      map[string]string{"category": "ops", "env": "prod"},
			wantNull:   false,
			wantKeys:   []string{"category", "env"},
			wantValues: map[string]string{"category": "ops", "env": "prod"},
		},
		{
			// Exact scenario from GitHub issue #15: hyphenated keys that caused
			// "invalid key in dynamically-typed value" during state upgrade.
			name: "hyphenated keys (issue #15)",
			input: map[string]string{
				"mews-template":             "true",
				"mews-template-description": "AutomationHubTemplateRoomUpgradeLoyaltyMemberDescription",
				"mews-template-icon":        "guest_in_house",
				"mews-template-prod":        "true",
			},
			wantNull: false,
			wantKeys: []string{
				"mews-template",
				"mews-template-description",
				"mews-template-icon",
				"mews-template-prod",
			},
			wantValues: map[string]string{
				"mews-template":             "true",
				"mews-template-description": "AutomationHubTemplateRoomUpgradeLoyaltyMemberDescription",
				"mews-template-icon":        "guest_in_house",
				"mews-template-prod":        "true",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := upgradeV0CustomFields(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tc.wantNull {
				if !got.IsNull() {
					t.Errorf("want DynamicNull, got %v", got)
				}
				return
			}

			if got.IsNull() {
				t.Fatal("want non-null Dynamic, got null")
			}

			obj, ok := got.UnderlyingValue().(types.Object)
			if !ok {
				t.Fatalf("want underlying types.Object, got %T", got.UnderlyingValue())
			}

			attrs := obj.Attributes()
			for _, key := range tc.wantKeys {
				val, exists := attrs[key]
				if !exists {
					t.Errorf("expected key %q to be present in object", key)
					continue
				}
				sv, ok := val.(types.String)
				if !ok {
					t.Errorf("key %q: want types.String, got %T", key, val)
					continue
				}
				if want := tc.wantValues[key]; sv.ValueString() != want {
					t.Errorf("key %q: want %q, got %q", key, want, sv.ValueString())
				}
			}
		})
	}
}

// TestFlowResource_HasUpgradeStateV0 verifies the v0 upgrader is registered.
func TestFlowResource_HasUpgradeStateV0(t *testing.T) {
	r := &flowResource{}
	upgraders := r.UpgradeState(context.Background())
	if _, ok := upgraders[0]; !ok {
		t.Fatal("no v0 → v1 upgrader registered in UpgradeState")
	}
}

// TestFlowResource_SchemaVersion verifies the schema declares version 1.
func TestFlowResource_SchemaVersion(t *testing.T) {
	r := &flowResource{}
	var resp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &resp)
	if resp.Schema.Version != 1 {
		t.Errorf("expected schema version 1, got %d", resp.Schema.Version)
	}
}
