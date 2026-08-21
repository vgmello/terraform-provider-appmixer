package resource

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func mustJSON(t *testing.T, s string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("invalid test JSON %q: %s", s, err)
	}
	return v
}

func TestReconcileFlow(t *testing.T) {
	tests := []struct {
		name   string
		prior  string
		server string
		want   string
	}{
		{
			name:   "component version upgrade is reconciled away",
			prior:  `{"c1":{"type":"appmixer.utils.controls.Each","version":"1.4.5","x":1312}}`,
			server: `{"c1":{"type":"appmixer.utils.controls.Each","version":"1.4.8","x":1312}}`,
			want:   `{"c1":{"type":"appmixer.utils.controls.Each","version":"1.4.5","x":1312}}`,
		},
		{
			name:   "real drift alongside a version upgrade is kept",
			prior:  `{"c1":{"type":"a","version":"1.0.0","x":10}}`,
			server: `{"c1":{"type":"a","version":"2.0.0","x":99}}`,
			want:   `{"c1":{"type":"a","version":"1.0.0","x":99}}`,
		},
		{
			name:   "version without a sibling type is user data and is kept",
			prior:  `{"nodes":[],"version":"2"}`,
			server: `{"nodes":[],"version":"3"}`,
			want:   `{"nodes":[],"version":"3"}`,
		},
		{
			name:   "nested component versions are reconciled",
			prior:  `{"outer":{"type":"a","version":"1.0.0","inner":{"type":"b","version":"1.1.0"}}}`,
			server: `{"outer":{"type":"a","version":"2.0.0","inner":{"type":"b","version":"2.2.0"}}}`,
			want:   `{"outer":{"type":"a","version":"1.0.0","inner":{"type":"b","version":"1.1.0"}}}`,
		},
		{
			name:   "components inside arrays are reconciled elementwise",
			prior:  `{"nodes":[{"type":"a","version":"1.0.0"},{"type":"b","version":"1.0.0"}]}`,
			server: `{"nodes":[{"type":"a","version":"9.9.9"},{"type":"b","version":"9.9.9"}]}`,
			want:   `{"nodes":[{"type":"a","version":"1.0.0"},{"type":"b","version":"1.0.0"}]}`,
		},
		{
			name:   "array length change is treated as drift and taken from the server",
			prior:  `{"nodes":[{"type":"a","version":"1.0.0"}]}`,
			server: `{"nodes":[{"type":"a","version":"9.9.9"},{"type":"b","version":"9.9.9"}]}`,
			want:   `{"nodes":[{"type":"a","version":"9.9.9"},{"type":"b","version":"9.9.9"}]}`,
		},
		{
			name:   "component added server-side is surfaced as drift",
			prior:  `{"c1":{"type":"a","version":"1.0.0"}}`,
			server: `{"c1":{"type":"a","version":"9.9.9"},"c2":{"type":"b","version":"9.9.9"}}`,
			want:   `{"c1":{"type":"a","version":"1.0.0"},"c2":{"type":"b","version":"9.9.9"}}`,
		},
		{
			name:   "component removed server-side is surfaced as drift",
			prior:  `{"c1":{"type":"a","version":"1.0.0"},"c2":{"type":"b","version":"1.0.0"}}`,
			server: `{"c1":{"type":"a","version":"9.9.9"}}`,
			want:   `{"c1":{"type":"a","version":"1.0.0"}}`,
		},
		{
			name:   "type change makes the version a genuine difference",
			prior:  `{"c1":{"type":"a","version":"1.0.0"}}`,
			server: `{"c1":{"type":"b","version":"9.9.9"}}`,
			want:   `{"c1":{"type":"b","version":"1.0.0"}}`,
		},
		{
			name:   "prior node of a different shape yields the server value",
			prior:  `{"c1":"scalar"}`,
			server: `{"c1":{"type":"a","version":"9.9.9"}}`,
			want:   `{"c1":{"type":"a","version":"9.9.9"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reconcileFlow(mustJSON(t, tt.prior), mustJSON(t, tt.server))
			b, err := json.Marshal(got)
			if err != nil {
				t.Fatalf("marshal result: %s", err)
			}
			wantB, err := json.Marshal(mustJSON(t, tt.want))
			if err != nil {
				t.Fatalf("marshal want: %s", err)
			}
			if string(b) != string(wantB) {
				t.Errorf("reconcileFlow()\n got: %s\nwant: %s", b, wantB)
			}
		})
	}
}

func TestReconcileFlowJSON(t *testing.T) {
	server := map[string]any{
		"c1": map[string]any{"type": "a", "version": "9.9.9"},
	}

	t.Run("reconciles against a usable prior value", func(t *testing.T) {
		prior := types.StringValue(`{"c1":{"type":"a","version":"1.0.0"}}`)
		got, ok := reconcileFlowJSON(prior, server)
		if !ok {
			t.Fatal("expected ok=true for a parseable prior value")
		}
		if want := `{"c1":{"type":"a","version":"1.0.0"}}`; got.ValueString() != want {
			t.Errorf("got %s, want %s", got.ValueString(), want)
		}
	})

	t.Run("falls back to the server copy without a prior value", func(t *testing.T) {
		for name, prior := range map[string]types.String{
			"null":        types.StringNull(),
			"unknown":     types.StringUnknown(),
			"unparseable": types.StringValue("not json"),
		} {
			if _, ok := reconcileFlowJSON(prior, server); ok {
				t.Errorf("%s prior: expected ok=false", name)
			}
		}
	})

	t.Run("falls back on an empty server response", func(t *testing.T) {
		prior := types.StringValue(`{"c1":{"type":"a","version":"1.0.0"}}`)
		if _, ok := reconcileFlowJSON(prior, nil); ok {
			t.Error("expected ok=false for a nil server flow")
		}
	})
}
