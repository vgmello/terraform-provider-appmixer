package resource

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ellosoft/terraform-provider-appmixer/internal/client"
)

type serviceConfigResource struct {
	client *client.Client
}

func NewServiceConfigResource() resource.Resource { return &serviceConfigResource{} }

type serviceConfigModel struct {
	ID              types.String `tfsdk:"id"`
	ServiceID       types.String `tfsdk:"service_id"`
	Fields          types.Map    `tfsdk:"fields"`
	SensitiveFields types.Map    `tfsdk:"sensitive_fields"`
}

func (r *serviceConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_config"
}

func (r *serviceConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"service_id": schema.StringAttribute{
				Required:      true,
				Description:   `Service identifier in "vendor:service" form, e.g. "appmixer:google".`,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"fields": schema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Non-sensitive configuration fields. Visible in plan output.",
			},
			"sensitive_fields": schema.MapAttribute{
				Optional:    true,
				Sensitive:   true,
				ElementType: types.StringType,
				Description: "Sensitive configuration fields. Redacted in plan output.",
			},
		},
	}
}

func (r *serviceConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// mergedPayload combines service_id + fields + sensitive_fields into the
// single object the Appmixer API expects.
func mergedPayload(ctx context.Context, plan serviceConfigModel) (map[string]any, error) {
	out := map[string]any{"serviceId": plan.ServiceID.ValueString()}

	var fields map[string]string
	if !plan.Fields.IsNull() && !plan.Fields.IsUnknown() {
		if diags := plan.Fields.ElementsAs(ctx, &fields, false); diags.HasError() {
			return nil, fmt.Errorf("read fields: %v", diags)
		}
	}
	var sensitive map[string]string
	if !plan.SensitiveFields.IsNull() && !plan.SensitiveFields.IsUnknown() {
		if diags := plan.SensitiveFields.ElementsAs(ctx, &sensitive, false); diags.HasError() {
			return nil, fmt.Errorf("read sensitive_fields: %v", diags)
		}
	}

	// Key-collision detection (Task 7 will add a plan-time validator; this is a runtime backstop).
	for k := range sensitive {
		if _, dup := fields[k]; dup {
			return nil, fmt.Errorf("key %q appears in both fields and sensitive_fields", k)
		}
	}

	for k, v := range fields {
		out[k] = v
	}
	for k, v := range sensitive {
		out[k] = v
	}
	return out, nil
}

func (r *serviceConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serviceConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body, err := mergedPayload(ctx, plan)
	if err != nil {
		resp.Diagnostics.AddError("Invalid service_config payload", err.Error())
		return
	}

	if _, err := client.Post[map[string]any](ctx, r.client, "/service-config", body); err != nil {
		resp.Diagnostics.AddError("Create /service-config failed", diagDetail(err))
		return
	}

	plan.ID = plan.ServiceID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *serviceConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state serviceConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	serviceID := state.ServiceID.ValueString()
	got, err := client.Get[map[string]json.RawMessage](ctx, r.client, "/service-config/"+serviceID)
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read /service-config failed", diagDetail(err))
		return
	}

	// Determine which keys were previously in sensitive_fields so we can
	// keep them there after re-hydration. New keys default to fields.
	priorSensitive := map[string]bool{}
	if !state.SensitiveFields.IsNull() {
		var m map[string]string
		_ = state.SensitiveFields.ElementsAs(ctx, &m, false)
		for k := range m {
			priorSensitive[k] = true
		}
	}

	fields := map[string]string{}
	sensitive := map[string]string{}
	for k, raw := range got {
		if k == "serviceId" {
			continue
		}
		var strVal string
		if err := json.Unmarshal(raw, &strVal); err != nil {
			// Non-string server value — convert to JSON text so operator sees raw form.
			strVal = string(raw)
		}
		if priorSensitive[k] {
			sensitive[k] = strVal
		} else {
			fields[k] = strVal
		}
	}

	fieldsMap, diags := types.MapValueFrom(ctx, types.StringType, fields)
	resp.Diagnostics.Append(diags...)
	sensitiveMap, diags := types.MapValueFrom(ctx, types.StringType, sensitive)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.Fields = fieldsMap
	state.SensitiveFields = sensitiveMap
	state.ID = state.ServiceID

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *serviceConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan serviceConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body, err := mergedPayload(ctx, plan)
	if err != nil {
		resp.Diagnostics.AddError("Invalid service_config payload", err.Error())
		return
	}

	serviceID := plan.ServiceID.ValueString()
	if _, err := client.Put[map[string]any](ctx, r.client, "/service-config/"+serviceID, body); err != nil {
		resp.Diagnostics.AddError("Update /service-config failed", diagDetail(err))
		return
	}

	plan.ID = plan.ServiceID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *serviceConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state serviceConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	serviceID := state.ServiceID.ValueString()
	if _, err := client.Delete[map[string]any](ctx, r.client, "/service-config/"+serviceID); err != nil {
		resp.Diagnostics.AddError("Delete /service-config failed", diagDetail(err))
	}
}

func (r *serviceConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("service_id"), req, resp)
}
