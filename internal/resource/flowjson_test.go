package resource

import "testing"

func TestFlowJSONSemanticallyEqual(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want bool
	}{
		{
			name: "identical documents",
			a:    `{"n1":{"type":"appmixer.utils.controls.Each","version":"1.4.5"}}`,
			b:    `{"n1":{"type":"appmixer.utils.controls.Each","version":"1.4.5"}}`,
			want: true,
		},
		{
			name: "node version rewritten by server",
			a:    `{"n1":{"type":"appmixer.utils.controls.Each","version":"1.4.5","x":100}}`,
			b:    `{"n1":{"type":"appmixer.utils.controls.Each","version":"1.4.8","x":100}}`,
			want: true,
		},
		{
			name: "node version added by server",
			a:    `{"n1":{"type":"mews.automation.reservations.AddNote"}}`,
			b:    `{"n1":{"type":"mews.automation.reservations.AddNote","version":"1.0.0"}}`,
			want: true,
		},
		{
			name: "key order and whitespace ignored",
			a:    `{"n1": {"version": "1.4.5", "type": "appmixer.utils.controls.Each"}}`,
			b:    `{"n1":{"type":"appmixer.utils.controls.Each","version":"1.4.8"}}`,
			want: true,
		},
		{
			name: "non-version node field differs",
			a:    `{"n1":{"type":"appmixer.utils.controls.Each","version":"1.4.5","x":100}}`,
			b:    `{"n1":{"type":"appmixer.utils.controls.Each","version":"1.4.5","x":200}}`,
			want: false,
		},
		{
			name: "node type differs",
			a:    `{"n1":{"type":"appmixer.utils.controls.Each","version":"1.4.5"}}`,
			b:    `{"n1":{"type":"appmixer.utils.timers.Scheduler","version":"1.4.5"}}`,
			want: false,
		},
		{
			name: "node added",
			a:    `{"n1":{"type":"t","version":"1"}}`,
			b:    `{"n1":{"type":"t","version":"1"},"n2":{"type":"t","version":"1"}}`,
			want: false,
		},
		{
			name: "nested version inside config still compared",
			a:    `{"n1":{"type":"t","config":{"properties":{"version":"a"}}}}`,
			b:    `{"n1":{"type":"t","config":{"properties":{"version":"b"}}}}`,
			want: false,
		},
		{
			name: "top-level non-object version key still compared",
			a:    `{"nodes":[],"version":"1"}`,
			b:    `{"nodes":[],"version":"2"}`,
			want: false,
		},
		{
			name: "invalid JSON falls back to exact equality",
			a:    `not-json`,
			b:    `not-json`,
			want: true,
		},
		{
			name: "invalid JSON differs from valid",
			a:    `not-json`,
			b:    `{}`,
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := flowJSONSemanticallyEqual(tc.a, tc.b); got != tc.want {
				t.Errorf("flowJSONSemanticallyEqual(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
			// The comparison must be symmetric.
			if got := flowJSONSemanticallyEqual(tc.b, tc.a); got != tc.want {
				t.Errorf("flowJSONSemanticallyEqual(%q, %q) = %v, want %v (asymmetric)", tc.b, tc.a, got, tc.want)
			}
		})
	}
}
