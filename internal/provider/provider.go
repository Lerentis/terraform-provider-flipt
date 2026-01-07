// Copyright (c) terraform-flipt contributors
// SPDX-License-Identifier: MIT

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	flipt "github.com/lerentis/flipt-server-rest-sdk-go/generated"
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
	Endpoint types.String `tfsdk:"endpoint"`
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

	// Create Flipt SDK client configuration
	cfg := flipt.NewConfiguration()
	cfg.Servers = flipt.ServerConfigurations{
		{
			URL: data.Endpoint.ValueString(),
		},
	}

	// Create Flipt API client
	client := flipt.NewAPIClient(cfg)

	resp.DataSourceData = client
	resp.ResourceData = client
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
		NewFlagDataSource,
		NewSegmentDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &FliptProvider{
			version: version,
		}
	}
}
