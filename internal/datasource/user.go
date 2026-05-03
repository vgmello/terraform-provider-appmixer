package datasource

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ellosoft/terraform-provider-appmixer/internal/apitypes"
	"github.com/ellosoft/terraform-provider-appmixer/internal/client"
)

type userDataSource struct {
	client *client.Client
}

func NewUserDataSource() datasource.DataSource { return &userDataSource{} }

type userDataModel struct {
	ID       types.String `tfsdk:"id"`
	UserID   types.String `tfsdk:"user_id"`
	Email    types.String `tfsdk:"email"`
	Scope    types.List   `tfsdk:"scope"`
	Metadata types.Map    `tfsdk:"metadata"`
}

func (d *userDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (d *userDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads an existing Appmixer user by ID. Use this to reference a user that was created outside of Terraform.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"user_id": schema.StringAttribute{
				Required:    true,
				Description: "Server-assigned user ID to look up.",
			},
			"email": schema.StringAttribute{
				Computed:    true,
				Description: "User's email address (login username).",
			},
			"scope": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Permission scopes assigned to the user.",
			},
			"metadata": schema.MapAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Arbitrary metadata attached to the user account.",
			},
		},
	}
}

func (d *userDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("%T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *userDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg userDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	wire, err := client.Get[apitypes.UserWire](ctx, d.client, "/users/"+cfg.UserID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.Diagnostics.AddError("User not found", fmt.Sprintf("No user with ID %q exists.", cfg.UserID.ValueString()))
			return
		}
		resp.Diagnostics.AddError("Read /users failed", diagDetail(err))
		return
	}

	scope, diags := types.ListValueFrom(ctx, types.StringType, wire.Scope)
	resp.Diagnostics.Append(diags...)
	meta, diags := types.MapValueFrom(ctx, types.StringType, wire.Metadata)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &userDataModel{
		ID:       types.StringValue(wire.UserID),
		UserID:   types.StringValue(wire.UserID),
		Email:    types.StringValue(wire.Username),
		Scope:    scope,
		Metadata: meta,
	})...)
}

// diagDetail is a package-local alias for client.DiagDetail so datasource
// files can surface client errors without importing the error type directly.
func diagDetail(err error) string { return client.DiagDetail(err) }
