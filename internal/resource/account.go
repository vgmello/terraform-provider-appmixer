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

type accountResource struct {
	client *client.Client
}

func NewAccountResource() resource.Resource { return &accountResource{} }

type accountModel struct {
	ID          types.String `tfsdk:"id"`
	Service     types.String `tfsdk:"service"`
	Name        types.String `tfsdk:"name"`
	DisplayName types.String `tfsdk:"display_name"`
	Token       types.String `tfsdk:"token"`
	ProfileInfo types.String `tfsdk:"profile_info"`
}

type accountWire struct {
	AccountID   string `json:"accountId"`
	Service     string `json:"service"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
}

func (r *accountResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_account"
}

func (r *accountResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a pre-obtained service account credential (API key, OAuth refresh token, or similar). " +
			"Intended for non-interactive, machine-managed accounts only — end-user OAuth flows remain UI-driven.\n\n" +
			"~> The account is identified by the composite key `(service, name)`. Changing either forces replacement. " +
			"`display_name`, `token`, and `profile_info` can be updated in place.\n\n" +
			"~> `token` is never returned by the Appmixer API. Terraform persists the last-written value in state. " +
			"Changing the token value in HCL rotates it in place; the server-assigned `id` is preserved.\n\n" +
			"~> After `terraform import`, `token` will be empty in state. Supply the desired token in HCL before the next " +
			"`terraform apply` — it will be written in place to rotate the credential without destroying the account.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"service": schema.StringAttribute{
				Required:      true,
				Description:   "Service identifier in `vendor:service` form, e.g. `appmixer:slack`. Changes force replacement.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				Required:      true,
				Description:   "Stable account name used as the identity key alongside `service`. Changes force replacement.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"display_name": schema.StringAttribute{
				Optional:    true,
				Description: "Human-readable label shown in the Appmixer UI. Updatable in-place.",
			},
			"token": schema.StringAttribute{
				Required:  true,
				Sensitive: true,
				Description: "Serialized credential token as a JSON string (e.g. `jsonencode({ accessToken = \"...\" })`). " +
					"Sensitive. Not returned by the API — Terraform persists the last-written value. " +
					"Updatable in-place: changes are applied via an upsert against the existing account.",
			},
			"profile_info": schema.StringAttribute{
				Optional:    true,
				Description: "Optional JSON string with additional profile metadata for the account. Updatable in-place.",
			},
		},
	}
}

func (r *accountResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *accountResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan accountModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var tokenObj any
	if err := json.Unmarshal([]byte(plan.Token.ValueString()), &tokenObj); err != nil {
		resp.Diagnostics.AddError("Invalid token JSON", err.Error())
		return
	}

	body := map[string]any{
		"service": plan.Service.ValueString(),
		"name":    plan.Name.ValueString(),
		"token":   tokenObj,
	}
	if !plan.DisplayName.IsNull() {
		body["displayName"] = plan.DisplayName.ValueString()
	}
	if !plan.ProfileInfo.IsNull() {
		var piObj any
		if err := json.Unmarshal([]byte(plan.ProfileInfo.ValueString()), &piObj); err != nil {
			resp.Diagnostics.AddError("Invalid profile_info JSON", err.Error())
			return
		}
		body["profileInfo"] = piObj
	}

	wire, err := client.Post[accountWire](ctx, r.client, "/accounts", body)
	if err != nil {
		resp.Diagnostics.AddError("Create /accounts failed", diagDetail(err))
		return
	}

	plan.ID = types.StringValue(wire.AccountID)

	if _, err := client.Post[map[string]any](ctx, r.client, "/accounts/"+wire.AccountID+"/test", nil); err != nil {
		var apiErr *client.APIError
		if !errors.As(err, &apiErr) || apiErr.StatusCode != 404 {
			resp.Diagnostics.AddWarning("Account test failed", diagDetail(err))
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *accountResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state accountModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	accountID := state.ID.ValueString()
	accounts, err := client.Get[[]accountWire](ctx, r.client, "/accounts")
	if err != nil {
		resp.Diagnostics.AddError("Read /accounts failed", diagDetail(err))
		return
	}

	var wire *accountWire
	for i := range accounts {
		if accounts[i].AccountID == accountID {
			wire = &accounts[i]
			break
		}
	}
	if wire == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	state.ID = types.StringValue(wire.AccountID)
	state.Service = types.StringValue(wire.Service)
	state.Name = types.StringValue(wire.Name)
	if wire.DisplayName != "" {
		state.DisplayName = types.StringValue(wire.DisplayName)
	}

	if _, err := client.Post[map[string]any](ctx, r.client, "/accounts/"+accountID+"/test", nil); err != nil {
		var apiErr *client.APIError
		if !errors.As(err, &apiErr) || apiErr.StatusCode != 404 {
			resp.Diagnostics.AddWarning("Account test failed", diagDetail(err))
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *accountResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state accountModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !plan.Token.Equal(state.Token) || !plan.ProfileInfo.Equal(state.ProfileInfo) || !plan.DisplayName.Equal(state.DisplayName) {
		var tokenObj any
		if err := json.Unmarshal([]byte(plan.Token.ValueString()), &tokenObj); err != nil {
			resp.Diagnostics.AddError("Invalid token JSON", err.Error())
			return
		}
		body := map[string]any{
			"service": plan.Service.ValueString(),
			"name":    plan.Name.ValueString(),
			"token":   tokenObj,
		}
		if !plan.DisplayName.IsNull() {
			body["displayName"] = plan.DisplayName.ValueString()
		}
		if !plan.ProfileInfo.IsNull() {
			var piObj any
			if err := json.Unmarshal([]byte(plan.ProfileInfo.ValueString()), &piObj); err != nil {
				resp.Diagnostics.AddError("Invalid profile_info JSON", err.Error())
				return
			}
			body["profileInfo"] = piObj
		}
		if _, err := client.Post[accountWire](ctx, r.client, "/accounts", body); err != nil {
			resp.Diagnostics.AddError("Update /accounts (upsert) failed", diagDetail(err))
			return
		}
	}

	plan.ID = state.ID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *accountResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state accountModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := client.Delete[map[string]any](ctx, r.client, "/accounts/"+state.ID.ValueString()); err != nil {
		if client.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Delete /accounts failed", diagDetail(err))
	}
}

func (r *accountResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
