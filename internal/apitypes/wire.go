// Package apitypes holds API wire types and helpers shared between the
// resource and datasource packages. Keeping them here avoids cyclic imports
// and eliminates duplicate struct definitions.
package apitypes

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// FlowWire is the JSON representation returned by GET/POST/PUT /flows.
type FlowWire struct {
	FlowID       string         `json:"flowId"`
	Name         string         `json:"name"`
	Flow         map[string]any `json:"flow"`
	CustomFields map[string]any `json:"customFields"`
	Stage        string         `json:"stage"`
}

// UserWire is the JSON representation returned by GET /users/:id.
type UserWire struct {
	UserID   string            `json:"userId"`
	Username string            `json:"username"`
	Scope    []string          `json:"scope"`
	Metadata map[string]string `json:"metadata"`
}

// BuildCustomFieldsMap converts a server-side customFields map into a
// Terraform types.Map. An empty or nil map becomes types.MapNull so that
// a null config attribute does not produce a perpetual diff.
func BuildCustomFieldsMap(cf map[string]any) types.Map {
	if len(cf) == 0 {
		return types.MapNull(types.StringType)
	}
	vals := make(map[string]attr.Value, len(cf))
	for k, v := range cf {
		vals[k] = types.StringValue(fmt.Sprintf("%v", v))
	}
	return types.MapValueMust(types.StringType, vals)
}
