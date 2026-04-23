package resource

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"

	"github.com/ellosoft/terraform-provider-appmixer/internal/client"
)

type aclResource struct {
	client *client.Client
}

func NewACLResource() resource.Resource { return &aclResource{} }

type aclModel struct {
	ID    types.String   `tfsdk:"id"`
	Type  types.String   `tfsdk:"type"`
	Rules []aclRuleModel `tfsdk:"rules"`
}

type aclRuleModel struct {
	Role       types.String `tfsdk:"role"`
	Resource   types.String `tfsdk:"resource"`
	Action     types.Set    `tfsdk:"action"`
	Attributes types.Set    `tfsdk:"attributes"`
}

type aclRuleWire struct {
	Role       string   `json:"role"`
	Resource   string   `json:"resource"`
	Action     []string `json:"action"`
	Attributes []string `json:"attributes"`
}

func (r *aclResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_acl"
}

func (r *aclResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the complete ACL rule list for a given `type`. " +
			"This resource owns the **entire** list — creating it replaces whatever exists server-side, " +
			"and destroying it resets the list to empty (not the tenant defaults).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"type": schema.StringAttribute{
				Required:      true,
				Description:   "ACL list to manage. Valid values: `components`, `routes`. Changes force replacement.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    []validator.String{stringvalidator.OneOf("components", "routes")},
			},
			"rules": schema.SetNestedAttribute{
				Required:    true,
				Description: "Ordered set of access-control rules. The full list is pushed on every apply.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"role": schema.StringAttribute{
							Required:    true,
							Description: "Role identifier this rule applies to, e.g. `admin`, `viewer`.",
						},
						"resource": schema.StringAttribute{
							Required:    true,
							Description: "Resource pattern this rule matches. Use `*` to match all resources.",
						},
						"action": schema.SetAttribute{
							Required:    true,
							ElementType: types.StringType,
							Description: "Set of allowed actions, e.g. `[\"read\", \"write\"]`. Use `[\"*\"]` to allow all actions.",
						},
						"attributes": schema.SetAttribute{
							Required:    true,
							ElementType: types.StringType,
							Description: "Attribute filter. Use `[\"*\"]` to allow all attributes or `[\"non-private\"]` to restrict to public ones.",
						},
					},
				},
			},
		},
	}
}

func (r *aclResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *aclResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan aclModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.pushRules(ctx, plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = plan.Type
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *aclResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state aclModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	wire, err := client.Get[[]aclRuleWire](ctx, r.client, "/acl/"+state.Type.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read /acl failed", diagDetail(err))
		return
	}
	rules := make([]aclRuleModel, len(wire))
	for i, w := range wire {
		actions, d := types.SetValueFrom(ctx, types.StringType, w.Action)
		resp.Diagnostics.Append(d...)
		attrs, d := types.SetValueFrom(ctx, types.StringType, w.Attributes)
		resp.Diagnostics.Append(d...)
		rules[i] = aclRuleModel{
			Role:       types.StringValue(w.Role),
			Resource:   types.StringValue(w.Resource),
			Action:     actions,
			Attributes: attrs,
		}
	}
	if resp.Diagnostics.HasError() {
		return
	}
	state.Rules = rules
	state.ID = state.Type
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *aclResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan aclModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.pushRules(ctx, plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = plan.Type
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *aclResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state aclModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	empty := aclModel{ID: state.ID, Type: state.Type}
	resp.Diagnostics.Append(r.pushRules(ctx, empty)...)
}

func (r *aclResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("type"), req, resp)
}

func (r *aclResource) pushRules(ctx context.Context, m aclModel) diag.Diagnostics {
	var diags diag.Diagnostics
	wire := make([]aclRuleWire, len(m.Rules))
	for i, rule := range m.Rules {
		var actions []string
		diags.Append(rule.Action.ElementsAs(ctx, &actions, false)...)
		var attrs []string
		diags.Append(rule.Attributes.ElementsAs(ctx, &attrs, false)...)
		wire[i] = aclRuleWire{
			Role:       rule.Role.ValueString(),
			Resource:   rule.Resource.ValueString(),
			Action:     actions,
			Attributes: attrs,
		}
	}
	if diags.HasError() {
		return diags
	}
	if _, err := client.Post[[]aclRuleWire](ctx, r.client, "/acl/"+m.Type.ValueString(), wire); err != nil {
		diags.AddError("POST /acl failed", diagDetail(err))
	}
	return diags
}
