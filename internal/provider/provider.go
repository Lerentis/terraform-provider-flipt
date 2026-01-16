// Copyright (c) terraform-provider-flipt contributors
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
	Token    types.String `tfsdk:"token"`
	JWT      types.String `tfsdk:"jwt"`
}

// FliptProviderConfig holds the configured HTTP client and endpoint for resources.
type FliptProviderConfig struct {
	HTTPClient *http.Client
	Endpoint   string
	Token      string
	JWT        string
}

// AddAuthHeader adds the appropriate authentication header to an HTTP request.
func (c *FliptProviderConfig) AddAuthHeader(req *http.Request) {
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	} else if c.JWT != "" {
		req.Header.Set("Authorization", "JWT "+c.JWT)
	}
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
			"token": schema.StringAttribute{
				MarkdownDescription: "Static authentication token for Bearer authentication",
				Optional:            true,
				Sensitive:           true,
			},
			"jwt": schema.StringAttribute{
				MarkdownDescription: "JWT token for JWT authentication",
				Optional:            true,
				Sensitive:           true,
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

	// Use the base endpoint without environment path
	endpoint := data.Endpoint.ValueString()

	// Get authentication tokens
	token := ""
	if !data.Token.IsNull() {
		token = data.Token.ValueString()
	}

	jwt := ""
	if !data.JWT.IsNull() {
		jwt = data.JWT.ValueString()
	}

	// Validate that only one authentication method is provided
	if token != "" && jwt != "" {
		resp.Diagnostics.AddError(
			"Conflicting Authentication",
			"Both token and jwt are configured. Please provide only one authentication method.",
		)
		return
	}

	// Create HTTP client
	httpClient := &http.Client{}

	// Create provider configuration
	config := &FliptProviderConfig{
		HTTPClient: httpClient,
		Endpoint:   endpoint,
		Token:      token,
		JWT:        jwt,
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
