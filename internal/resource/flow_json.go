package resource

import (
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// serverOwnedComponentField is the flow-descriptor field Appmixer rewrites on
// write: every component is upgraded to the newest version installed on the
// tenant, so a descriptor pinning "1.4.5" reads back as "1.4.8".
const serverOwnedComponentField = "version"

// isComponentNode reports whether m looks like an Appmixer component node: an
// object carrying both a "type" selector and a "version". The pairing matters —
// a bare "version" key elsewhere in the descriptor is user data, not a
// component version, and must not be reconciled away.
func isComponentNode(m map[string]any) bool {
	if _, ok := m["type"].(string); !ok {
		return false
	}
	_, ok := m[serverOwnedComponentField]
	return ok
}

// reconcileFlow returns the server's flow document with server-owned component
// fields restored from the prior state document. Appmixer upgrades component
// versions server-side, and surfacing that as drift produces a perpetual diff
// the user cannot resolve. Every other difference is genuine drift and is
// preserved from the server copy.
func reconcileFlow(prior, server any) any {
	switch srv := server.(type) {
	case map[string]any:
		pri, ok := prior.(map[string]any)
		if !ok {
			return server
		}
		out := make(map[string]any, len(srv))
		for k, v := range srv {
			out[k] = reconcileFlow(pri[k], v)
		}
		if isComponentNode(srv) && isComponentNode(pri) {
			out[serverOwnedComponentField] = pri[serverOwnedComponentField]
		}
		return out
	case []any:
		pri, ok := prior.([]any)
		if !ok || len(pri) != len(srv) {
			return server
		}
		out := make([]any, len(srv))
		for i, v := range srv {
			out[i] = reconcileFlow(pri[i], v)
		}
		return out
	default:
		return server
	}
}

// reconcileFlowJSON re-serialises the server flow with server-owned component
// fields taken from the prior state value. ok is false when there is no usable
// prior value (fresh import, unparseable state, empty server response), in
// which case the caller keeps the server copy verbatim.
func reconcileFlowJSON(prior types.String, server map[string]any) (types.String, bool) {
	if server == nil || prior.IsNull() || prior.IsUnknown() {
		return types.StringNull(), false
	}
	var priorDoc any
	if err := json.Unmarshal([]byte(prior.ValueString()), &priorDoc); err != nil {
		return types.StringNull(), false
	}
	b, err := json.Marshal(reconcileFlow(priorDoc, any(server)))
	if err != nil {
		return types.StringNull(), false
	}
	return types.StringValue(string(b)), true
}
