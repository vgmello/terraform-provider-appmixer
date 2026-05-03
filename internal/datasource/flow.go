package datasource

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ellosoft/terraform-provider-appmixer/internal/apitypes"
	"github.com/ellosoft/terraform-provider-appmixer/internal/client"
)

type flowDataSource struct {
	client *client.Client
}

func NewFlowDataSource() datasource.DataSource { return &flowDataSource{} }

type flowDataModel struct {
	ID           types.String `tfsdk:"id"`
	FlowID       types.String `tfsdk:"flow_id"`
	Name         types.String `tfsdk:"name"`
	FlowJSON     types.String `tfsdk:"flow_json"`
	CustomFields types.Map    `tfsdk:"custom_fields"`
	Stage        types.String `tfsdk:"stage"`
}

func (d *flowDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_flow"
}

func (d *flowDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads an existing Appmixer flow by ID. Use this to reference a flow that was created outside of Terraform.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"flow_id": schema.StringAttribute{
				Required:    true,
				Description: "Server-assigned flow ID to look up.",
			},
			"name": schema.StringAttribute{
				Computed:    true,
				Description: "Human-readable name of the flow.",
			},
			"flow_json": schema.StringAttribute{
				Computed:    true,
				Description: "Flow descriptor as a canonical JSON string.",
			},
			"custom_fields": schema.MapAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Arbitrary string metadata attached to the flow.",
			},
			"stage": schema.StringAttribute{
				Computed:    true,
				Description: "Current execution stage: `running` or `stopped`.",
			},
		},
	}
}

func (d *flowDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *flowDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg flowDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	wire, err := client.Get[apitypes.FlowWire](ctx, d.client, "/flows/"+cfg.FlowID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.Diagnostics.AddError("Flow not found", fmt.Sprintf("No flow with ID %q exists.", cfg.FlowID.ValueString()))
			return
		}
		resp.Diagnostics.AddError("Read /flows failed", diagDetail(err))
		return
	}

	flowJSON, err := json.Marshal(wire.Flow)
	if err != nil {
		resp.Diagnostics.AddError("Marshal flow failed", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &flowDataModel{
		ID:           types.StringValue(wire.FlowID),
		FlowID:       types.StringValue(wire.FlowID),
		Name:         types.StringValue(wire.Name),
		FlowJSON:     types.StringValue(string(flowJSON)),
		CustomFields: apitypes.BuildCustomFieldsMap(wire.CustomFields),
		Stage:        types.StringValue(wire.Stage),
	})...)
}
