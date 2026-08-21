package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// flowJSONType is the schema type for flow_json. Its value's semantic equality
// ignores each node's "version": the Appmixer server rewrites component
// versions to the tenant's installed version when a flow is saved, so comparing
// the raw strings would fail Terraform's post-apply consistency check and
// produce perpetual refresh drift.
type flowJSONType struct {
	basetypes.StringType
}

var _ basetypes.StringTypable = flowJSONType{}

func (t flowJSONType) Equal(o attr.Type) bool {
	other, ok := o.(flowJSONType)
	if !ok {
		return false
	}
	return t.StringType.Equal(other.StringType)
}

func (t flowJSONType) String() string { return "flowJSONType" }

func (t flowJSONType) ValueFromString(_ context.Context, in basetypes.StringValue) (basetypes.StringValuable, diag.Diagnostics) {
	return flowJSONValue{StringValue: in}, nil
}

func (t flowJSONType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	attrValue, err := t.StringType.ValueFromTerraform(ctx, in)
	if err != nil {
		return nil, err
	}
	stringValue, ok := attrValue.(basetypes.StringValue)
	if !ok {
		return nil, fmt.Errorf("unexpected value type %T, expected basetypes.StringValue", attrValue)
	}
	value, diags := t.ValueFromString(ctx, stringValue)
	if diags.HasError() {
		return nil, fmt.Errorf("converting StringValue to flowJSONValue: %v", diags)
	}
	return value, nil
}

func (t flowJSONType) ValueType(_ context.Context) attr.Value {
	return flowJSONValue{}
}

type flowJSONValue struct {
	basetypes.StringValue
}

var _ basetypes.StringValuableWithSemanticEquals = flowJSONValue{}

func newFlowJSONValue(s string) flowJSONValue {
	return flowJSONValue{StringValue: basetypes.NewStringValue(s)}
}

func (v flowJSONValue) Equal(o attr.Value) bool {
	other, ok := o.(flowJSONValue)
	if !ok {
		return false
	}
	return v.StringValue.Equal(other.StringValue)
}

func (v flowJSONValue) Type(_ context.Context) attr.Type {
	return flowJSONType{}
}

func (v flowJSONValue) StringSemanticEquals(_ context.Context, newValuable basetypes.StringValuable) (bool, diag.Diagnostics) {
	var diags diag.Diagnostics
	newValue, ok := newValuable.(flowJSONValue)
	if !ok {
		diags.AddError(
			"Semantic equality check failed",
			fmt.Sprintf("Expected flowJSONValue, got %T. Please report this issue to the provider developers.", newValuable),
		)
		return false, diags
	}
	return flowJSONSemanticallyEqual(v.ValueString(), newValue.ValueString()), diags
}

// flowJSONSemanticallyEqual reports whether two flow documents are equivalent,
// ignoring JSON formatting, key order, and each node's "version". Only node
// versions are ignored (top-level object values); a "version" key anywhere else
// still participates in the comparison. Falls back to exact string comparison
// when either side is not a JSON object.
func flowJSONSemanticallyEqual(a, b string) bool {
	docA, okA := decodeFlowDroppingNodeVersions(a)
	docB, okB := decodeFlowDroppingNodeVersions(b)
	if !okA || !okB {
		return a == b
	}
	return reflect.DeepEqual(docA, docB)
}

func decodeFlowDroppingNodeVersions(s string) (map[string]any, bool) {
	var doc map[string]any
	if err := json.Unmarshal([]byte(s), &doc); err != nil {
		return nil, false
	}
	for _, node := range doc {
		if nodeMap, ok := node.(map[string]any); ok {
			delete(nodeMap, "version")
		}
	}
	return doc, true
}
