package resource

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ellosoft/terraform-provider-appmixer/internal/client"
)

type quotaResource struct {
	client *client.Client
}

func NewQuotaResource() resource.Resource { return &quotaResource{} }

type quotaModel struct {
	ID            types.String `tfsdk:"id"`
	ServiceID     types.String `tfsdk:"service_id"`
	Source        types.String `tfsdk:"source"`
	DefaultSource types.String `tfsdk:"default_source"`
	IsCustom      types.Bool   `tfsdk:"is_custom"`
}

type quotaWire struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	DefaultSource string `json:"defaultSource"`
	IsCustom      *bool  `json:"isCustom"`
	Source        string `json:"source"`
}

func (r *quotaResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_quota"
}

func (r *quotaResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a custom Appmixer quota rule. Each quota overrides the built-in default " +
			"for a namespaced service or module (e.g. `appmixer:hubspot`). Destroying the resource reverts " +
			"the service to its default quota.\n\n" +
			"The `source` field is a Node.js module source string exporting a `rules` array. Typical authoring " +
			"path: keep the source in a `.js` file and reference it with `file(\"...\")`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Server-assigned identifier for the quota record.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"service_id": schema.StringAttribute{
				Required: true,
				Description: "Namespaced quota key, e.g. `appmixer:hubspot` or `tenant:custom-rule`. " +
					"Changes force replacement.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"source": schema.StringAttribute{
				Required: true,
				Description: "Node.js module source code defining the quota rules. " +
					"Must be a `'use strict';` module exporting `{ rules: [...] }`.",
			},
			"default_source": schema.StringAttribute{
				Computed:      true,
				Description:   "The built-in default quota source for this name (empty string if none).",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"is_custom": schema.BoolAttribute{
				Computed:      true,
				Description:   "Whether the quota has been customized. Always true once this resource is applied.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *quotaResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *quotaResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan quotaModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	wire, err := client.Put[quotaWire](ctx, r.client, "/quota/"+plan.ServiceID.ValueString(), map[string]any{
		"source": plan.Source.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Create /quota failed", diagDetail(err))
		return
	}

	applyQuotaWire(&plan, wire)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *quotaResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state quotaModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The Appmixer API has no per-name GET for quotas; list all and filter client-side.
	list, err := client.Get[[]quotaWire](ctx, r.client, "/quota")
	if err != nil {
		resp.Diagnostics.AddError("Read /quota failed", diagDetail(err))
		return
	}

	serviceID := state.ServiceID.ValueString()
	for _, w := range list {
		if w.Name == serviceID {
			applyQuotaWire(&state, w)
			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
			return
		}
	}
	// Not in the list — quota was removed server-side or reverted to default.
	resp.State.RemoveResource(ctx)
}

func (r *quotaResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan quotaModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	wire, err := client.Put[quotaWire](ctx, r.client, "/quota/"+plan.ServiceID.ValueString(), map[string]any{
		"source": plan.Source.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Update /quota failed", diagDetail(err))
		return
	}

	applyQuotaWire(&plan, wire)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *quotaResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state quotaModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := client.Delete[map[string]any](ctx, r.client, "/quota/"+state.ServiceID.ValueString()); err != nil {
		if client.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Delete /quota failed", diagDetail(err))
	}
}

func (r *quotaResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("service_id"), req, resp)
}

func applyQuotaWire(m *quotaModel, w quotaWire) {
	m.ID = types.StringValue(w.ID)
	m.ServiceID = types.StringValue(w.Name)
	m.Source = types.StringValue(w.Source)
	m.DefaultSource = types.StringValue(w.DefaultSource)
	if w.IsCustom == nil {
		m.IsCustom = types.BoolNull()
	} else {
		m.IsCustom = types.BoolValue(*w.IsCustom)
	}
}
