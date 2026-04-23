package provider

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/ellosoft/terraform-provider-appmixer/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type appmixerProvider struct{}

func New() func() provider.Provider {
	return func() provider.Provider { return &appmixerProvider{} }
}

func (p *appmixerProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "appmixer"
	resp.Version = "0.0.1"
}

func (p *appmixerProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"base_url": schema.StringAttribute{
				Optional:    true,
				Description: "Appmixer API base URL. Falls back to APPMIXER_BASE_URL.",
			},
			"username": schema.StringAttribute{
				Optional:    true,
				Description: "Appmixer admin username. Falls back to APPMIXER_USERNAME.",
			},
			"password": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Appmixer admin password. Falls back to APPMIXER_PASSWORD.",
			},
		},
	}
}

type providerConfig struct {
	BaseURL  types.String `tfsdk:"base_url"`
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`
}

func (p *appmixerProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg providerConfig
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	baseURL := firstNonEmpty(cfg.BaseURL.ValueString(), os.Getenv("APPMIXER_BASE_URL"))
	username := firstNonEmpty(cfg.Username.ValueString(), os.Getenv("APPMIXER_USERNAME"))
	password := firstNonEmpty(cfg.Password.ValueString(), os.Getenv("APPMIXER_PASSWORD"))

	if baseURL == "" || username == "" || password == "" {
		resp.Diagnostics.AddError(
			"Missing provider configuration",
			"base_url, username, and password must all be set (via HCL or APPMIXER_BASE_URL/APPMIXER_USERNAME/APPMIXER_PASSWORD env vars).",
		)
		return
	}

	c := client.New(baseURL)
	if err := c.Login(ctx, username, password); err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) {
			resp.Diagnostics.AddError(
				"Login failed",
				fmt.Sprintf("%s %s returned status %d", apiErr.Method, apiErr.Path, apiErr.StatusCode),
			)
		} else {
			resp.Diagnostics.AddError("Login failed", err.Error())
		}
		return
	}

	resp.ResourceData = c
	resp.DataSourceData = c
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func (p *appmixerProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{}
}

func (p *appmixerProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}
