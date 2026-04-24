package resource

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ellosoft/terraform-provider-appmixer/internal/client"
)

// diagDetail is defined in system_config.go (same package).

type serviceConfigResource struct {
	client *client.Client
}

func NewServiceConfigResource() resource.Resource { return &serviceConfigResource{} }

type serviceConfigModel struct {
	ID             types.String `tfsdk:"id"`
	ServiceID      types.String `tfsdk:"service_id"`
	Items          types.Map    `tfsdk:"items"`
	SensitiveItems types.Map    `tfsdk:"sensitive_items"`
}

func (r *serviceConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_config"
}

func (r *serviceConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages configuration for a third-party service integration (e.g. OAuth credentials for Google or Slack). " +
			"Non-sensitive and sensitive items are stored in separate attributes so only secrets are redacted in plan output.\n\n" +
			"~> After `terraform import`, all keys land in `sensitive_items` (the safe default — secrets stay redacted in plan output). " +
			"Move non-secrets into `items` before the next `terraform apply`; the first plan will show the partition as drift, which the apply reconciles.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"service_id": schema.StringAttribute{
				Required:      true,
				Description:   `Service identifier in "vendor:service" form, e.g. "appmixer:google". Changes force replacement.`,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"items": schema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Non-sensitive configuration items for the service. Visible in plan output.",
			},
			"sensitive_items": schema.MapAttribute{
				Optional:    true,
				Sensitive:   true,
				ElementType: types.StringType,
				Description: "Sensitive configuration items (e.g. client secrets, API keys). Redacted in plan output. Keys must not overlap with `items`.",
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

// mergedPayload combines service_id + items + sensitive_items into the
// single object the Appmixer API expects.
func mergedPayload(ctx context.Context, plan serviceConfigModel) (map[string]any, error) {
	out := map[string]any{"serviceId": plan.ServiceID.ValueString()}

	var items map[string]string
	if !plan.Items.IsNull() && !plan.Items.IsUnknown() {
		if diags := plan.Items.ElementsAs(ctx, &items, false); diags.HasError() {
			return nil, fmt.Errorf("read items: %v", diags)
		}
	}
	var sensitive map[string]string
	if !plan.SensitiveItems.IsNull() && !plan.SensitiveItems.IsUnknown() {
		if diags := plan.SensitiveItems.ElementsAs(ctx, &sensitive, false); diags.HasError() {
			return nil, fmt.Errorf("read sensitive_items: %v", diags)
		}
	}

	// Key-collision detection backstop. The plan-time noDuplicateKeysValidator catches this
	// earlier; this guard handles the edge case where the validator is bypassed.
	for k := range sensitive {
		if _, dup := items[k]; dup {
			return nil, fmt.Errorf("key %q appears in both items and sensitive_items", k)
		}
	}

	for k, v := range items {
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
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read /service-config failed", diagDetail(err))
		return
	}

	// Re-bucket server keys using prior state as the authoritative partition:
	//   - prior sensitive_items key -> sensitive_items
	//   - prior items key           -> items
	//   - unknown key (import, or newly added server-side) -> sensitive_items
	// Defaulting unknown keys to sensitive_items keeps secrets redacted on
	// first import; operators then promote non-secrets into `items` in HCL.
	priorSensitive := map[string]bool{}
	if !state.SensitiveItems.IsNull() {
		var m map[string]string
		resp.Diagnostics.Append(state.SensitiveItems.ElementsAs(ctx, &m, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		for k := range m {
			priorSensitive[k] = true
		}
	}
	priorItems := map[string]bool{}
	if !state.Items.IsNull() {
		var m map[string]string
		resp.Diagnostics.Append(state.Items.ElementsAs(ctx, &m, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		for k := range m {
			priorItems[k] = true
		}
	}

	items := map[string]string{}
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
		switch {
		case priorSensitive[k]:
			sensitive[k] = strVal
		case priorItems[k]:
			items[k] = strVal
		default:
			// Unknown key (import, or new server-side addition): default to
			// sensitive_items so secrets stay redacted until the operator
			// explicitly moves non-secrets into `items`.
			sensitive[k] = strVal
		}
	}

	// Empty maps must be written as null — the attributes are Optional (not
	// Computed), so config-absent (null) vs server-empty ({}) would cause a
	// perpetual diff on every apply.
	itemsMap := types.MapNull(types.StringType)
	if len(items) > 0 {
		m, diags := types.MapValueFrom(ctx, types.StringType, items)
		resp.Diagnostics.Append(diags...)
		itemsMap = m
	}
	sensitiveMap := types.MapNull(types.StringType)
	if len(sensitive) > 0 {
		m, diags := types.MapValueFrom(ctx, types.StringType, sensitive)
		resp.Diagnostics.Append(diags...)
		sensitiveMap = m
	}
	if resp.Diagnostics.HasError() {
		return
	}

	state.Items = itemsMap
	state.SensitiveItems = sensitiveMap
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
		if client.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Delete /service-config failed", diagDetail(err))
	}
}

func (r *serviceConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("service_id"), req, resp)
}

// ConfigValidators plugs plan-time validation into the framework.
func (r *serviceConfigResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		noDuplicateKeysValidator{},
	}
}

// noDuplicateKeysValidator rejects configurations where the same map key
// appears in both `items` and `sensitive_items`. Emits an attribute-scoped
// diagnostic so the error lands on the offending field in plan output.
type noDuplicateKeysValidator struct{}

func (v noDuplicateKeysValidator) Description(_ context.Context) string {
	return "ensures no key appears in both items and sensitive_items"
}

func (v noDuplicateKeysValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v noDuplicateKeysValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var cfg serviceConfigModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Unknown or null maps can't conflict; skip until values are concrete.
	if cfg.Items.IsNull() || cfg.Items.IsUnknown() {
		return
	}
	if cfg.SensitiveItems.IsNull() || cfg.SensitiveItems.IsUnknown() {
		return
	}

	var items map[string]string
	var sensitive map[string]string
	resp.Diagnostics.Append(cfg.Items.ElementsAs(ctx, &items, false)...)
	resp.Diagnostics.Append(cfg.SensitiveItems.ElementsAs(ctx, &sensitive, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	for k := range sensitive {
		if _, dup := items[k]; dup {
			resp.Diagnostics.AddAttributeError(
				path.Root("sensitive_items").AtMapKey(k),
				"Duplicate key",
				fmt.Sprintf("Key %q appears in both items and sensitive_items; move it to only one.", k),
			)
		}
	}
}
