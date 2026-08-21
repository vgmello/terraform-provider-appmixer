package resource

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// serverOwnedComponentField is the flow-descriptor field Appmixer decides on
// write: every component is upgraded to the newest version installed on the
// tenant, so a descriptor pinning "1.4.5" reads back as "1.4.8", and a
// descriptor that pins nothing reads back with a version it never declared.
const serverOwnedComponentField = "version"

// componentType returns the component selector of a flow node and whether the
// node is a component at all. A component is an object carrying a "type"
// string; its "version" may be absent, because Appmixer fills one in when the
// descriptor does not pin it.
func componentType(m map[string]any) (string, bool) {
	t, ok := m["type"].(string)
	return t, ok
}

// reconcileFlow returns the server's flow document with server-owned component
// fields restored from the prior state document. Appmixer decides a component's
// version server-side — it upgrades a pinned one and supplies a missing one —
// and surfacing that as drift produces a perpetual diff the user cannot
// resolve. Every other difference is genuine drift and is preserved from the
// server copy.
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
		// The version of a component that still has the same type belongs
		// wholly to prior state: restore it, or drop it when the descriptor
		// never carried one and the server filled it in. Matching on type
		// matters — once a node's type changes the component is a different
		// one, and the version that came back with it is not a rewrite of the
		// old one but genuine drift.
		if srvType, ok := componentType(srv); ok {
			if priType, ok := componentType(pri); ok && priType == srvType {
				if v, had := pri[serverOwnedComponentField]; had {
					out[serverOwnedComponentField] = v
				} else {
					delete(out, serverOwnedComponentField)
				}
			}
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

// maxReportedFlowDiffs bounds the paths named in the rewrite warning. A server
// that rewrites the descriptor wholesale would otherwise produce a diagnostic
// nobody reads.
const maxReportedFlowDiffs = 5

// flowDiffPaths lists dotted paths at which two flow documents differ, stopping
// once limit paths have been collected. A differing subtree is reported at the
// point it diverges rather than at every leaf below it.
func flowDiffPaths(prior, server any, prefix string, out *[]string, limit int) {
	if len(*out) >= limit {
		return
	}
	switch srv := server.(type) {
	case map[string]any:
		pri, ok := prior.(map[string]any)
		if !ok {
			*out = append(*out, prefix)
			return
		}
		for k, v := range srv {
			if len(*out) >= limit {
				return
			}
			child := k
			if prefix != "" {
				child = prefix + "." + k
			}
			if _, present := pri[k]; !present {
				*out = append(*out, child)
				continue
			}
			flowDiffPaths(pri[k], v, child, out, limit)
		}
		for k := range pri {
			if len(*out) >= limit {
				return
			}
			if _, present := srv[k]; !present {
				child := k
				if prefix != "" {
					child = prefix + "." + k
				}
				*out = append(*out, child)
			}
		}
	case []any:
		pri, ok := prior.([]any)
		if !ok || len(pri) != len(srv) {
			*out = append(*out, prefix)
			return
		}
		for i, v := range srv {
			if len(*out) >= limit {
				return
			}
			flowDiffPaths(pri[i], v, fmt.Sprintf("%s[%d]", prefix, i), out, limit)
		}
	default:
		if !reflect.DeepEqual(prior, server) {
			*out = append(*out, prefix)
		}
	}
}

// serverRewriteWarning compares the descriptor just written against what the
// server stored, after reconciliation has already absorbed the component
// versions it legitimately owns. Anything still differing is a rewrite the
// provider cannot reconcile: the next plan proposes to undo it, the apply after
// that does not, and the loop repeats. Reporting it here — immediately after
// the write, before anything else could have touched the flow — is what
// separates it from ordinary out-of-band drift, which Read cannot tell apart.
//
// Returns an empty diagnostics when the flow round-tripped intact, or when the
// server response carried no descriptor to compare against.
func serverRewriteWarning(written types.String, serverFlow map[string]any) diag.Diagnostics {
	var diags diag.Diagnostics
	if serverFlow == nil || written.IsNull() || written.IsUnknown() {
		return diags
	}
	var writtenDoc any
	if err := json.Unmarshal([]byte(written.ValueString()), &writtenDoc); err != nil {
		return diags
	}
	reconciled := reconcileFlow(writtenDoc, any(serverFlow))
	if reflect.DeepEqual(writtenDoc, reconciled) {
		return diags
	}
	var paths []string
	flowDiffPaths(writtenDoc, reconciled, "", &paths, maxReportedFlowDiffs)
	sort.Strings(paths)
	diags.AddAttributeWarning(
		path.Root("flow_json"),
		"Appmixer rewrote the flow descriptor",
		fmt.Sprintf(
			"The flow stored by the server differs from the descriptor Terraform sent, at: %s.\n\n"+
				"Terraform keeps your configured value in state, so the next plan will show this as drift "+
				"and applying it will not resolve it. Either match the server's value in your configuration, "+
				"or report the field so the provider can treat it as server-owned.",
			strings.Join(paths, ", "),
		),
	)
	return diags
}
