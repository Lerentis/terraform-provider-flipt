// Copyright (c) terraform-flipt contributors
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	sdk "go.flipt.io/flipt/sdk/go"
	sdkhttp "go.flipt.io/flipt/sdk/go/http"
)

// Ensure FliptProvider satisfies various provider interfaces.
var _ provider.Provider = &FliptProvider{}

// FliptProvider defines the provider implementation.
type FliptProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// FliptProviderModel describes the provider data model.
type FliptProviderModel struct {
	Endpoint       types.String `tfsdk:"endpoint"`
	EnvironmentKey types.String `tfsdk:"environment_key"`
}

// FliptProviderConfig holds the configured HTTP client and endpoint for resources.
type FliptProviderConfig struct {
	HTTPClient *http.Client
	Endpoint   string
	SDKClient  *sdk.SDK // TODO: Remove when all resources are migrated to manual HTTP
}

func (p *FliptProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "flipt"
	resp.Version = p.version
}

func (p *FliptProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "Flipt server endpoint URL",
				Required:            true,
			},
			"environment_key": schema.StringAttribute{
				MarkdownDescription: "Default environment key for Flipt v2 (defaults to 'default')",
				Optional:            true,
			},
		},
	}
}

func (p *FliptProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data FliptProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Validate endpoint is provided
	if data.Endpoint.IsNull() || data.Endpoint.ValueString() == "" {
		resp.Diagnostics.AddError(
			"Missing Flipt Endpoint",
			"The provider requires a Flipt server endpoint URL to be configured.",
		)
		return
	}

	// Get environment key, default to "default"
	envKey := "default"
	if !data.EnvironmentKey.IsNull() && data.EnvironmentKey.ValueString() != "" {
		envKey = data.EnvironmentKey.ValueString()
	}

	// For Flipt v2, append environment path to endpoint
	endpoint := data.Endpoint.ValueString() + "/api/v2/environments/" + envKey

	// Create HTTP client
	httpClient := &http.Client{}

	// Create SDK client for unmigrated resources (TODO: Remove when all migrated)
	sdkTransport := sdkhttp.NewTransport(data.Endpoint.ValueString())
	sdkClient := sdk.New(sdkTransport)

	// Create provider configuration
	config := &FliptProviderConfig{
		HTTPClient: httpClient,
		Endpoint:   endpoint,
		SDKClient:  &sdkClient,
	}

	resp.DataSourceData = config
	resp.ResourceData = config
}

func (p *FliptProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewNamespaceResource,
		NewFlagResource,
		NewSegmentResource,
		NewVariantResource,
		NewConstraintResource,
		NewRuleResource,
	}
}

func (p *FliptProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewNamespaceDataSource,
		NewEnvironmentDataSource,
		NewFlagDataSource,
		NewSegmentDataSource,
		NewVariantDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &FliptProvider{
			version: version,
		}
	}
}
