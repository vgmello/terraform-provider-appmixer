// Package apitypes holds API wire types and helpers shared between the
// resource and datasource packages. Keeping them here avoids cyclic imports
// and eliminates duplicate struct definitions.
package apitypes

import (
	"fmt"
	"math/big"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// FlowWire is the JSON representation returned by GET/POST/PUT /flows.
type FlowWire struct {
	FlowID       string           `json:"flowId"`
	Name         string           `json:"name"`
	Flow         map[string]any   `json:"flow"`
	CustomFields map[string]any   `json:"customFields"`
	SharedWith   []map[string]any `json:"sharedWith,omitempty"`
	Stage        string           `json:"stage"`
}

// UserWire is the JSON representation returned by GET /users/:id.
type UserWire struct {
	UserID   string            `json:"userId"`
	Username string            `json:"username"`
	Scope    []string          `json:"scope"`
	Metadata map[string]string `json:"metadata"`
}

// BuildCustomFieldsDynamic converts a server-side customFields map into a
// Terraform types.Dynamic. A nil map (field absent from the API response)
// becomes types.DynamicNull; an explicitly empty map becomes an empty object
// so that `custom_fields = {}` does not produce a perpetual diff.
func BuildCustomFieldsDynamic(cf map[string]any) types.Dynamic {
	if cf == nil {
		return types.DynamicNull()
	}
	if len(cf) == 0 {
		obj, diags := types.ObjectValue(map[string]attr.Type{}, map[string]attr.Value{})
		if diags.HasError() {
			return types.DynamicNull()
		}
		return types.DynamicValue(obj)
	}
	attrTypes := make(map[string]attr.Type, len(cf))
	attrVals := make(map[string]attr.Value, len(cf))
	for k, v := range cf {
		switch tv := v.(type) {
		case bool:
			attrTypes[k] = types.BoolType
			attrVals[k] = types.BoolValue(tv)
		case float64:
			attrTypes[k] = types.NumberType
			attrVals[k] = types.NumberValue(big.NewFloat(tv))
		case string:
			attrTypes[k] = types.StringType
			attrVals[k] = types.StringValue(tv)
		default:
			attrTypes[k] = types.StringType
			attrVals[k] = types.StringValue(fmt.Sprintf("%v", tv))
		}
	}
	obj, diags := types.ObjectValue(attrTypes, attrVals)
	if diags.HasError() {
		return types.DynamicNull()
	}
	return types.DynamicValue(obj)
}

// SharedWithAttrTypes defines the Terraform attribute types for a shared_with entry.
var SharedWithAttrTypes = map[string]attr.Type{
	"permissions": types.ListType{ElemType: types.StringType},
	"scope":       types.StringType,
	"email":       types.StringType,
	"domain":      types.StringType,
}

// SharedWithObjectType is the Terraform object type for a shared_with entry.
var SharedWithObjectType = types.ObjectType{AttrTypes: SharedWithAttrTypes}

// BuildSharedWithList converts a server-side sharedWith slice into a Terraform
// types.List. A nil slice (field absent from the API response) becomes
// types.ListNull; an explicitly empty slice becomes an empty list value so
// that `shared_with = []` does not produce a perpetual diff.
func BuildSharedWithList(sw []map[string]any) (types.List, error) {
	if sw == nil {
		return types.ListNull(SharedWithObjectType), nil
	}
	if len(sw) == 0 {
		return types.ListValueMust(SharedWithObjectType, []attr.Value{}), nil
	}
	items := make([]attr.Value, 0, len(sw))
	for _, m := range sw {
		rawPerms, _ := m["permissions"].([]any)
		permVals := make([]attr.Value, 0, len(rawPerms))
		for _, p := range rawPerms {
			permVals = append(permVals, types.StringValue(fmt.Sprintf("%v", p)))
		}
		permList, diags := types.ListValue(types.StringType, permVals)
		if diags.HasError() {
			return types.ListNull(SharedWithObjectType), fmt.Errorf("building permissions list")
		}

		scope := types.StringNull()
		if s, ok := m["scope"].(string); ok {
			scope = types.StringValue(s)
		}
		email := types.StringNull()
		if e, ok := m["email"].(string); ok {
			email = types.StringValue(e)
		}
		domain := types.StringNull()
		if d, ok := m["domain"].(string); ok {
			domain = types.StringValue(d)
		}

		obj, diags := types.ObjectValue(SharedWithAttrTypes, map[string]attr.Value{
			"permissions": permList,
			"scope":       scope,
			"email":       email,
			"domain":      domain,
		})
		if diags.HasError() {
			return types.ListNull(SharedWithObjectType), fmt.Errorf("building shared_with object for entry %v", m)
		}
		items = append(items, obj)
	}
	list, diags := types.ListValue(SharedWithObjectType, items)
	if diags.HasError() {
		return types.ListNull(SharedWithObjectType), fmt.Errorf("building shared_with list")
	}
	return list, nil
}
