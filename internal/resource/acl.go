package resource

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"

	"github.com/ellosoft/terraform-provider-appmixer/internal/client"
)

const (
	aclModeAuthoritative = "authoritative"
	aclModeMerge         = "merge"
)

type aclResource struct {
	client *client.Client
}

func NewACLResource() resource.Resource { return &aclResource{} }

type aclModel struct {
	ID    types.String   `tfsdk:"id"`
	Type  types.String   `tfsdk:"type"`
	Mode  types.String   `tfsdk:"mode"`
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
		MarkdownDescription: "Manages ACL rules for a given `type`. Ownership of the server-side rule list is " +
			"controlled by `mode`:\n\n" +
			"- `authoritative` (default): this resource owns the **entire** list. Apply replaces whatever is " +
			"server-side with `rules`, and destroy resets the list to empty (not the tenant defaults).\n" +
			"- `merge`: this resource owns **only the rules it declares**. Rules configured out-of-band are " +
			"preserved across apply and destroy. Rules removed from `rules` between applies are deleted from the " +
			"server; externally-added rules are left in place.",
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
			"mode": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Ownership mode: `authoritative` (replace entire list, default) or `merge` " +
					"(only manage declared rules, preserve externals).",
				Default:    stringdefault.StaticString(aclModeAuthoritative),
				Validators: []validator.String{stringvalidator.OneOf(aclModeAuthoritative, aclModeMerge)},
			},
			"rules": schema.SetNestedAttribute{
				Required:    true,
				Description: "Set of access-control rules this resource declares.",
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

	planWire, diags := rulesToWire(ctx, plan.Rules)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var payload []aclRuleWire
	if effectiveMode(plan.Mode) == aclModeMerge {
		external, err := fetchExternal(ctx, r.client, plan.Type.ValueString(), planWire)
		if err != nil {
			resp.Diagnostics.AddError("Read /acl before merge failed", diagDetail(err))
			return
		}
		payload = append(external, planWire...)
	} else {
		payload = planWire
	}

	if _, err := client.Post[[]aclRuleWire](ctx, r.client, "/acl/"+plan.Type.ValueString(), payload); err != nil {
		resp.Diagnostics.AddError("POST /acl failed", diagDetail(err))
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

	serverWire, err := client.Get[[]aclRuleWire](ctx, r.client, "/acl/"+state.Type.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read /acl failed", diagDetail(err))
		return
	}

	// In merge mode, only rules that remain in both state-wire and server are
	// reported back. Rules we managed but that were externally deleted fall
	// out of state (drift → next plan re-adds them if config still wants them).
	var visible []aclRuleWire
	if effectiveMode(state.Mode) == aclModeMerge {
		stateWire, diags := rulesToWire(ctx, state.Rules)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		serverSet := make(map[string]struct{}, len(serverWire))
		for _, w := range serverWire {
			serverSet[ruleKey(w)] = struct{}{}
		}
		for _, w := range stateWire {
			if _, ok := serverSet[ruleKey(w)]; ok {
				visible = append(visible, w)
			}
		}
	} else {
		visible = serverWire
	}

	rules, diags := wireToRules(ctx, visible)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.Rules = rules
	state.ID = state.Type
	if state.Mode.IsNull() || state.Mode.IsUnknown() {
		// Migrate legacy state that predates the mode attribute.
		state.Mode = types.StringValue(aclModeAuthoritative)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *aclResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state aclModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	planWire, diags := rulesToWire(ctx, plan.Rules)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var payload []aclRuleWire
	if effectiveMode(plan.Mode) == aclModeMerge {
		// Compute external = server - (rules we previously managed).
		// This preserves unmanaged rules and removes entries we owned but
		// dropped from config.
		stateWire, diags := rulesToWire(ctx, state.Rules)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		external, err := fetchExternal(ctx, r.client, plan.Type.ValueString(), stateWire)
		if err != nil {
			resp.Diagnostics.AddError("Read /acl before merge failed", diagDetail(err))
			return
		}
		payload = append(external, planWire...)
	} else {
		payload = planWire
	}

	if _, err := client.Post[[]aclRuleWire](ctx, r.client, "/acl/"+plan.Type.ValueString(), payload); err != nil {
		resp.Diagnostics.AddError("POST /acl failed", diagDetail(err))
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

	if effectiveMode(state.Mode) == aclModeMerge {
		stateWire, diags := rulesToWire(ctx, state.Rules)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		external, err := fetchExternal(ctx, r.client, state.Type.ValueString(), stateWire)
		if err != nil {
			if client.IsNotFound(err) {
				return
			}
			resp.Diagnostics.AddError("Read /acl before merge delete failed", diagDetail(err))
			return
		}
		if _, err := client.Post[[]aclRuleWire](ctx, r.client, "/acl/"+state.Type.ValueString(), external); err != nil {
			resp.Diagnostics.AddError("POST /acl failed", diagDetail(err))
		}
		return
	}

	// Authoritative: reset the list to empty.
	if _, err := client.Post[[]aclRuleWire](ctx, r.client, "/acl/"+state.Type.ValueString(), []aclRuleWire{}); err != nil {
		resp.Diagnostics.AddError("POST /acl failed", diagDetail(err))
	}
}

func (r *aclResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("type"), req, resp)
}

// effectiveMode returns the mode string, defaulting to authoritative when the
// attribute is null or unknown (legacy state or first plan).
func effectiveMode(m types.String) string {
	if m.IsNull() || m.IsUnknown() {
		return aclModeAuthoritative
	}
	return m.ValueString()
}

// rulesToWire flattens the TF-level model into wire structs. It's used both
// when pushing rules to the server and when computing set membership.
func rulesToWire(ctx context.Context, rules []aclRuleModel) ([]aclRuleWire, diag.Diagnostics) {
	var diags diag.Diagnostics
	out := make([]aclRuleWire, 0, len(rules))
	for _, rule := range rules {
		var actions []string
		diags.Append(rule.Action.ElementsAs(ctx, &actions, false)...)
		var attrs []string
		diags.Append(rule.Attributes.ElementsAs(ctx, &attrs, false)...)
		out = append(out, aclRuleWire{
			Role:       rule.Role.ValueString(),
			Resource:   rule.Resource.ValueString(),
			Action:     actions,
			Attributes: attrs,
		})
	}
	return out, diags
}

func wireToRules(ctx context.Context, wire []aclRuleWire) ([]aclRuleModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	out := make([]aclRuleModel, len(wire))
	for i, w := range wire {
		actions, d := types.SetValueFrom(ctx, types.StringType, w.Action)
		diags.Append(d...)
		attrs, d := types.SetValueFrom(ctx, types.StringType, w.Attributes)
		diags.Append(d...)
		out[i] = aclRuleModel{
			Role:       types.StringValue(w.Role),
			Resource:   types.StringValue(w.Resource),
			Action:     actions,
			Attributes: attrs,
		}
	}
	return out, diags
}

// ruleKey returns a deterministic identity string for a wire rule. Identity
// is full content (role + resource + sorted action set + sorted attribute
// set); any field change produces a different key, so external edits to a
// managed rule appear as "rule gone" on refresh.
func ruleKey(w aclRuleWire) string {
	actions := append([]string(nil), w.Action...)
	attrs := append([]string(nil), w.Attributes...)
	sort.Strings(actions)
	sort.Strings(attrs)
	return w.Role + "|" + w.Resource + "|" + strings.Join(actions, ",") + "|" + strings.Join(attrs, ",")
}

// fetchExternal returns all server-side rules that are NOT in `ours`. Used
// by merge mode to preserve out-of-band configuration.
func fetchExternal(ctx context.Context, c *client.Client, aclType string, ours []aclRuleWire) ([]aclRuleWire, error) {
	server, err := client.Get[[]aclRuleWire](ctx, c, "/acl/"+aclType)
	if err != nil {
		return nil, err
	}
	ownSet := make(map[string]struct{}, len(ours))
	for _, w := range ours {
		ownSet[ruleKey(w)] = struct{}{}
	}
	var external []aclRuleWire
	for _, w := range server {
		if _, ok := ownSet[ruleKey(w)]; !ok {
			external = append(external, w)
		}
	}
	return external, nil
}
