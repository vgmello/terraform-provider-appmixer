package resource

import (
	"encoding/json"
	"fmt"
	"strings"
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
			// A node whose type changed is a different component, so its
			// version is not a rewrite of the old one — keeping the prior
			// version here would put a type/version pair into state that
			// exists on neither side and understate the real drift.
			name:   "type change makes the version a genuine difference",
			prior:  `{"c1":{"type":"a","version":"1.0.0"}}`,
			server: `{"c1":{"type":"b","version":"9.9.9"}}`,
			want:   `{"c1":{"type":"b","version":"9.9.9"}}`,
		},
		{
			// The descriptor pins no version, so the one the server supplies
			// is server-owned too. Keeping it would propose removing a field
			// that the next apply cannot remove.
			name:   "version supplied by the server for an unpinned component is dropped",
			prior:  `{"c1":{"type":"a","x":10}}`,
			server: `{"c1":{"type":"a","version":"9.9.9","x":10}}`,
			want:   `{"c1":{"type":"a","x":10}}`,
		},
		{
			name:   "real drift alongside a server-supplied version is still kept",
			prior:  `{"c1":{"type":"a","x":10}}`,
			server: `{"c1":{"type":"a","version":"9.9.9","x":99}}`,
			want:   `{"c1":{"type":"a","x":99}}`,
		},
		{
			// The server dropping a pinned version is the mirror image: a
			// plan proposing to add it back would never converge.
			name:   "version dropped by the server is restored from state",
			prior:  `{"c1":{"type":"a","version":"1.0.0"}}`,
			server: `{"c1":{"type":"a"}}`,
			want:   `{"c1":{"type":"a","version":"1.0.0"}}`,
		},
		{
			name:   "non-string type is not a component and is left alone",
			prior:  `{"c1":{"type":7,"version":"1.0.0"}}`,
			server: `{"c1":{"type":7,"version":"9.9.9"}}`,
			want:   `{"c1":{"type":7,"version":"9.9.9"}}`,
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

func TestServerRewriteWarning(t *testing.T) {
	tests := []struct {
		name        string
		written     types.String
		server      map[string]any
		wantWarning bool
		wantPaths   string
	}{
		{
			name:    "component version decided by the server is not a rewrite",
			written: types.StringValue(`{"c1":{"type":"a","version":"1.4.5","x":10}}`),
			server: map[string]any{
				"c1": map[string]any{"type": "a", "version": "9.9.9", "x": float64(10)},
			},
			wantWarning: false,
		},
		{
			name:    "version supplied for an unpinned component is not a rewrite",
			written: types.StringValue(`{"c1":{"type":"a","x":10}}`),
			server: map[string]any{
				"c1": map[string]any{"type": "a", "version": "9.9.9", "x": float64(10)},
			},
			wantWarning: false,
		},
		{
			name:    "a field the server changed is reported",
			written: types.StringValue(`{"c1":{"type":"a","version":"1.0.0","x":10}}`),
			server: map[string]any{
				"c1": map[string]any{"type": "a", "version": "9.9.9", "x": float64(999)},
			},
			wantWarning: true,
			wantPaths:   "c1.x",
		},
		{
			name:    "a field the server added is reported",
			written: types.StringValue(`{"c1":{"type":"a","version":"1.0.0"}}`),
			server: map[string]any{
				"c1": map[string]any{"type": "a", "version": "9.9.9", "retries": float64(3)},
			},
			wantWarning: true,
			wantPaths:   "c1.retries",
		},
		{
			name:    "a field the server dropped is reported",
			written: types.StringValue(`{"c1":{"type":"a","version":"1.0.0","label":"hi"}}`),
			server: map[string]any{
				"c1": map[string]any{"type": "a", "version": "9.9.9"},
			},
			wantWarning: true,
			wantPaths:   "c1.label",
		},
		{
			name:        "no descriptor in the response yields no warning",
			written:     types.StringValue(`{"c1":{"type":"a"}}`),
			server:      nil,
			wantWarning: false,
		},
		{
			name:        "unparseable written value yields no warning",
			written:     types.StringValue("not json"),
			server:      map[string]any{"c1": map[string]any{"type": "a"}},
			wantWarning: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := serverRewriteWarning(tt.written, tt.server)
			if got := diags.WarningsCount() > 0; got != tt.wantWarning {
				t.Fatalf("warning emitted = %v, want %v (diags: %v)", got, tt.wantWarning, diags)
			}
			if !tt.wantWarning {
				return
			}
			if detail := diags.Warnings()[0].Detail(); !strings.Contains(detail, tt.wantPaths) {
				t.Errorf("warning detail %q does not name %q", detail, tt.wantPaths)
			}
		})
	}
}

func TestFlowDiffPathsIsBounded(t *testing.T) {
	prior := map[string]any{}
	server := map[string]any{}
	for i := range 20 {
		server[fmt.Sprintf("k%02d", i)] = i
	}
	var paths []string
	flowDiffPaths(prior, server, "", &paths, maxReportedFlowDiffs)
	if len(paths) > maxReportedFlowDiffs {
		t.Errorf("collected %d paths, want at most %d", len(paths), maxReportedFlowDiffs)
	}
}
