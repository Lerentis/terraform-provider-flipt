// Copyright (c) terraform-provider-flipt contributors
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ datasource.DataSource = &EnvironmentDataSource{}

func NewEnvironmentDataSource() datasource.DataSource {
	return &EnvironmentDataSource{}
}

type EnvironmentDataSource struct {
	config *FliptProviderConfig
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
		Description:         "Flipt environment data source",

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

	providerConfig, ok := req.ProviderData.(*FliptProviderConfig)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *FliptProviderConfig, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.config = providerConfig
}

func (d *EnvironmentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data EnvironmentDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading environment", map[string]interface{}{
		"key": data.Key.ValueString(),
	})

	// List all environments and find the requested one
	url := fmt.Sprintf("%s/api/v2/environments", d.config.Endpoint)
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		resp.Diagnostics.AddError("Request Error", fmt.Sprintf("Unable to create request: %s", err))
		return
	}

	d.config.AddAuthHeader(httpReq)
	httpResp, err := d.config.HTTPClient.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read environments, got error: %s", err))
		return
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Read Error", fmt.Sprintf("Unable to read response: %s", err))
		return
	}

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read environments, status: %d, body: %s", httpResp.StatusCode, string(body)))
		return
	}

	var response struct {
		Environments []struct {
			Key     string `json:"key"`
			Name    string `json:"name"`
			Default bool   `json:"default"`
		} `json:"environments"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return
	}

	// Find the requested environment by key
	var found bool
	for _, env := range response.Environments {
		if env.Key == data.Key.ValueString() {
			data.Key = types.StringValue(env.Key)
			data.Name = types.StringValue(env.Name)
			data.Default = types.BoolValue(env.Default)
			found = true
			break
		}
	}

	if !found {
		resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Environment with key '%s' not found", data.Key.ValueString()))
		return
	}

	tflog.Trace(ctx, "read an environment data source")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
