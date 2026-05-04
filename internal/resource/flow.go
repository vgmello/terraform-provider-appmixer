package resource

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ellosoft/terraform-provider-appmixer/internal/apitypes"
	"github.com/ellosoft/terraform-provider-appmixer/internal/client"
)

type flowResource struct {
	client *client.Client
}

func NewFlowResource() resource.Resource { return &flowResource{} }

type sharedWithModel struct {
	Permissions types.List   `tfsdk:"permissions"`
	Scope       types.String `tfsdk:"scope"`
	Email       types.String `tfsdk:"email"`
	Domain      types.String `tfsdk:"domain"`
}

type flowModel struct {
	ID           types.String  `tfsdk:"id"`
	Name         types.String  `tfsdk:"name"`
	FlowJSON     types.String  `tfsdk:"flow_json"`
	CustomFields types.Dynamic `tfsdk:"custom_fields"`
	SharedWith   types.List    `tfsdk:"shared_with"`
	Stage        types.String  `tfsdk:"stage"`
}

type flowCreateResponse struct {
	FlowID string `json:"flowId"`
}

func (r *flowResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_flow"
}

func (r *flowResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Appmixer flow. The flow descriptor is stored in `flow_json` and compared on every plan; " +
			"volatile server-owned fields (`stage`, timestamps, `userId`) are excluded from drift detection.\n\n" +
			"~> `stage` (`running`/`stopped`) is read-only — start and stop flows via the Appmixer UI or API, " +
			"not through this resource.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Human-readable name shown in the Appmixer UI.",
			},
			"flow_json": schema.StringAttribute{
				Required: true,
				Description: "Flow descriptor as a JSON string. Typical authoring path: design in the Appmixer UI, " +
					"export, store as a file, and reference with `file()`. " +
					"JSON key order is normalized on plan to prevent perpetual diffs.",
				PlanModifiers: []planmodifier.String{
					normalizeJSONModifier{},
				},
			},
			"custom_fields": schema.DynamicAttribute{
				Optional: true,
				Description: "Arbitrary metadata attached to the flow. Values may be strings, booleans, or numbers, " +
					"e.g. `{ category = \"customer-ops\", active = true, priority = 1 }`.",
			},
			"shared_with": schema.ListNestedAttribute{
				Optional:    true,
				Description: "List of sharing permissions for this flow.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"permissions": schema.ListAttribute{
							Required:    true,
							ElementType: types.StringType,
							Description: `Permissions to grant. Supported values: "read", "start", "stop".`,
						},
						"scope": schema.StringAttribute{
							Optional:    true,
							Description: `Share with a named scope, e.g. "template".`,
						},
						"email": schema.StringAttribute{
							Optional:    true,
							Description: "Share with a specific user by email address.",
						},
						"domain": schema.StringAttribute{
							Optional:    true,
							Description: "Share with all users in a domain, e.g. \"acme.com\".",
						},
					},
				},
			},
			"stage": schema.StringAttribute{
				Computed:      true,
				Description:   "Current execution stage: `running` or `stopped`. Managed by the server; read-only.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *flowResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("%T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *flowResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan flowModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var flowDoc map[string]any
	if err := json.Unmarshal([]byte(plan.FlowJSON.ValueString()), &flowDoc); err != nil {
		resp.Diagnostics.AddError("Invalid flow_json", err.Error())
		return
	}

	cf, err := customFieldsToAPI(plan.CustomFields)
	if err != nil {
		resp.Diagnostics.AddError("Invalid custom_fields", err.Error())
		return
	}

	sw, err := sharedWithToAPI(ctx, plan.SharedWith)
	if err != nil {
		resp.Diagnostics.AddError("Invalid shared_with", err.Error())
		return
	}

	body := map[string]any{
		"name":         plan.Name.ValueString(),
		"flow":         flowDoc,
		"customFields": cf,
	}
	if sw != nil {
		body["sharedWith"] = sw
	}

	created, err := client.Post[flowCreateResponse](ctx, r.client, "/flows", body)
	if err != nil {
		resp.Diagnostics.AddError("Create /flows failed", diagDetail(err))
		return
	}

	wire, err := client.Get[apitypes.FlowWire](ctx, r.client, "/flows/"+created.FlowID)
	if err != nil {
		resp.Diagnostics.AddError("Read /flows after create failed", diagDetail(err))
		return
	}

	m, err := wireToFlowModel(wire)
	if err != nil {
		resp.Diagnostics.AddError("Parse /flows response failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *flowResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state flowModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	wire, err := client.Get[apitypes.FlowWire](ctx, r.client, "/flows/"+state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read /flows failed", diagDetail(err))
		return
	}

	m, err := wireToFlowModel(wire)
	if err != nil {
		resp.Diagnostics.AddError("Parse /flows response failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *flowResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan flowModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state flowModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var flowDoc map[string]any
	if err := json.Unmarshal([]byte(plan.FlowJSON.ValueString()), &flowDoc); err != nil {
		resp.Diagnostics.AddError("Invalid flow_json", err.Error())
		return
	}

	cf, err := customFieldsToAPI(plan.CustomFields)
	if err != nil {
		resp.Diagnostics.AddError("Invalid custom_fields", err.Error())
		return
	}

	sw, err := sharedWithToAPI(ctx, plan.SharedWith)
	if err != nil {
		resp.Diagnostics.AddError("Invalid shared_with", err.Error())
		return
	}

	flowID := state.ID.ValueString()
	body := map[string]any{
		"flowId":       flowID,
		"name":         plan.Name.ValueString(),
		"flow":         flowDoc,
		"stage":        state.Stage.ValueString(),
		"customFields": cf,
	}
	if sw != nil {
		body["sharedWith"] = sw
	}

	if _, err := client.Put[map[string]any](ctx, r.client, "/flows/"+flowID, body); err != nil {
		resp.Diagnostics.AddError("Update /flows failed", diagDetail(err))
		return
	}

	wire, err := client.Get[apitypes.FlowWire](ctx, r.client, "/flows/"+flowID)
	if err != nil {
		resp.Diagnostics.AddError("Read /flows after update failed", diagDetail(err))
		return
	}

	m, err := wireToFlowModel(wire)
	if err != nil {
		resp.Diagnostics.AddError("Parse /flows response failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *flowResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state flowModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := client.Delete[map[string]any](ctx, r.client, "/flows/"+state.ID.ValueString()); err != nil {
		if client.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Delete /flows failed", diagDetail(err))
	}
}

func (r *flowResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// customFieldsToAPI converts the plan's CustomFields dynamic value into a
// map[string]any for the API. Returns nil when the attribute is null or unknown.
func customFieldsToAPI(d types.Dynamic) (map[string]any, error) {
	if d.IsNull() || d.IsUnknown() {
		return nil, nil
	}
	underlying := d.UnderlyingValue()
	obj, ok := underlying.(types.Object)
	if !ok {
		return nil, fmt.Errorf("custom_fields must be an object, got %T", underlying)
	}
	attrs := obj.Attributes()
	result := make(map[string]any, len(attrs))
	for k, v := range attrs {
		goVal, err := attrToGoValue(v)
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", k, err)
		}
		result[k] = goVal
	}
	return result, nil
}

func attrToGoValue(v attr.Value) (any, error) {
	switch tv := v.(type) {
	case types.String:
		return tv.ValueString(), nil
	case types.Bool:
		return tv.ValueBool(), nil
	case types.Number:
		n := tv.ValueBigFloat()
		if n.IsInt() {
			i, _ := n.Int64()
			return i, nil
		}
		f, _ := n.Float64()
		return f, nil
	default:
		return nil, fmt.Errorf("unsupported type %T", v)
	}
}

// sharedWithToAPI converts the plan's SharedWith list into a []map[string]any
// for the API. Returns nil when the attribute is null or unknown.
func sharedWithToAPI(ctx context.Context, l types.List) ([]map[string]any, error) {
	if l.IsNull() || l.IsUnknown() {
		return nil, nil
	}
	var items []sharedWithModel
	if diags := l.ElementsAs(ctx, &items, false); diags.HasError() {
		return nil, fmt.Errorf("reading shared_with: %s", diags[0].Detail())
	}
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		var perms []string
		if diags := item.Permissions.ElementsAs(ctx, &perms, false); diags.HasError() {
			return nil, fmt.Errorf("reading permissions: %s", diags[0].Detail())
		}
		m := map[string]any{"permissions": perms}
		if !item.Scope.IsNull() && !item.Scope.IsUnknown() {
			m["scope"] = item.Scope.ValueString()
		}
		if !item.Email.IsNull() && !item.Email.IsUnknown() {
			m["email"] = item.Email.ValueString()
		}
		if !item.Domain.IsNull() && !item.Domain.IsUnknown() {
			m["domain"] = item.Domain.ValueString()
		}
		result = append(result, m)
	}
	return result, nil
}

// wireToFlowModel converts a FlowWire (API response) into a flowModel.
func wireToFlowModel(w apitypes.FlowWire) (flowModel, error) {
	b, err := json.Marshal(w.Flow)
	if err != nil {
		return flowModel{}, fmt.Errorf("marshal flow: %w", err)
	}

	sharedWith, err := apitypes.BuildSharedWithList(w.SharedWith)
	if err != nil {
		return flowModel{}, fmt.Errorf("build shared_with: %w", err)
	}

	return flowModel{
		ID:           types.StringValue(w.FlowID),
		Name:         types.StringValue(w.Name),
		FlowJSON:     types.StringValue(string(b)),
		CustomFields: apitypes.BuildCustomFieldsDynamic(w.CustomFields),
		SharedWith:   sharedWith,
		Stage:        types.StringValue(w.Stage),
	}, nil
}
