package resource

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ellosoft/terraform-provider-appmixer/internal/client"
)

type componentResource struct {
	client *client.Client
}

func NewComponentResource() resource.Resource { return &componentResource{} }

type componentModel struct {
	ID          types.String `tfsdk:"id"`
	Selector    types.String `tfsdk:"selector"`
	Source      types.String `tfsdk:"source"`
	ReplaceAll  types.Bool   `tfsdk:"replace_all"`
	FileHash    types.String `tfsdk:"file_hash"`
	PublishedAt types.String `tfsdk:"published_at"`
}

type publishResponse struct {
	Ticket string `json:"ticket"`
}

type uploadStatus struct {
	Finished string `json:"finished"`
	Err      string `json:"err"`
	Data     []any  `json:"data"`
}

// fileHashModifier is a custom plan modifier that computes the SHA256 hash of
// the zip file referenced by the "source" attribute. When the hash changes
// between plan and state, Terraform treats it as a diff and triggers an update.
type fileHashModifier struct{}

func (m fileHashModifier) Description(_ context.Context) string {
	return "Computes SHA256 of the source zip file to detect changes."
}

func (m fileHashModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m fileHashModifier) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	var source types.String
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("source"), &source)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// During initial plan before apply, source may be unknown.
	if source.IsUnknown() {
		resp.PlanValue = types.StringUnknown()
		return
	}

	filePath := source.ValueString()
	data, err := os.ReadFile(filePath)
	if err != nil {
		resp.Diagnostics.AddError(
			"Cannot read source file",
			fmt.Sprintf("Failed to read %q: %s", filePath, err),
		)
		return
	}

	hash := fmt.Sprintf("%x", sha256.Sum256(data))

	// New resource (no prior state).
	if req.StateValue.IsNull() {
		resp.PlanValue = types.StringValue(hash)
		return
	}

	// Existing resource — only update if hash changed.
	if hash != req.StateValue.ValueString() {
		resp.PlanValue = types.StringValue(hash)
		return
	}

	// Hash unchanged — keep state value.
	resp.PlanValue = req.StateValue
}

// publishedAtModifier marks published_at as unknown whenever file_hash changes
// (i.e., a re-publish will happen), so the framework doesn't see an inconsistency
// between the planned and applied values.
type publishedAtModifier struct{}

func (m publishedAtModifier) Description(_ context.Context) string {
	return "Marks published_at as unknown when a re-publish is pending."
}

func (m publishedAtModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m publishedAtModifier) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	// New resource — value will be computed on apply.
	if req.StateValue.IsNull() {
		resp.PlanValue = types.StringUnknown()
		return
	}

	// Check if file_hash is changing — if so, a re-publish will produce a new timestamp.
	var stateHash, planHash types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("file_hash"), &stateHash)...)
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("file_hash"), &planHash)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if planHash.IsUnknown() || planHash.ValueString() != stateHash.ValueString() {
		resp.PlanValue = types.StringUnknown()
		return
	}

	// No change — keep state value.
	resp.PlanValue = req.StateValue
}

func (r *componentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_component"
}

func (r *componentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Publishes an Appmixer component from a local zip file. " +
			"The component is identified by its `selector` (dot-separated identifier). " +
			"Changes to the zip file (detected via SHA256 hash) trigger a re-publish.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"selector": schema.StringAttribute{
				Required:      true,
				Description:   "Dot-separated component identifier. Changes force replacement.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"source": schema.StringAttribute{
				Required:    true,
				Description: "Path to the component zip file.",
			},
			"replace_all": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Whether to replace all existing versions of the component. Defaults to false.",
			},
			"file_hash": schema.StringAttribute{
				Computed:      true,
				Description:   "SHA256 hash of the source zip file. Changes trigger an update.",
				PlanModifiers: []planmodifier.String{fileHashModifier{}},
			},
			"published_at": schema.StringAttribute{
				Computed:      true,
				Description:   "Timestamp from the upload response indicating when the component was published.",
				PlanModifiers: []planmodifier.String{publishedAtModifier{}},
			},
		},
	}
}

func (r *componentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *componentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan componentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	hash, finished, diags := r.publishAndWait(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	plan.ID = plan.Selector
	plan.FileHash = types.StringValue(hash)
	plan.PublishedAt = types.StringValue(finished)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *componentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state componentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	selector := state.Selector.ValueString()
	components, err := client.Get[[]map[string]any](ctx, r.client, "/apps/components?app="+selector)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read /apps/components failed", diagDetail(err))
		return
	}

	if len(components) == 0 {
		resp.State.RemoveResource(ctx)
		return
	}

	// Component exists — keep state as-is (file_hash and published_at are local-only).
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *componentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan componentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	hash, finished, diags := r.publishAndWait(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	plan.ID = plan.Selector
	plan.FileHash = types.StringValue(hash)
	plan.PublishedAt = types.StringValue(finished)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *componentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state componentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	selector := state.Selector.ValueString()
	if _, err := client.Delete[map[string]any](ctx, r.client, "/components/"+selector); err != nil {
		if client.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Delete /components failed", diagDetail(err))
	}
}

func (r *componentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("selector"), req.ID)...)
}

// publishAndWait reads the zip file from the plan's Source path, uploads it to
// the Appmixer components endpoint, and polls for completion. It returns the
// SHA256 hex hash of the file, the finished timestamp, and any diagnostics.
func (r *componentResource) publishAndWait(ctx context.Context, plan componentModel) (string, string, diag.Diagnostics) {
	var diags diag.Diagnostics

	sourcePath := plan.Source.ValueString()
	zipBytes, err := os.ReadFile(sourcePath)
	if err != nil {
		diags.AddError("Cannot read source file", fmt.Sprintf("Failed to read %q: %s", sourcePath, err))
		return "", "", diags
	}

	hash := fmt.Sprintf("%x", sha256.Sum256(zipBytes))

	replaceAllStr := "false"
	if plan.ReplaceAll.ValueBool() {
		replaceAllStr = "true"
	}

	pub, err := client.PostBinary[publishResponse](ctx, r.client, "/components?replaceAll="+replaceAllStr, bytes.NewReader(zipBytes))
	if err != nil {
		diags.AddError("Publish component failed", diagDetail(err))
		return "", "", diags
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	deadline := time.After(5 * time.Minute)

	for {
		select {
		case <-ctx.Done():
			diags.AddError("Publish component cancelled", ctx.Err().Error())
			return "", "", diags
		case <-deadline:
			diags.AddError("Publish component timed out", "Upload did not complete within 5 minutes.")
			return "", "", diags
		case <-ticker.C:
			status, err := client.Get[uploadStatus](ctx, r.client, "/components/uploader/"+pub.Ticket)
			if err != nil {
				diags.AddError("Poll upload status failed", diagDetail(err))
				return "", "", diags
			}
			if status.Err != "" {
				detail := status.Err
				if len(status.Data) > 0 {
					detail = fmt.Sprintf("%s\nValidation errors: %v", status.Err, status.Data)
				}
				diags.AddError("Component upload failed", detail)
				return "", "", diags
			}
			if status.Finished != "" {
				return hash, status.Finished, diags
			}
		}
	}
}
