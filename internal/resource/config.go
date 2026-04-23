package resource

import (
	"context"
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

type configResource struct {
	client *client.Client
}

func NewConfigResource() resource.Resource { return &configResource{} }

type configModel struct {
	ID    types.String `tfsdk:"id"`
	Key   types.String `tfsdk:"key"`
	Value types.String `tfsdk:"value"`
}

func (r *configResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_config"
}

func (r *configResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"key": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"value": schema.StringAttribute{
				Required:      true,
				Sensitive:     true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
		},
	}
}

func (r *configResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

type configEntry struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

func (r *configResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan configModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := configEntry{Key: plan.Key.ValueString(), Value: plan.Value.ValueString()}
	if _, err := client.Post[configEntry](ctx, r.client, "/config", body); err != nil {
		resp.Diagnostics.AddError("Create /config failed", diagDetail(err))
		return
	}
	plan.ID = plan.Key
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *configResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state configModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	all, err := client.Get[[]configEntry](ctx, r.client, "/config")
	if err != nil {
		resp.Diagnostics.AddError("Read /config failed", diagDetail(err))
		return
	}
	key := state.Key.ValueString()
	for _, e := range all {
		if e.Key == key {
			state.Value = types.StringValue(fmt.Sprintf("%v", e.Value))
			state.ID = state.Key
			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
			return
		}
	}
	resp.State.RemoveResource(ctx) // drift: removed server-side
}

func (r *configResource) Update(ctx context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("unexpected update", "appmixer_config has no PUT; changes to key or value force replacement")
}

func (r *configResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state configModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := client.Delete[map[string]any](ctx, r.client, "/config/"+state.Key.ValueString()); err != nil {
		resp.Diagnostics.AddError("Delete /config failed", diagDetail(err))
	}
}

func (r *configResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("key"), req, resp)
}

func diagDetail(err error) string {
	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		return apiErr.SafeMessage()
	}
	return err.Error()
}
