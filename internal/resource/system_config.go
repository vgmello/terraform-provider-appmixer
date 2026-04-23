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

type systemConfigResource struct {
	client *client.Client
}

func NewSystemConfigResource() resource.Resource { return &systemConfigResource{} }

type systemConfigModel struct {
	ID    types.String `tfsdk:"id"`
	Key   types.String `tfsdk:"key"`
	Value types.String `tfsdk:"value"`
}

func (r *systemConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_system_config"
}

func (r *systemConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a single tenant configuration entry. Only string values are supported.\n\n" +
			"~> Changes to `key` or `value` force replacement — the Appmixer API has no update path for config entries.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"key": schema.StringAttribute{
				Required:      true,
				Description:   "Configuration key name, e.g. `JWTSecret`. Changes force replacement.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"value": schema.StringAttribute{
				Required:      true,
				Sensitive:     true,
				Description:   "Configuration value. Sensitive. Changes force replacement.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
		},
	}
}

func (r *systemConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

type systemConfigEntry struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

func (r *systemConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan systemConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	valBytes, _ := json.Marshal(plan.Value.ValueString())
	body := systemConfigEntry{Key: plan.Key.ValueString(), Value: valBytes}
	if _, err := client.Post[systemConfigEntry](ctx, r.client, "/config", body); err != nil {
		resp.Diagnostics.AddError("Create /config failed", diagDetail(err))
		return
	}
	plan.ID = plan.Key
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *systemConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state systemConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	all, err := client.Get[[]systemConfigEntry](ctx, r.client, "/config")
	if err != nil {
		resp.Diagnostics.AddError("Read /config failed", diagDetail(err))
		return
	}
	key := state.Key.ValueString()
	for _, e := range all {
		if e.Key == key {
			var strVal string
			if err := json.Unmarshal(e.Value, &strVal); err != nil {
				var rawVal interface{}
				_ = json.Unmarshal(e.Value, &rawVal)
				resp.Diagnostics.AddError(
					"Unsupported config value type",
					fmt.Sprintf(
						"Server returned a non-string JSON value (type %T) for key %q. "+
							"appmixer_system_config only manages string-valued entries; "+
							"use the Appmixer API directly for other types.",
						rawVal, key,
					),
				)
				return
			}
			state.Value = types.StringValue(strVal)
			state.ID = state.Key
			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
			return
		}
	}
	resp.State.RemoveResource(ctx) // drift: removed server-side
}

func (r *systemConfigResource) Update(ctx context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("unexpected update", "appmixer_system_config has no PUT; changes to key or value force replacement")
}

func (r *systemConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state systemConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := client.Delete[map[string]any](ctx, r.client, "/config/"+state.Key.ValueString()); err != nil {
		resp.Diagnostics.AddError("Delete /config failed", diagDetail(err))
	}
}

func (r *systemConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("key"), req, resp)
}

func diagDetail(err error) string {
	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		return apiErr.SafeMessage()
	}
	return err.Error()
}
