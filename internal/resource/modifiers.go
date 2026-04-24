package resource

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ellosoft/terraform-provider-appmixer/internal/client"
)

type modifiersResource struct {
	client *client.Client
}

func NewModifiersResource() resource.Resource { return &modifiersResource{} }

type modifiersModel struct {
	ID       types.String `tfsdk:"id"`
	Document types.String `tfsdk:"document"`
}

// normalizeJSONModifier prevents perpetual diffs from JSON key order differences.
// Go's json.Marshal sorts map keys alphabetically, making round-trips stable.
type normalizeJSONModifier struct{}

func (normalizeJSONModifier) Description(_ context.Context) string {
	return "Normalizes JSON key order to prevent perpetual plan diffs."
}

func (normalizeJSONModifier) MarkdownDescription(_ context.Context) string {
	return "Normalizes JSON key order to prevent perpetual plan diffs."
}

func (normalizeJSONModifier) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.PlanValue.IsNull() || req.PlanValue.IsUnknown() {
		return
	}
	var v any
	if err := json.Unmarshal([]byte(req.PlanValue.ValueString()), &v); err != nil {
		resp.Diagnostics.AddError("Invalid document JSON", err.Error())
		return
	}
	b, _ := json.Marshal(v)
	resp.PlanValue = types.StringValue(string(b))
}

func (r *modifiersResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_modifiers"
}

func (r *modifiersResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the tenant's custom modifier functions as a single JSON document (singleton resource — one per tenant).\n\n" +
			"~> **Warning** Destroying this resource calls `DELETE /modifiers`, which removes **all** modifiers " +
			"including Appmixer's built-in defaults, not only the custom ones added here. " +
			"To restore defaults, reapply the resource or reprovision the tenant.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Always `default`. Used as the import ID.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"document": schema.StringAttribute{
				Required: true,
				Description: "Full modifiers configuration as a JSON object with `categories` and `modifiers` keys. " +
					"JSON key order is normalized on plan to prevent perpetual diffs.",
				PlanModifiers: []planmodifier.String{
					normalizeJSONModifier{},
				},
			},
		},
	}
}

func (r *modifiersResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *modifiersResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan modifiersModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(plan.Document.ValueString()), &doc); err != nil {
		resp.Diagnostics.AddError("Invalid document JSON", err.Error())
		return
	}
	if _, err := client.Put[map[string]any](ctx, r.client, "/modifiers", doc); err != nil {
		resp.Diagnostics.AddError("Create /modifiers failed", diagDetail(err))
		return
	}
	plan.ID = types.StringValue("default")
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *modifiersResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state modifiersModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	doc, err := client.Get[map[string]any](ctx, r.client, "/modifiers")
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read /modifiers failed", diagDetail(err))
		return
	}
	b, _ := json.Marshal(doc)
	state.Document = types.StringValue(string(b))
	state.ID = types.StringValue("default")
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *modifiersResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan modifiersModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(plan.Document.ValueString()), &doc); err != nil {
		resp.Diagnostics.AddError("Invalid document JSON", err.Error())
		return
	}
	if _, err := client.Put[map[string]any](ctx, r.client, "/modifiers", doc); err != nil {
		resp.Diagnostics.AddError("Update /modifiers failed", diagDetail(err))
		return
	}
	plan.ID = types.StringValue("default")
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *modifiersResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.AddWarning(
		"appmixer_modifiers destroy removes built-in defaults",
		"Destroying this resource calls DELETE /modifiers, which removes all modifier overrides. "+
			"Built-in Appmixer defaults will be restored on next startup.",
	)
	if _, err := client.Delete[map[string]any](ctx, r.client, "/modifiers"); err != nil {
		if client.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Delete /modifiers failed", diagDetail(err))
	}
}

func (r *modifiersResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
