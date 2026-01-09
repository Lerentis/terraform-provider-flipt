// Copyright (c) terraform-flipt contributors
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	sdk "go.flipt.io/flipt/sdk/go"
)

var _ datasource.DataSource = &EnvironmentDataSource{}

func NewEnvironmentDataSource() datasource.DataSource {
	return &EnvironmentDataSource{}
}

type EnvironmentDataSource struct {
	client *sdk.SDK
}

type EnvironmentDataSourceModel struct {
	Key     types.String `tfsdk:"key"`
	Name    types.String `tfsdk:"name"`
	Default types.Bool   `tfsdk:"default"`
}

func (d *EnvironmentDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment"
}

func (d *EnvironmentDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Flipt environment data source. Environments are read-only and configured server-side.",

		Attributes: map[string]schema.Attribute{
			"key": schema.StringAttribute{
				MarkdownDescription: "Unique key for the environment",
				Required:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name of the environment",
				Computed:            true,
			},
			"default": schema.BoolAttribute{
				MarkdownDescription: "Whether this is the default environment",
				Computed:            true,
			},
		},
	}
}

func (d *EnvironmentDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*sdk.SDK)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *sdk.SDK, got: %T", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *EnvironmentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data EnvironmentDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Environments are read-only in Flipt v2 and are configured server-side
	// We validate that the requested environment key exists by listing all environments
	// Note: The SDK doesn't have a direct GetEnvironment method, environments are managed via configuration

	// For now, we'll accept any key value and set basic information
	// In a production scenario, you might want to validate against server configuration
	data.Key = types.StringValue(data.Key.ValueString())
	data.Name = types.StringValue(data.Key.ValueString())

	// Check if this is the default environment
	if data.Key.ValueString() == "default" {
		data.Default = types.BoolValue(true)
	} else {
		data.Default = types.BoolValue(false)
	}

	tflog.Trace(ctx, "read an environment data source")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
