package resource

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ellosoft/terraform-provider-appmixer/internal/client"
)

type flowResource struct {
	client *client.Client
}

func NewFlowResource() resource.Resource { return &flowResource{} }

type flowModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	FlowJSON     types.String `tfsdk:"flow_json"`
	CustomFields types.Map    `tfsdk:"custom_fields"`
	Stage        types.String `tfsdk:"stage"`
}

type flowCreateResponse struct {
	FlowID string `json:"flowId"`
}

type flowWire struct {
	FlowID       string         `json:"flowId"`
	Name         string         `json:"name"`
	Flow         map[string]any `json:"flow"`
	CustomFields map[string]any `json:"customFields"`
	Stage        string         `json:"stage"`
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
			"custom_fields": schema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Arbitrary string metadata attached to the flow, e.g. `{ category = \"customer-ops\" }`.",
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

	cf, err := customFieldsToAPI(ctx, plan.CustomFields)
	if err != nil {
		resp.Diagnostics.AddError("Invalid custom_fields", err.Error())
		return
	}

	body := map[string]any{
		"name":         plan.Name.ValueString(),
		"flow":         flowDoc,
		"customFields": cf,
	}

	created, err := client.Post[flowCreateResponse](ctx, r.client, "/flows", body)
	if err != nil {
		resp.Diagnostics.AddError("Create /flows failed", diagDetail(err))
		return
	}

	wire, err := client.Get[flowWire](ctx, r.client, "/flows/"+created.FlowID)
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

	wire, err := client.Get[flowWire](ctx, r.client, "/flows/"+state.ID.ValueString())
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
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

	cf, err := customFieldsToAPI(ctx, plan.CustomFields)
	if err != nil {
		resp.Diagnostics.AddError("Invalid custom_fields", err.Error())
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

	if _, err := client.Put[map[string]any](ctx, r.client, "/flows/"+flowID, body); err != nil {
		resp.Diagnostics.AddError("Update /flows failed", diagDetail(err))
		return
	}

	wire, err := client.Get[flowWire](ctx, r.client, "/flows/"+flowID)
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

// customFieldsToAPI converts the plan's CustomFields map into a map[string]any
// for the API. Returns nil when the attribute is null or unknown.
func customFieldsToAPI(ctx context.Context, m types.Map) (map[string]any, error) {
	if m.IsNull() || m.IsUnknown() {
		return nil, nil
	}
	var cf map[string]string
	if diags := m.ElementsAs(ctx, &cf, false); diags.HasError() {
		return nil, fmt.Errorf("reading custom_fields: %s", diags[0].Detail())
	}
	result := make(map[string]any, len(cf))
	for k, v := range cf {
		result[k] = v
	}
	return result, nil
}

// wireToFlowModel converts a flowWire (API response) into a flowModel.
func wireToFlowModel(w flowWire) (flowModel, error) {
	b, err := json.Marshal(w.Flow)
	if err != nil {
		return flowModel{}, fmt.Errorf("marshal flow: %w", err)
	}

	// Only populate custom_fields when the server actually returned values.
	// Returning an empty map when the plan had null causes an inconsistent-result error.
	var customFields types.Map
	if len(w.CustomFields) > 0 {
		cfVals := make(map[string]attr.Value, len(w.CustomFields))
		for k, v := range w.CustomFields {
			cfVals[k] = types.StringValue(fmt.Sprintf("%v", v))
		}
		customFields = types.MapValueMust(types.StringType, cfVals)
	} else {
		customFields = types.MapNull(types.StringType)
	}

	return flowModel{
		ID:           types.StringValue(w.FlowID),
		Name:         types.StringValue(w.Name),
		FlowJSON:     types.StringValue(string(b)),
		CustomFields: customFields,
		Stage:        types.StringValue(w.Stage),
	}, nil
}
