package resource

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ellosoft/terraform-provider-appmixer/internal/client"
)

type userResource struct {
	client *client.Client
}

func NewUserResource() resource.Resource { return &userResource{} }

type userModel struct {
	ID       types.String `tfsdk:"id"`
	UserID   types.String `tfsdk:"user_id"`
	Email    types.String `tfsdk:"email"`
	Password types.String `tfsdk:"password"`
	Scope    types.List   `tfsdk:"scope"`
	Metadata types.Map    `tfsdk:"metadata"`
}

type userWire struct {
	UserID   string            `json:"userId"`
	Username string            `json:"username"`
	Scope    []string          `json:"scope"`
	Metadata map[string]string `json:"metadata"`
}

type deleteTicketResponse struct {
	Ticket string `json:"ticket"`
}

type deleteStatusResponse struct {
	Status string `json:"status"`
}

func (r *userResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (r *userResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"user_id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"email": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"password": schema.StringAttribute{
				Required:      true,
				Sensitive:     true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"scope": schema.ListAttribute{
				Optional:      true,
				ElementType:   types.StringType,
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"metadata": schema.MapAttribute{
				Optional:      true,
				ElementType:   types.StringType,
				PlanModifiers: []planmodifier.Map{mapplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *userResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *userResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan userModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var scope []string
	if !plan.Scope.IsNull() && !plan.Scope.IsUnknown() {
		resp.Diagnostics.Append(plan.Scope.ElementsAs(ctx, &scope, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	var metadata map[string]string
	if !plan.Metadata.IsNull() && !plan.Metadata.IsUnknown() {
		resp.Diagnostics.Append(plan.Metadata.ElementsAs(ctx, &metadata, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	body := map[string]any{
		"username": plan.Email.ValueString(),
		"password": plan.Password.ValueString(),
		"scope":    scope,
		"metadata": metadata,
	}

	wire, err := client.Post[userWire](ctx, r.client, "/user", body)
	if err != nil {
		resp.Diagnostics.AddError("Create /user failed", diagDetail(err))
		return
	}

	plan.ID = types.StringValue(wire.UserID)
	plan.UserID = types.StringValue(wire.UserID)
	// password stays from plan; scope and metadata stay from plan
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *userResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state userModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// After an import, user_id may not be set yet; fall back to id.
	userID := state.UserID.ValueString()
	if userID == "" {
		userID = state.ID.ValueString()
	}

	wire, err := client.Get[userWire](ctx, r.client, "/users/"+userID)
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read /users failed", diagDetail(err))
		return
	}

	// Always ensure id and user_id are set (important after import).
	state.ID = types.StringValue(userID)
	state.UserID = types.StringValue(userID)
	state.Email = types.StringValue(wire.Username)

	scopeVal, d := types.ListValueFrom(ctx, types.StringType, wire.Scope)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Scope = scopeVal

	metaVal, d := types.MapValueFrom(ctx, types.StringType, wire.Metadata)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Metadata = metaVal

	// Do NOT update password from server — keep whatever is in state.
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *userResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan userModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state userModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var scope []string
	if !plan.Scope.IsNull() && !plan.Scope.IsUnknown() {
		resp.Diagnostics.Append(plan.Scope.ElementsAs(ctx, &scope, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	var metadata map[string]string
	if !plan.Metadata.IsNull() && !plan.Metadata.IsUnknown() {
		resp.Diagnostics.Append(plan.Metadata.ElementsAs(ctx, &metadata, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	body := map[string]any{
		"userId":   state.UserID.ValueString(),
		"username": plan.Email.ValueString(),
		"scope":    scope,
		"metadata": metadata,
	}

	if _, err := client.Put[userWire](ctx, r.client, "/users/"+state.UserID.ValueString(), body); err != nil {
		resp.Diagnostics.AddError("Update /users failed", diagDetail(err))
		return
	}

	// Keep id/user_id from state; update other fields from plan.
	plan.ID = state.ID
	plan.UserID = state.UserID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *userResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state userModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	userID := state.UserID.ValueString()
	ticket, err := client.Delete[deleteTicketResponse](ctx, r.client, "/users/"+userID)
	if err != nil {
		resp.Diagnostics.AddError("Delete /users failed", diagDetail(err))
		return
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			resp.Diagnostics.AddError("Delete timed out", "context deadline exceeded while waiting for user deletion to complete")
			return
		case <-ticker.C:
			statusResp, err := client.Get[deleteStatusResponse](
				ctx, r.client,
				"/users/"+userID+"/delete-status/"+ticket.Ticket,
			)
			if err != nil {
				resp.Diagnostics.AddError("Delete status poll failed", diagDetail(err))
				return
			}
			switch statusResp.Status {
			case "completed":
				return
			case "failed":
				resp.Diagnostics.AddError("User deletion failed", "server reported deletion status: failed")
				return
			}
			// "in-progress" or any other status: keep polling
		}
	}
}

func (r *userResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
